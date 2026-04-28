package temporal

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/activity"
)

type CreateCipherPredictionActivity struct {
	PredictionService service.PredictionService
}

// CreateCipherPrediction persists a cipher_prediction row for every transaction that
// was created by the workflow. Transactions and predictions are matched by position.
func (a *CreateCipherPredictionActivity) CreateCipherPrediction(
	ctx context.Context,
	input sharedModel.CreateCipherPredictionInput,
) error {
	ctx = utils.WithServiceName(ctx, "pennywise-api")
	activityInfo := activity.GetInfo(ctx)
	log := logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)

	if input.BudgetID == uuid.Nil {
		return errs.New(errs.CodeInvalidArgument, "no budget id found")
	}

	if len(input.Transactions) == 0 {
		log.Info("no transactions to create cipher predictions for")
		return nil
	}

	if len(input.Transactions) != len(input.Predictions) {
		return errs.New(errs.CodeInvalidArgument, "transactions and predictions slice lengths do not match")
	}

	ctx = utils.WithBudgetID(ctx, input.BudgetID)

	for i, txn := range input.Transactions {
		record, err := buildCipherPredictionRecord(input.BudgetID, txn, input.Predictions[i])
		if err != nil {
			return err
		}

		log.Info("creating cipher prediction", "transactionId", txn.ID, "source", input.Predictions[i].Source)
		_, err = a.PredictionService.CreateCipherPrediction(ctx, record)
		if err != nil {
			return err
		}
	}

	return nil
}

func createCipherPredictionWithTx(
	ctx context.Context,
	tx pgx.Tx,
	service service.PredictionService,
	budgetID uuid.UUID,
	txn sharedModel.Transaction,
	pred sharedModel.CipherPredictionResult,
) error {
	record, err := buildCipherPredictionRecord(budgetID, txn, pred)
	if err != nil {
		return err
	}
	_, err = service.CreateCipherPredictionWithTx(ctx, tx, record)
	return err
}

func buildCipherPredictionRecord(
	budgetID uuid.UUID,
	txn sharedModel.Transaction,
	pred sharedModel.CipherPredictionResult,
) (sharedModel.CipherPredictionRecord, error) {
	emailText := pred.OriginalRawText
	if emailText == "" && txn.RawBankText != nil {
		emailText = *txn.RawBankText
	}
	var emailTextPtr *string
	if emailText != "" {
		emailTextPtr = &emailText
	}
	var reasoning *string
	if pred.Reasoning != "" {
		reasoning = &pred.Reasoning
	}
	var metadata json.RawMessage
	if len(pred.Metadata) > 0 {
		data, err := json.Marshal(pred.Metadata)
		if err != nil {
			return sharedModel.CipherPredictionRecord{}, errs.Wrap(
				errs.CodeInvalidArgument,
				"invalid prediction metadata",
				err,
			)
		}
		metadata = data
	}

	var confidence *float64
	if pred.Confidence != "" {
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(pred.Confidence, "%"), 64)
		if err != nil {
			return sharedModel.CipherPredictionRecord{}, errs.Wrap(
				errs.CodeInvalidArgument,
				"invalid prediction confidence",
				err,
			)
		}
		confidence = &parsed
	}
	var predictedPayeeID *uuid.UUID
	if txn.PayeeID != nil && *txn.PayeeID != uuid.Nil {
		predictedPayeeID = txn.PayeeID
	} else if pred.PayeeID != uuid.Nil {
		predictedPayeeID = &pred.PayeeID
	}
	var predictedCategoryID *uuid.UUID
	if txn.CategoryID != nil && *txn.CategoryID != uuid.Nil {
		predictedCategoryID = txn.CategoryID
	} else if pred.CategoryID != uuid.Nil {
		predictedCategoryID = &pred.CategoryID
	}
	accountConfidence := 100.0

	return sharedModel.CipherPredictionRecord{
		BudgetID:            budgetID,
		TransactionID:       txn.ID,
		EmailText:           emailTextPtr,
		LLMReasoning:        reasoning,
		Metadata:            metadata,
		ExtractedAccount:    &pred.Account,
		ExtractedPayee:      &pred.Payee,
		PredictedPayeeID:    predictedPayeeID,
		PredictedCategoryID: predictedCategoryID,
		AccountConfidence:   &accountConfidence,
		PayeeConfidence:     confidence,
		CategoryConfidence:  confidence,
		Amount:              &pred.Amount,
		Source:              pred.Source,
	}, nil
}
