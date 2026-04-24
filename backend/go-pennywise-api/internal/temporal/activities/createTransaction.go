package temporal

import (
	"context"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type CreateTransactionActivity struct {
	TransactionService service.TransactionService
	PayeeService       service.PayeeService
}

func (a *CreateTransactionActivity) CreateTransaction(
	ctx context.Context,
	input sharedModel.PredictionResultInput,
) ([]sharedModel.Transaction, error) {
	log := logger.Logger(ctx)

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
		if payeeID == uuid.Nil || p.Payee == "" {
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
	return createdTxns, nil
}
