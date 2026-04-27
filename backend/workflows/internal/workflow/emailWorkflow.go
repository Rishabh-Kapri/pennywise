package workflow

import (
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedTemporal "github.com/Rishabh-Kapri/pennywise/backend/shared/temporal"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func EmailToTransactionWorkflow(ctx workflow.Context, input sharedModel.EmailWorflowInput) error {
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

	// ----- Step 1: Fetch emails data from the historyId -----
	gmailCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.GmailActivitiesTaskQueue,
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	var fetchAndParseEmailResult sharedModel.ParsedEmailsInput
	err := workflow.ExecuteActivity(gmailCtx, "FetchAndParseEmails", input).Get(gmailCtx, &fetchAndParseEmailResult)
	if err != nil {
		return err
	}

	parsedEmails := fetchAndParseEmailResult.ParsedEmails
	workflow.GetLogger(ctx).Info("fetched emails", append(workflowLogFields, "count", len(parsedEmails))...)
	if len(parsedEmails) == 0 {
		return nil
	}

	if len(parsedEmails) == 0 {
		return nil
	}

	// Start child workflow for steps 2-4 (Predict -> CreateTransaction -> CreateCipherPrediction)
	childWorkflowID := workflowInfo.WorkflowExecution.ID + "-parsed"
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: childWorkflowID,
	})
	workflow.GetLogger(ctx).Info("starting child workflow for parsed emails",
		append(workflowLogFields, "child_workflow_id", childWorkflowID)...)

	err = workflow.ExecuteChildWorkflow(childCtx, sharedModel.ParsedEmailToTransactionWorkflowName, fetchAndParseEmailResult).
		Get(childCtx, nil)
	if err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("email-to-transaction workflow completed", workflowLogFields...)
	return nil
}

// ParsedEmailToTransactionWorkflow runs steps 2-4 only: Predict, CreateTransaction,
// CreateCipherPrediction. It accepts pre-parsed email data and skips the Gmail fetch.
func ParsedEmailToTransactionWorkflow(ctx workflow.Context, input sharedModel.ParsedEmailsInput) error {
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

	if err := processParsedEmails(ctx, input, workflowLogFields); err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("parsed-email-to-transaction workflow completed", workflowLogFields...)
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
			StartToCloseTimeout: 90 * time.Second,
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

	// ----- Step 3: Create transactions -----
	pennywiseCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.PennywiseActivitiesTaskQueue,
		StartToCloseTimeout: 30 * time.Second,
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
	err := workflow.ExecuteActivity(pennywiseCtx, "CreateTransaction", txnInput).Get(pennywiseCtx, &createdTransactions)
	if err != nil {
		return err
	}

	// ----- Step 4: Create cipher predictions -----
	if len(createdTransactions) > 0 {
		predInput := sharedModel.CreateCipherPredictionInput{
			Transactions: createdTransactions,
			Predictions:  predictionResult,
			BudgetID:     input.BudgetID,
		}
		err = workflow.ExecuteActivity(pennywiseCtx, "CreateCipherPrediction", predInput).Get(pennywiseCtx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
