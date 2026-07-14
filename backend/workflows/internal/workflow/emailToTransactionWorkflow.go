package workflow

import (
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedTemporal "github.com/Rishabh-Kapri/pennywise/backend/shared/temporal"
	"github.com/google/uuid"

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

	// Budget is known now — start pipeline run tracking for the UI.
	reporter := startPipelineRun(ctx, sharedModel.StartPipelineRunInput{
		BudgetID:      googleUser.BudgetID,
		WorkflowID:    workflowInfo.WorkflowExecution.ID,
		WorkflowRunID: workflowInfo.WorkflowExecution.RunID,
		Trigger:       sharedModel.PipelineTriggerGmailPush,
		EmailAccount:  input.Email,
	})

	updateHistoryInput := sharedModel.UpdateGmailHistoryInput{
		Email:           input.Email,
		OAuthClientType: googleUser.OAuthClientType,
		GmailHistoryID:  input.HistoryId,
	}
	if err := workflow.ExecuteActivity(pennywiseCtx, "UpdateGmailHistoryID", updateHistoryInput).
		Get(pennywiseCtx, nil); err != nil {
		reporter.reportFailed(ctx, sharedModel.PipelineStepFetchUser, err)
		return err
	}

	// ----- Step 2: Fetch emails data from Gmail using Pennywise-owned user data -----
	reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
		CurrentStep: sharedModel.PipelineStepFetchEmails,
		Events: []sharedModel.PipelineEventInput{{
			Step:   sharedModel.PipelineStepFetchEmails,
			Status: sharedModel.PipelineEventStarted,
		}},
	})

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
		reporter.reportFailed(ctx, sharedModel.PipelineStepFetchEmails, err)
		return err
	}

	emailCount := len(emailDataInput.EmailData)
	workflow.GetLogger(ctx).Info("fetched emails", append(workflowLogFields, "count", emailCount)...)
	if emailCount == 0 {
		reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
			RunStatus:     sharedModel.PipelineRunStatusCompleted,
			CurrentStep:   sharedModel.PipelineStepDone,
			EmailsFetched: &emailCount,
			Events: []sharedModel.PipelineEventInput{{
				Step:   sharedModel.PipelineStepFetchEmails,
				Status: sharedModel.PipelineEventSucceeded,
				Detail: map[string]any{"count": emailCount},
			}},
		})
		return nil
	}

	// Start child workflow for steps 2-4 (Predict -> CreateTransaction -> CreateCipherPrediction)
	childWorkflowID := workflowInfo.WorkflowExecution.ID + "-parsed"
	reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
		ChildWorkflowID: childWorkflowID,
		EmailsFetched:   &emailCount,
		Events: []sharedModel.PipelineEventInput{{
			Step:   sharedModel.PipelineStepFetchEmails,
			Status: sharedModel.PipelineEventSucceeded,
			Detail: map[string]any{"count": emailCount},
		}},
	})

	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: childWorkflowID,
	})
	workflow.GetLogger(ctx).Info("starting child workflow for parsed emails",
		append(workflowLogFields, "child_workflow_id", childWorkflowID)...)

	emailDataInput.PipelineRunID = reporter.runID
	err = workflow.ExecuteChildWorkflow(childCtx, sharedModel.ParsedEmailToTransactionWorkflowName, emailDataInput).
		Get(childCtx, nil)
	if err != nil {
		// The child reports its own step-level failures; this catches the case
		// where the child could not run (or report) at all.
		msg := err.Error()
		reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
			RunStatus: sharedModel.PipelineRunStatusFailed,
			Error:     &msg,
		})
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

	// When started by the parent workflow the run row already exists; when
	// started standalone this workflow owns its own run with trigger "manual".
	reporter := pipelineReporter{runID: input.PipelineRunID, budgetID: input.BudgetID}
	if input.PipelineRunID == uuid.Nil {
		reporter = startPipelineRun(ctx, sharedModel.StartPipelineRunInput{
			BudgetID:      input.BudgetID,
			WorkflowID:    workflowInfo.WorkflowExecution.ID,
			WorkflowRunID: workflowInfo.WorkflowExecution.RunID,
			Trigger:       sharedModel.PipelineTriggerManual,
		})
		emailCount := len(input.EmailData)
		reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
			EmailsFetched: &emailCount,
		})
	}

	var parsedEmailsInput sharedModel.ParsedEmailsInput
	if err := parseRawEmails(ctx, input, &parsedEmailsInput, reporter, workflowLogFields); err != nil {
		return err
	}

	if err := processParsedEmails(ctx, parsedEmailsInput, reporter, workflowLogFields); err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("parsed-email-to-transaction workflow completed", workflowLogFields...)
	return nil
}

func parseRawEmails(
	ctx workflow.Context,
	input sharedModel.EmailDataInput,
	parsedEmailsInput *sharedModel.ParsedEmailsInput,
	reporter pipelineReporter,
	workflowLogFields []interface{},
) error {
	retrySignalCh := workflow.GetSignalChannel(ctx, sharedModel.RetryEmailParseSignal)

	reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
		CurrentStep: sharedModel.PipelineStepParse,
		Events: []sharedModel.PipelineEventInput{{
			Step:   sharedModel.PipelineStepParse,
			Status: sharedModel.PipelineEventStarted,
		}},
	})

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
		reporter.reportWaitingRetry(ctx, sharedModel.PipelineStepParse, sharedModel.RetryEmailParseSignal, parseErr)

		var gotSignal bool
		workflow.NewSelector(ctx).AddReceive(retrySignalCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(ctx, nil)
			gotSignal = true
		}).AddFuture(workflow.NewTimer(ctx, sharedModel.RetryPredictWaitTimeout), func(_ workflow.Future) {}).Select(ctx)

		if !gotSignal {
			// Timer fired — no retry signal arrived within the wait window.
			workflow.GetLogger(ctx).
				Error("no retry signal received withing wait window, failing workflow", workflowLogFields...)
			reporter.reportFailed(ctx, sharedModel.PipelineStepParse, parseErr)
			return parseErr
		}
		workflow.GetLogger(ctx).Info("retry-parse signal received, retrying ParseEmailData", workflowLogFields...)
		reporter.reportRetrySignaled(ctx, sharedModel.PipelineStepParse)
	}

	workflow.GetLogger(ctx).
		Info("parse-email-data workflow completed", append(workflowLogFields, "result", parsedEmailsInput)...)

	reporter.report(ctx, parseResultReport(input, *parsedEmailsInput))

	// carry on with the rest of the workflow
	return nil
}

// parseResultReport builds per-email timeline events for the parse step:
// succeeded for every extracted transaction, skipped for emails the extractor
// decided are not transactions.
func parseResultReport(
	input sharedModel.EmailDataInput,
	parsed sharedModel.ParsedEmailsInput,
) sharedModel.ReportPipelineStatusInput {
	events := make([]sharedModel.PipelineEventInput, 0, len(input.EmailData))
	parsedByMessageID := make(map[string]bool, len(parsed.ParsedEmails))

	for _, email := range parsed.ParsedEmails {
		parsedByMessageID[email.MessageId] = true
		events = append(events, sharedModel.PipelineEventInput{
			Step:      sharedModel.PipelineStepParse,
			Status:    sharedModel.PipelineEventSucceeded,
			MessageID: email.MessageId,
			Detail: map[string]any{
				"merchant":        email.ExtractedMerchant,
				"account":         email.ExtractedAccount,
				"amount":          email.Amount,
				"date":            email.Date,
				"transactionType": email.TransactionType,
			},
		})
	}

	skipped := 0
	for _, email := range input.EmailData {
		if !parsedByMessageID[email.MessageId] {
			skipped++
			events = append(events, sharedModel.PipelineEventInput{
				Step:      sharedModel.PipelineStepParse,
				Status:    sharedModel.PipelineEventSkipped,
				MessageID: email.MessageId,
				Detail:    map[string]any{"reason": "not a transaction email"},
			})
		}
	}

	return sharedModel.ReportPipelineStatusInput{
		EmailsSkipped: &skipped,
		Events:        events,
	}
}

// processParsedEmails executes steps 2-4 (Predict -> CreateTransaction -> CreateCipherPrediction).
func processParsedEmails(
	ctx workflow.Context,
	input sharedModel.ParsedEmailsInput,
	reporter pipelineReporter,
	workflowLogFields []interface{},
) error {
	// ----- Step 2: Predict the transactions -----
	// Ollama may be temporarily unavailable. The retry policy waits PredictRetryInterval
	// between each automatic attempt. If all attempts are exhausted, the workflow parks on
	// a RetryPredictSignal so an operator can retrigger without re-submitting the email.
	retrySignalCh := workflow.GetSignalChannel(ctx, sharedModel.RetryPredictSignal)
	var predictionResult []sharedModel.CipherPredictionResult

	reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
		CurrentStep: sharedModel.PipelineStepPredict,
		Events: []sharedModel.PipelineEventInput{{
			Step:   sharedModel.PipelineStepPredict,
			Status: sharedModel.PipelineEventStarted,
		}},
	})

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
		reporter.reportWaitingRetry(ctx, sharedModel.PipelineStepPredict, sharedModel.RetryPredictSignal, predictErr)

		var gotSignal bool
		workflow.NewSelector(ctx).AddReceive(retrySignalCh, func(ch workflow.ReceiveChannel, _ bool) {
			ch.Receive(ctx, nil)
			gotSignal = true
		}).AddFuture(workflow.NewTimer(ctx, sharedModel.RetryPredictWaitTimeout), func(_ workflow.Future) {}).Select(ctx)

		if !gotSignal {
			// Timer fired — no retry signal arrived within the wait window.
			workflow.GetLogger(ctx).
				Error("no retry signal received within wait window, failing workflow", workflowLogFields...)
			reporter.reportFailed(ctx, sharedModel.PipelineStepPredict, predictErr)
			return predictErr
		}
		workflow.GetLogger(ctx).Info("retry-predict signal received, retrying Predict", workflowLogFields...)
		reporter.reportRetrySignaled(ctx, sharedModel.PipelineStepPredict)
	}
	workflow.GetLogger(ctx).Info("prediction result", append(workflowLogFields, "result", predictionResult)...)

	reporter.report(ctx, predictResultReport(input, predictionResult))

	// ----- Step 3: Create transactions and cipher predictions atomically -----
	reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
		CurrentStep: sharedModel.PipelineStepCreateTransactions,
		Events: []sharedModel.PipelineEventInput{{
			Step:   sharedModel.PipelineStepCreateTransactions,
			Status: sharedModel.PipelineEventStarted,
		}},
	})

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
		reporter.reportFailed(ctx, sharedModel.PipelineStepCreateTransactions, err)
		return err
	}
	workflow.GetLogger(ctx).Info("created transactions and cipher predictions",
		append(workflowLogFields, "count", len(createdTransactions))...)

	transactionIDs := make([]string, 0, len(createdTransactions))
	for _, txn := range createdTransactions {
		transactionIDs = append(transactionIDs, txn.ID.String())
	}
	createdCount := len(createdTransactions)
	reporter.report(ctx, sharedModel.ReportPipelineStatusInput{
		RunStatus:           sharedModel.PipelineRunStatusCompleted,
		CurrentStep:         sharedModel.PipelineStepDone,
		TransactionsCreated: &createdCount,
		Events: []sharedModel.PipelineEventInput{{
			Step:   sharedModel.PipelineStepCreateTransactions,
			Status: sharedModel.PipelineEventSucceeded,
			Detail: map[string]any{"transactionIds": transactionIDs, "count": createdCount},
		}},
	})

	return nil
}

// predictResultReport builds per-email timeline events for the predict step.
// The Predict activity silently drops emails whose prediction failed, so any
// parsed email without a matching prediction is reported as failed.
func predictResultReport(
	input sharedModel.ParsedEmailsInput,
	predictions []sharedModel.CipherPredictionResult,
) sharedModel.ReportPipelineStatusInput {
	events := make([]sharedModel.PipelineEventInput, 0, len(input.ParsedEmails))
	predictedByMessageID := make(map[string]bool, len(predictions))

	for _, prediction := range predictions {
		predictedByMessageID[prediction.MessageId] = true
		events = append(events, sharedModel.PipelineEventInput{
			Step:      sharedModel.PipelineStepPredict,
			Status:    sharedModel.PipelineEventSucceeded,
			MessageID: prediction.MessageId,
			Detail: map[string]any{
				"payee":      prediction.Payee,
				"category":   prediction.Category,
				"account":    prediction.Account,
				"source":     prediction.Source,
				"confidence": prediction.Confidence,
				"reasoning":  prediction.Reasoning,
				"amount":     prediction.Amount,
				"date":       prediction.Date,
			},
		})
	}

	for _, email := range input.ParsedEmails {
		if !predictedByMessageID[email.MessageId] {
			events = append(events, sharedModel.PipelineEventInput{
				Step:      sharedModel.PipelineStepPredict,
				Status:    sharedModel.PipelineEventFailed,
				MessageID: email.MessageId,
				Detail:    map[string]any{"error": "prediction failed for this email"},
			})
		}
	}

	return sharedModel.ReportPipelineStatusInput{Events: events}
}
