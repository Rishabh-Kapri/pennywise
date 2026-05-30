package temporal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.temporal.io/sdk/activity"
)

type CreateTransactionActivity struct {
	TransactionService service.TransactionService
	PayeeService       service.PayeeService
	PredictionService  service.PredictionService
	WebsocketService   service.WebsocketService
	DB                 *pgxpool.Pool
}

func (a *CreateTransactionActivity) CreateTransaction(
	ctx context.Context,
	input sharedModel.PredictionResultInput,
) ([]sharedModel.Transaction, error) {
	ctx = utils.WithServiceName(ctx, "pennywise-api")
	activityInfo := activity.GetInfo(ctx)
	log := logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)

	predictions := input.Predictions
	budgetId := input.BudgetID

	if len(predictions) == 0 {
		log.Info("no predictions found")
		return nil, nil
	}
	if budgetId == uuid.Nil {
		log.Error("no budget id found")
		return nil, errs.New(errs.CodeInvalidArgument, "no budget id found")
	}

	ctx = utils.WithBudgetID(ctx, budgetId)
	var createdTxns []sharedModel.Transaction

	for _, p := range predictions {
		payeeID := p.PayeeID
		// handle payee creation
		// payee will only be not present for the llm fallback
		if payeeID == uuid.Nil {
			payeeName := p.Payee
			if payeeName == "" {
				payeeName = "Unknown Payee"
			}
			log.Info("payee missing, creating new payee", "name", payeeName)
			newPayee, err := a.PayeeService.Create(ctx, sharedModel.Payee{Name: payeeName})
			if err != nil {
				return nil, errs.Wrap(errs.CodePayeeCreateFailed, "error creating payee", err)
			}
			payeeID = newPayee.ID
		}

		log.Info("creating transaction", "prediction", p)
		hash := utils.Hash(p.AccountID.String() + p.Date + fmt.Sprintf("%.2f", p.Amount) + p.OriginalRawText)
		txn := sharedModel.Transaction{
			BudgetID:    budgetId,
			AccountID:   &p.AccountID,
			PayeeID:     &payeeID,
			CategoryID:  &p.CategoryID,
			Amount:      p.Amount,
			Date:        sharedModel.Date(p.Date),
			Status:      sharedModel.TransactionStatusUnapproved,
			DedupeHash:  &hash,
			RawBankText: &p.OriginalRawText,
			Summary:     &p.Summary,
		}

		createdTxn, err := a.TransactionService.Create(ctx, txn)
		if err != nil {
			return nil, err
		}
		if len(createdTxn) == 0 {
			return nil, errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
		}
		createdTxns = append(createdTxns, createdTxn[0])
	}
	a.sendTransactionCreatedNotification(ctx, budgetId, createdTxns, log)
	return createdTxns, nil
}

func (a *CreateTransactionActivity) CreateTransactionAndCipherPrediction(
	ctx context.Context,
	input sharedModel.PredictionResultInput,
) ([]sharedModel.Transaction, error) {
	ctx = utils.WithServiceName(ctx, "pennywise-api")
	activityInfo := activity.GetInfo(ctx)
	log := logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)

	if a.DB == nil {
		return nil, errs.New(errs.CodeInvalidArgument, "database pool is required")
	}
	if a.PredictionService == nil {
		return nil, errs.New(errs.CodeInvalidArgument, "prediction service is required")
	}
	if len(input.Predictions) == 0 {
		log.Info("no predictions found")
		return nil, nil
	}
	if input.BudgetID == uuid.Nil {
		log.Error("no budget id found")
		return nil, errs.New(errs.CodeInvalidArgument, "no budget id found")
	}

	ctx = utils.WithBudgetID(ctx, input.BudgetID)

	var createdTxns []sharedModel.Transaction

	err := utils.WithTx(ctx, a.DB, func(tx pgx.Tx) error {
		var err error
		createdTxns, err = a.createTransactions(ctx, tx, input.Predictions, input.BudgetID, log)
		if err != nil {
			return err
		}
		for i, txn := range createdTxns {
			log.Info("creating cipher prediction", "transactionId", txn.ID, "source", input.Predictions[i].Source)
			if err = createCipherPredictionWithTx(
				ctx,
				tx,
				a.PredictionService,
				input.BudgetID,
				txn,
				input.Predictions[i],
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	a.sendTransactionCreatedNotification(ctx, input.BudgetID, createdTxns, log)
	return createdTxns, nil
}

func (a *CreateTransactionActivity) sendTransactionCreatedNotification(
	ctx context.Context,
	budgetId uuid.UUID,
	transactions []sharedModel.Transaction,
	log *slog.Logger,
) {
	if a.WebsocketService == nil || len(transactions) == 0 {
		return
	}

	if err := a.WebsocketService.SendNotification(ctx, budgetId, "pennywise::transaction::created", transactions); err != nil {
		log.Warn("failed to send transaction created websocket notification", "error", err)
	}
}

func (a *CreateTransactionActivity) createTransactions(
	ctx context.Context,
	tx pgx.Tx,
	predictions []sharedModel.CipherPredictionResult,
	budgetId uuid.UUID,
	log *slog.Logger,
) ([]sharedModel.Transaction, error) {
	createdTxns := make([]sharedModel.Transaction, 0, len(predictions))
	for _, p := range predictions {
		payeeID := p.PayeeID
		if payeeID == uuid.Nil {
			payeeName := p.Payee
			if payeeName == "" {
				payeeName = "Unknown Payee"
			}
			log.Info("payee missing, creating new payee", "name", payeeName)
			newPayee, err := a.PayeeService.CreateWithTx(ctx, tx, sharedModel.Payee{Name: payeeName})
			if err != nil {
				return nil, errs.Wrap(errs.CodePayeeCreateFailed, "error creating payee", err)
			}
			payeeID = newPayee.ID
		}

		log.Info("creating transaction", "prediction", p)
		hash := utils.Hash(p.AccountID.String() + p.Date + fmt.Sprintf("%.2f", p.Amount) + p.OriginalRawText)
		txn := sharedModel.Transaction{
			BudgetID:    budgetId,
			AccountID:   &p.AccountID,
			PayeeID:     &payeeID,
			CategoryID:  &p.CategoryID,
			Amount:      p.Amount,
			Date:        sharedModel.Date(p.Date),
			Status:      sharedModel.TransactionStatusUnapproved,
			DedupeHash:  &hash,
			RawBankText: &p.OriginalRawText,
		}

		createdTxn, err := a.TransactionService.CreateWithTx(ctx, tx, txn)
		if err != nil {
			return nil, err
		}
		if len(createdTxn) == 0 {
			return nil, errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
		}
		createdTxns = append(createdTxns, createdTxn[0])
	}
	return createdTxns, nil
}
