package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type PredictionActivity struct {
	PredictionService service.PredictionService
}

func (a *PredictionActivity) Predict(
	ctx context.Context,
	input sharedModel.ParsedEmailsInput,
) ([]sharedModel.CipherPredictionResult, error) {
	log := logger.Logger(ctx)

	var predictionResponse []sharedModel.CipherPredictionResult

	parsedEmails := input.ParsedEmails

	budgetId := input.BudgetID
	if budgetId == uuid.Nil {
		return nil, errs.New(errs.CodeInternalError, "Budget ID is required")
	}

	ctx = utils.WithBudgetID(ctx, budgetId)

	for _, email := range parsedEmails {
		log.Info("Predicting", "email", email, "amount", email.Amount, "date", email.Date)
		predictionInput := service.PredictRequest{
			EmailText: email.EmailText,
			Amount:    email.Amount,
		}
		prediction, err := a.PredictionService.Predict(ctx, predictionInput)
		if err != nil {
			log.Error("Prediction failed", "error", err)
			continue
		}
		log.Info("Prediction result", "result", prediction)
		predictionResponse = append(predictionResponse, sharedModel.CipherPredictionResult{
			PayeeID:    prediction.PayeeID,
			CategoryID: prediction.CategoryID,
			Payee:      prediction.Payee,
			Category:   prediction.Category,
			Amount:     email.Amount,
			Confidence: prediction.Confidence,
			Source:     prediction.Source,
			Reasoning:  prediction.Reasoning,
		})
	}

	return predictionResponse, nil
}
