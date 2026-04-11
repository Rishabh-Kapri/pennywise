package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func EmailToTransactionWorkflow(ctx workflow.Context, input EmailWorflowInput) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	var parsedEmails []ParsedEmail
	err := workflow.ExecuteActivity(ctx, "FetchAndParseEmails", input).Get(ctx, &parsedEmails)
	if err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("Fetched emails", "count", len(parsedEmails))

	var predictions PredictedFields
	err = workflow.ExecuteActivity(ctx, "PredictFields", parsedEmails).Get(ctx, &predictions)
	if err != nil {
		return err
	}

	// err = workflow.ExecuteActivity(ctx, "CreateTransaction", CreateTransactionInput{
	// 	ParsedData: parsedEmails,
	// 	Predictions:  predictions,
	// }).Get(ctx, nil)

	// err = workflow.ExecuteActivity(ctx, "CreatePrediction", )

	return nil
}
