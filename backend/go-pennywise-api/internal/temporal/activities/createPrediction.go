package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
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
	log := logger.Logger(ctx)

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
		pred := input.Predictions[i]

		record := sharedModel.CipherPredictionRecord{
			BudgetID:            input.BudgetID,
			TransactionID:       txn.ID,
			EmailText:           txn.RawBankText,
			ExtractedAccount:    &pred.Account,
			ExtractedMerchant:   &pred.Payee,
			PredictedPayeeID:    &pred.PayeeID,
			PredictedCategoryID: &pred.CategoryID,
			Amount:              &pred.Amount,
			Source:              pred.Source,
		}

		log.Info("creating cipher prediction", "transactionId", txn.ID, "source", pred.Source)
		_, err := a.PredictionService.CreateCipherPrediction(ctx, record)
		if err != nil {
			return err
		}
	}

	return nil
}
