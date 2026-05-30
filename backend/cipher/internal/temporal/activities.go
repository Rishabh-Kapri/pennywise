package temporal

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.temporal.io/sdk/activity"
)

type PredictionActivity struct {
	PredictionService service.PredictionService
}

func (a *PredictionActivity) Predict(
	ctx context.Context,
	input sharedModel.ParsedEmailsInput,
) ([]sharedModel.CipherPredictionResult, error) {
	ctx = utils.WithServiceName(ctx, "cipher")
	activityInfo := activity.GetInfo(ctx)
	log := logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)

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
		if email.ExtractedMerchant != "" && email.ExtractedAccount != "" && email.Date != "" {
			predictionInput.ExtractedInputs = &service.ExtractedInputs{
				Merchant: email.ExtractedMerchant,
				Account:  email.ExtractedAccount,
				Date:     email.Date,
			}
		}

		summary, err := a.PredictionService.SummarizeEmailText(ctx, email.EmailText)
		if err != nil {
			return nil, err
		}

		prediction, err := a.PredictionService.Predict(ctx, predictionInput)
		if err != nil {
			log.Error("Prediction failed", "error", err)
			continue
		}

		log.Info("Prediction result", "result", prediction)
		predictionResponse = append(predictionResponse, sharedModel.CipherPredictionResult{
			OriginalRawText: email.EmailText,
			Summary:         summary,
			AccountID:       prediction.AccountID,
			Account:         prediction.Account,
			PayeeID:         prediction.PayeeID,
			CategoryID:      prediction.CategoryID,
			Payee:           prediction.Payee,
			Category:        prediction.Category,
			Date:            email.Date,
			Amount:          email.Amount,
			Confidence:      prediction.Confidence,
			Source:          prediction.Source,
			Reasoning:       prediction.Reasoning,
			Metadata:        prediction.Metadata,
		})
	}

	return predictionResponse, nil
}

func (a *PredictionActivity) ParseEmailData(
	ctx context.Context,
	input sharedModel.EmailDataInput,
) (result sharedModel.ParsedEmailsInput, err error) {
	ctx = utils.WithServiceName(ctx, "cipher")

	activityInfo := activity.GetInfo(ctx)
	log := logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)

	budgetId := input.BudgetID
	if budgetId == uuid.Nil {
		return result, errs.New(errs.CodeInternalError, "Budget ID is required")
	}

	ctx = utils.WithBudgetID(ctx, budgetId)

	result = sharedModel.ParsedEmailsInput{
		ParsedEmails: make([]sharedModel.ParsedEmail, 0, len(input.EmailData)),
		BudgetID:     input.BudgetID,
	}

	for _, emailData := range input.EmailData {
		extracted, err := a.PredictionService.ExtractEmailData(
			ctx,
			service.ExtractEmailDataRequest{EmailHtml: emailData.Body},
		)
		if err != nil || extracted == nil {
			log.Error("error extracting email", "error", err)
			return result, err
		}

		if extracted.Amount == 0 || extracted.AccountCard == "" || extracted.Date == "" {
			log.Info("email not a transaction", "email", emailData.Body, "extracted", *extracted)
			continue
		}

		dateString := extracted.Date
		date, err := time.Parse("2006-01-02", dateString)
		if err != nil {
			log.Error("error parsing date", "error", err)
			return result, err
		}
		transactionType := "debit"
		if extracted.Amount > 0 {
			transactionType = "credit"
		}

		result.ParsedEmails = append(result.ParsedEmails, sharedModel.ParsedEmail{
			MessageId:         emailData.MessageId,
			EmailText:         extracted.EmailText,
			ExtractedMerchant: extracted.Merchant,
			ExtractedAccount:  extracted.AccountCard,
			Amount:            extracted.Amount,
			Date:              date.Format("2006-01-02"),
			TransactionType:   transactionType,
			Account:           "",
		})
	}

	return result, nil
}
