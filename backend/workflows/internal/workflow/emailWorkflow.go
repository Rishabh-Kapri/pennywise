package workflow

import (
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func EmailToTransactionWorkflow(ctx workflow.Context, input sharedModel.EmailWorflowInput) error {
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
	workflow.GetLogger(ctx).Info("Fetched emails", "count", len(parsedEmails))
	if parsedEmails == nil || len(parsedEmails) == 0 {
		return nil
	}

	// ----- Step 2: Predict the transactions -----
	cipherCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.CipherActivitiesTaskQueue,
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
		Summary: "Predict transaction from the parsed email data",
	})
	var predictionResult []sharedModel.CipherPredictionResult

	err = workflow.ExecuteActivity(cipherCtx, "Predict", fetchAndParseEmailResult).Get(cipherCtx, &predictionResult)
	if err != nil {
		return err
	}
	workflow.GetLogger(ctx).Info("Prediction result", "result", predictionResult)

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
		BudgetID:    fetchAndParseEmailResult.BudgetID,
	}

	var createdTransactions []sharedModel.Transaction
	err = workflow.ExecuteActivity(pennywiseCtx, "CreateTransaction", txnInput).Get(pennywiseCtx, &createdTransactions)
	if err != nil {
		return err
	}

	// ----- Step 4: Create cipher predictions -----
	if len(createdTransactions) > 0 {
		predInput := sharedModel.CreateCipherPredictionInput{
			Transactions: createdTransactions,
			Predictions:  predictionResult,
			BudgetID:     fetchAndParseEmailResult.BudgetID,
		}
		err = workflow.ExecuteActivity(pennywiseCtx, "CreateCipherPrediction", predInput).Get(pennywiseCtx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
