package temporal

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/llmusage"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/service"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
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

		if prediction == nil {
			log.Warn("No prediction result", "email", email)
			continue
		}

		prediction.Summary = summary
		log.Info("Prediction result", "result", prediction)

		predictionResponse = append(predictionResponse, sharedModel.CipherPredictionResult{
			MessageId:       email.MessageId,
			OriginalRawText: email.EmailText,
			Summary:         prediction.Summary,
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
		if emailData.Body == "" {
			continue
		}

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

// ----- Per-email activities -----
// One email per invocation so a bad email is skipped or retried on its own
// instead of failing the whole batch. The batch activities above stay
// registered for workflows started before the per-email rollout.

func activityLogger(ctx context.Context) *slog.Logger {
	activityInfo := activity.GetInfo(ctx)
	return logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)
}

// withHeartbeats forwards service-level progress reports (fired before each
// LLM/embedding step) to Temporal heartbeats, so a hung ollama call is
// detected at the HeartbeatTimeout instead of the whole StartToCloseTimeout.
func withHeartbeats(ctx context.Context) context.Context {
	activity.RecordHeartbeat(ctx, "started")
	return progress.With(ctx, func(step string) {
		activity.RecordHeartbeat(ctx, step)
	})
}

// ParseEmail extracts transaction data from a single raw email. Data problems
// (empty body, non-transaction email, unparseable date) come back as Skipped
// results; only infrastructure errors (ollama down) are returned as errors so
// Temporal's retry policy applies.
func (a *PredictionActivity) ParseEmail(
	ctx context.Context,
	input sharedModel.ParseEmailInput,
) (sharedModel.ParseEmailResult, error) {
	ctx = utils.WithServiceName(ctx, "cipher")
	log := activityLogger(ctx).With("step", sharedModel.PipelineStepParse, "messageId", input.Email.MessageId)

	if input.BudgetID == uuid.Nil {
		return sharedModel.ParseEmailResult{},
			temporal.NewNonRetryableApplicationError("budget id is required", "invalid_argument", nil)
	}
	ctx = utils.WithBudgetID(ctx, input.BudgetID)

	if input.Email.Body == "" {
		return sharedModel.ParseEmailResult{Skipped: true, SkipReason: "empty email body"}, nil
	}

	ctx = withHeartbeats(ctx)
	// Collect what the extraction cost in model calls/tokens; every return path
	// below reports it, so a skipped or failed email still accounts for its LLM
	// usage on the pipeline run.
	ctx, usage := llmusage.With(ctx)
	extracted, err := a.PredictionService.ExtractEmailData(
		ctx,
		service.ExtractEmailDataRequest{EmailHtml: input.Email.Body},
	)
	if err != nil {
		log.Error("error extracting email", "error", err)
		return sharedModel.ParseEmailResult{LLMCalls: usage.Calls()}, err
	}
	if extracted == nil {
		return sharedModel.ParseEmailResult{LLMCalls: usage.Calls()},
			errs.New(errs.CodeInternalError, "extraction returned no result")
	}

	if extracted.Skipped || extracted.Amount == 0 || extracted.AccountCard == "" || extracted.Date == "" {
		log.Info("email not a transaction", "extracted", *extracted)
		return sharedModel.ParseEmailResult{
			Skipped:    true,
			SkipReason: "not a transaction email",
			LLMCalls:   usage.Calls(),
		}, nil
	}

	date, err := time.Parse("2006-01-02", extracted.Date)
	if err != nil {
		// Bad data from the extractor is deterministic — retrying cannot fix it.
		log.Warn("unparseable extraction date, skipping email", "date", extracted.Date, "error", err)
		return sharedModel.ParseEmailResult{
			Skipped:    true,
			SkipReason: "unparseable date: " + extracted.Date,
			LLMCalls:   usage.Calls(),
		}, nil
	}

	transactionType := "debit"
	if extracted.Amount > 0 {
		transactionType = "credit"
	}

	return sharedModel.ParseEmailResult{
		Parsed: &sharedModel.ParsedEmail{
			MessageId:         input.Email.MessageId,
			EmailText:         extracted.EmailText,
			ExtractedMerchant: extracted.Merchant,
			ExtractedAccount:  extracted.AccountCard,
			Amount:            extracted.Amount,
			Date:              date.Format("2006-01-02"),
			TransactionType:   transactionType,
		},
		LLMCalls: usage.Calls(),
	}, nil
}

// PredictEmail predicts payee/category/account for a single parsed email.
// Unknown accounts are Skipped (data problem); LLM/DB failures return errors
// for Temporal retries.
func (a *PredictionActivity) PredictEmail(
	ctx context.Context,
	input sharedModel.PredictEmailInput,
) (sharedModel.PredictEmailResult, error) {
	ctx = utils.WithServiceName(ctx, "cipher")
	log := activityLogger(ctx).With("step", sharedModel.PipelineStepPredict, "messageId", input.Email.MessageId)

	if input.BudgetID == uuid.Nil {
		return sharedModel.PredictEmailResult{},
			temporal.NewNonRetryableApplicationError("budget id is required", "invalid_argument", nil)
	}
	ctx = utils.WithBudgetID(ctx, input.BudgetID)

	email := input.Email
	log.Info("predicting", "amount", email.Amount, "date", email.Date)

	ctx = withHeartbeats(ctx)
	ctx, usage := llmusage.With(ctx)
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
		log.Error("summarization failed", "error", err)
		return sharedModel.PredictEmailResult{LLMCalls: usage.Calls()}, err
	}

	prediction, err := a.PredictionService.Predict(ctx, predictionInput)
	if err != nil {
		var svcErr *errs.Error
		if errors.As(err, &svcErr) && svcErr.Code == errs.CodeAccountLookupFailed {
			log.Warn("no matching account, skipping email", "error", err)
			return sharedModel.PredictEmailResult{
				Skipped:    true,
				SkipReason: err.Error(),
				LLMCalls:   usage.Calls(),
			}, nil
		}
		log.Error("prediction failed", "error", err)
		return sharedModel.PredictEmailResult{LLMCalls: usage.Calls()}, err
	}
	if prediction == nil {
		log.Warn("no prediction result, skipping email")
		return sharedModel.PredictEmailResult{
			Skipped:    true,
			SkipReason: "predictor returned no result",
			LLMCalls:   usage.Calls(),
		}, nil
	}

	prediction.Summary = summary
	log.Info("prediction result", "result", prediction)

	return sharedModel.PredictEmailResult{
		Prediction: &sharedModel.CipherPredictionResult{
			MessageId:       email.MessageId,
			OriginalRawText: email.EmailText,
			Summary:         prediction.Summary,
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
		},
		LLMCalls: usage.Calls(),
	}, nil
}
