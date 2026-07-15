package workflow

import (
	"fmt"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedTemporal "github.com/Rishabh-Kapri/pennywise/backend/shared/temporal"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func EmailToTransactionWorkflow(ctx workflow.Context, input sharedModel.EmailToTransactionWorflowInput) error {
	workflowInfo := workflow.GetInfo(ctx)
	workflowMetadata := sharedTemporal.RequestMetadataFromWorkflowContext(ctx)
	workflowLogFields := []interface{}{
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"workflow_run_id", workflowInfo.WorkflowExecution.RunID,
	}
	if workflowMetadata.CorrelationID != "" {
		workflowLogFields = append(workflowLogFields, "correlation_id", workflowMetadata.CorrelationID)
	}
	if workflowMetadata.OriginService != "" {
		workflowLogFields = append(workflowLogFields, "origin_service", workflowMetadata.OriginService)
	}
	workflow.GetLogger(ctx).Info("starting email-to-transaction workflow", workflowLogFields...)

	// ----- Step 1: Fetch user data and update history id in Pennywise -----
	pennywiseCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.PennywiseActivitiesTaskQueue,
		StartToCloseTimeout: 300 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	var googleUser sharedModel.GoogleUserInfo
	err := workflow.ExecuteActivity(pennywiseCtx, "GetGoogleUserByEmail", input.Email).Get(pennywiseCtx, &googleUser)
	if err != nil {
		return err
	}

	updateHistoryInput := sharedModel.UpdateGmailHistoryInput{
		Email:           input.Email,
		OAuthClientType: googleUser.OAuthClientType,
		GmailHistoryID:  input.HistoryId,
	}
	if err := workflow.ExecuteActivity(pennywiseCtx, "UpdateGmailHistoryID", updateHistoryInput).
		Get(pennywiseCtx, nil); err != nil {
		return err
	}

	// ----- Step 2: Fetch emails data from Gmail using Pennywise-owned user data -----
	gmailCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.GmailActivitiesTaskQueue,
		StartToCloseTimeout: 300 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	fetchInput := sharedModel.FetchAndParseEmailsInput{
		Email:           input.Email,
		HistoryID:       googleUser.GmailHistoryID,
		RefreshToken:    googleUser.RefreshToken,
		OAuthClientType: googleUser.OAuthClientType,
		BudgetID:        googleUser.BudgetID,
	}

	var emailDataInput sharedModel.EmailDataInput
	err = workflow.ExecuteActivity(gmailCtx, "FetchEmailData", fetchInput).Get(gmailCtx, &emailDataInput)
	if err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("fetched emails", append(workflowLogFields, "count", len(emailDataInput.EmailData))...)
	if len(emailDataInput.EmailData) == 0 {
		return nil
	}

	// Start child workflow for steps 2-4 (Predict -> CreateTransaction -> CreateCipherPrediction)
	childWorkflowID := workflowInfo.WorkflowExecution.ID + "-parsed"
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: childWorkflowID,
	})
	workflow.GetLogger(ctx).Info("starting child workflow for parsed emails",
		append(workflowLogFields, "child_workflow_id", childWorkflowID)...)

	err = workflow.ExecuteChildWorkflow(childCtx, sharedModel.ParsedEmailToTransactionWorkflowName, emailDataInput).
		Get(childCtx, nil)
	if err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("email-to-transaction workflow completed", workflowLogFields...)
	return nil
}

// ParsedEmailToTransactionWorkflow runs steps 2-4 only: Predict, CreateTransaction,
// CreateCipherPrediction. It accepts pre-parsed email data and skips the Gmail fetch.
func ParsedEmailToTransactionWorkflow(ctx workflow.Context, input sharedModel.EmailDataInput) error {
	workflowInfo := workflow.GetInfo(ctx)
	workflowMetadata := sharedTemporal.RequestMetadataFromWorkflowContext(ctx)
	workflowLogFields := []interface{}{
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"workflow_run_id", workflowInfo.WorkflowExecution.RunID,
	}
	if workflowMetadata.CorrelationID != "" {
		workflowLogFields = append(workflowLogFields, "correlation_id", workflowMetadata.CorrelationID)
	}
	if workflowMetadata.OriginService != "" {
		workflowLogFields = append(workflowLogFields, "origin_service", workflowMetadata.OriginService)
	}
	workflow.GetLogger(ctx).Info("starting parsed-email-to-transaction workflow", workflowLogFields...)

	// Workflows started before the per-email rollout (possibly parked on a
	// retry signal for up to RetryPredictWaitTimeout) must replay the legacy
	// batch path; new executions process one email at a time.
	version := workflow.GetVersion(ctx, "per-email-pipeline", workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		var parsedEmailsInput sharedModel.ParsedEmailsInput
		if err := parseRawEmails(ctx, input, &parsedEmailsInput, workflowLogFields); err != nil {
			return err
		}

		if err := processParsedEmails(ctx, parsedEmailsInput, workflowLogFields); err != nil {
			return err
		}

		workflow.GetLogger(ctx).Info("parsed-email-to-transaction workflow completed", workflowLogFields...)
		return nil
	}

	if err := processEmailsIndividually(ctx, input, workflowLogFields); err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("parsed-email-to-transaction workflow completed", workflowLogFields...)
	return nil
}

// perEmailCipherOptions returns activity options for the per-email cipher
// activities: shorter, exponential retries (the old 10-minute fixed interval
// would serialize badly when applied per email). The activities heartbeat
// before every LLM/embedding step, so a hung ollama call surfaces at the
// HeartbeatTimeout instead of the full StartToCloseTimeout.
func perEmailCipherOptions(ctx workflow.Context, summary string) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue: sharedModel.CipherActivitiesTaskQueue,
		// Worst case per email: ~4 LLM round-trips at up to 3m each (cold model).
		StartToCloseTimeout: 15 * time.Minute,
		// Must exceed the per-LLM-call timeout (CIPHER_LLM_CALL_TIMEOUT, 3m default).
		HeartbeatTimeout: 4 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Minute,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
		Summary: summary,
	})
}

// processEmailsIndividually parses and predicts one email per activity so a
// bad email is skipped or retried on its own. Successful predictions are
// committed (idempotently) as soon as their round completes; only emails that
// exhausted activity retries park the workflow on a retry signal, and only
// those emails are re-run when the signal arrives.
func processEmailsIndividually(
	ctx workflow.Context,
	input sharedModel.EmailDataInput,
	workflowLogFields []interface{},
) error {
	logger := workflow.GetLogger(ctx)
	budgetID := input.BudgetID

	var skips []sharedModel.EmailSkip
	pendingParse := input.EmailData
	var pendingPredict []sharedModel.ParsedEmail

	for {
		// ----- Parse pending raw emails, one activity each -----
		var failedParse []sharedModel.EmailData
		parseCtx := perEmailCipherOptions(ctx, "Extract transaction data from one email")
		for _, email := range pendingParse {
			var result sharedModel.ParseEmailResult
			err := workflow.ExecuteActivity(parseCtx, "ParseEmail", sharedModel.ParseEmailInput{
				Email:    email,
				BudgetID: budgetID,
			}).Get(parseCtx, &result)
			switch {
			case err != nil:
				logger.Warn("email parse failed after retries",
					append(workflowLogFields, "step", sharedModel.PipelineStepParse, "message_id", email.MessageId, "error", err)...)
				failedParse = append(failedParse, email)
			case result.Skipped:
				skips = append(skips, sharedModel.EmailSkip{
					MessageId: email.MessageId,
					Step:      sharedModel.PipelineStepParse,
					Reason:    result.SkipReason,
				})
			case result.Parsed != nil:
				pendingPredict = append(pendingPredict, *result.Parsed)
			}
		}

		// ----- Predict pending parsed emails, one activity each -----
		var failedPredict []sharedModel.ParsedEmail
		var predictions []sharedModel.CipherPredictionResult
		predictCtx := perEmailCipherOptions(ctx, "Predict transaction for one email")
		for _, parsed := range pendingPredict {
			var result sharedModel.PredictEmailResult
			err := workflow.ExecuteActivity(predictCtx, "PredictEmail", sharedModel.PredictEmailInput{
				Email:    parsed,
				BudgetID: budgetID,
			}).Get(predictCtx, &result)
			switch {
			case err != nil:
				logger.Warn("email predict failed after retries",
					append(workflowLogFields, "step", sharedModel.PipelineStepPredict, "message_id", parsed.MessageId, "error", err)...)
				failedPredict = append(failedPredict, parsed)
			case result.Skipped:
				skips = append(skips, sharedModel.EmailSkip{
					MessageId: parsed.MessageId,
					Step:      sharedModel.PipelineStepPredict,
					Reason:    result.SkipReason,
				})
			case result.Prediction != nil:
				predictions = append(predictions, *result.Prediction)
			}
		}

		// ----- Commit this round's successes; the insert is deduped, so a
		// retried round can never create duplicate transactions -----
		if len(predictions) > 0 {
			pennywiseCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				TaskQueue:           sharedModel.PennywiseActivitiesTaskQueue,
				StartToCloseTimeout: 300 * time.Second,
				RetryPolicy: &temporal.RetryPolicy{
					InitialInterval: time.Second,
					MaximumAttempts: 5,
				},
			})
			var createdTransactions []sharedModel.Transaction
			err := workflow.ExecuteActivity(pennywiseCtx, "CreateTransactionAndCipherPrediction", sharedModel.PredictionResultInput{
				Predictions: predictions,
				BudgetID:    budgetID,
			}).Get(pennywiseCtx, &createdTransactions)
			if err != nil {
				return err
			}
			logger.Info("created transactions and cipher predictions",
				append(workflowLogFields, "step", sharedModel.PipelineStepCreateTxns, "count", len(createdTransactions))...)
		}

		pendingParse = failedParse
		pendingPredict = failedPredict
		if len(pendingParse) == 0 && len(pendingPredict) == 0 {
			break
		}

		// ----- Park only the failed emails and wait for a manual retry -----
		logger.Warn("emails failed after retries, waiting for retry signal",
			append(workflowLogFields,
				"failed_parse", messageIDs(pendingParse),
				"failed_predict", parsedMessageIDs(pendingPredict))...)
		if !awaitRetrySignal(ctx) {
			return temporal.NewApplicationError(
				fmt.Sprintf("emails unprocessed after retry window: parse=%v predict=%v",
					messageIDs(pendingParse), parsedMessageIDs(pendingPredict)),
				"emails_unprocessed",
			)
		}
		logger.Info("retry signal received, retrying failed emails", workflowLogFields...)
	}

	if len(skips) > 0 {
		logger.Info("emails skipped", append(workflowLogFields, "skips", skips)...)
	}
	return nil
}

// awaitRetrySignal parks the workflow until either retry signal arrives or the
// wait window expires. Returns true when signaled.
func awaitRetrySignal(ctx workflow.Context) bool {
	parseCh := workflow.GetSignalChannel(ctx, sharedModel.RetryEmailParseSignal)
	predictCh := workflow.GetSignalChannel(ctx, sharedModel.RetryPredictSignal)

	var signaled bool
	workflow.NewSelector(ctx).
		AddReceive(parseCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(ctx, nil)
			signaled = true
		}).
		AddReceive(predictCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(ctx, nil)
			signaled = true
		}).
		AddFuture(workflow.NewTimer(ctx, sharedModel.RetryPredictWaitTimeout), func(_ workflow.Future) {}).
		Select(ctx)
	return signaled
}

func messageIDs(emails []sharedModel.EmailData) []string {
	ids := make([]string, 0, len(emails))
	for _, email := range emails {
		ids = append(ids, email.MessageId)
	}
	return ids
}

func parsedMessageIDs(emails []sharedModel.ParsedEmail) []string {
	ids := make([]string, 0, len(emails))
	for _, email := range emails {
		ids = append(ids, email.MessageId)
	}
	return ids
}

func parseRawEmails(
	ctx workflow.Context,
	input sharedModel.EmailDataInput,
	parsedEmailsInput *sharedModel.ParsedEmailsInput,
	workflowLogFields []interface{},
) error {
	retrySignalCh := workflow.GetSignalChannel(ctx, sharedModel.RetryEmailParseSignal)

	for {
		cipherCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskQueue:           sharedModel.CipherActivitiesTaskQueue,
			StartToCloseTimeout: 300 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    sharedModel.PredictRetryInterval,
				BackoffCoefficient: 1.0, // fixed interval, not exponential
				MaximumAttempts:    3,   // 3 total attempts (1 initial + 2 retries)
			},
			Summary: "Parse email data from the raw email text",
		})

		parseErr := workflow.ExecuteActivity(cipherCtx, "ParseEmailData", input).Get(cipherCtx, parsedEmailsInput)
		if parseErr == nil {
			break
		}
		workflow.GetLogger(ctx).
			Info("Parsing email data failed after retries, waiting for retry signal", append(workflowLogFields, "error", parseErr)...)

		var gotSignal bool
		workflow.NewSelector(ctx).AddReceive(retrySignalCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(ctx, nil)
			gotSignal = true
		}).AddFuture(workflow.NewTimer(ctx, sharedModel.RetryPredictWaitTimeout), func(_ workflow.Future) {}).Select(ctx)

		if !gotSignal {
			// Timer fired — no retry signal arrived within the wait window.
			workflow.GetLogger(ctx).
				Error("no retry signal received withing wait window, failing workflow", workflowLogFields...)
			return parseErr
		}
		workflow.GetLogger(ctx).Info("retry-parse signal received, retrying ParseEmailData", workflowLogFields...)
	}

	workflow.GetLogger(ctx).
		Info("parse-email-data workflow completed", append(workflowLogFields, "result", parsedEmailsInput)...)

	// carry on with the rest of the workflow
	return nil
}

// processParsedEmails executes steps 2-4 (Predict -> CreateTransaction -> CreateCipherPrediction).
func processParsedEmails(
	ctx workflow.Context,
	input sharedModel.ParsedEmailsInput,
	workflowLogFields []interface{},
) error {
	// ----- Step 2: Predict the transactions -----
	// Ollama may be temporarily unavailable. The retry policy waits PredictRetryInterval
	// between each automatic attempt. If all attempts are exhausted, the workflow parks on
	// a RetryPredictSignal so an operator can retrigger without re-submitting the email.
	retrySignalCh := workflow.GetSignalChannel(ctx, sharedModel.RetryPredictSignal)
	var predictionResult []sharedModel.CipherPredictionResult

	for {
		cipherCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskQueue:           sharedModel.CipherActivitiesTaskQueue,
			StartToCloseTimeout: 300 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    sharedModel.PredictRetryInterval,
				BackoffCoefficient: 1.0, // fixed interval, not exponential
				MaximumAttempts:    3,   // 3 total attempts (1 initial + 2 retries)
			},
			Summary: "Predict transaction from the parsed email data",
		})

		predictErr := workflow.ExecuteActivity(cipherCtx, "Predict", input).Get(cipherCtx, &predictionResult)
		if predictErr == nil {
			break
		}

		// All automatic retries exhausted. Park the workflow and wait for a manual
		// retry signal (e.g. sent once Ollama is back online).
		workflow.GetLogger(ctx).Warn("Predict failed after retries, waiting for retry signal",
			append(workflowLogFields, "error", predictErr)...)

		var gotSignal bool
		workflow.NewSelector(ctx).AddReceive(retrySignalCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(ctx, nil)
			gotSignal = true
		}).AddFuture(workflow.NewTimer(ctx, sharedModel.RetryPredictWaitTimeout), func(_ workflow.Future) {}).Select(ctx)

		if !gotSignal {
			// Timer fired — no retry signal arrived within the wait window.
			workflow.GetLogger(ctx).
				Error("no retry signal received within wait window, failing workflow", workflowLogFields...)
			return predictErr
		}
		workflow.GetLogger(ctx).Info("retry-predict signal received, retrying Predict", workflowLogFields...)
	}
	workflow.GetLogger(ctx).Info("prediction result", append(workflowLogFields, "result", predictionResult)...)

	// ----- Step 3: Create transactions and cipher predictions atomically -----
	pennywiseCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.PennywiseActivitiesTaskQueue,
		StartToCloseTimeout: 300 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	txnInput := sharedModel.PredictionResultInput{
		Predictions: predictionResult,
		BudgetID:    input.BudgetID,
	}

	var createdTransactions []sharedModel.Transaction
	err := workflow.ExecuteActivity(pennywiseCtx, "CreateTransactionAndCipherPrediction", txnInput).
		Get(pennywiseCtx, &createdTransactions)
	if err != nil {
		return err
	}
	workflow.GetLogger(ctx).Info("created transactions and cipher predictions",
		append(workflowLogFields, "count", len(createdTransactions))...)

	return nil
}
