package workflow

import (
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func RefreshGmailWatchWorkflow(ctx workflow.Context) error {
	workflowInfo := workflow.GetInfo(ctx)
	logFields := []interface{}{
		"workflow_id", workflowInfo.WorkflowExecution.ID,
		"workflow_run_id", workflowInfo.WorkflowExecution.RunID,
	}
	workflow.GetLogger(ctx).Info("starting gmail watch refresh workflow", logFields...)

	pennywiseCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.PennywiseActivitiesTaskQueue,
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 3,
		},
	})

	var users []sharedModel.GoogleWatchUser
	if err := workflow.ExecuteActivity(pennywiseCtx, "ListGoogleUsersNeedingWatchRefresh").Get(pennywiseCtx, &users); err != nil {
		return err
	}
	workflow.GetLogger(ctx).Info("google users needing gmail watch refresh", append(logFields, "count", len(users))...)
	if len(users) == 0 {
		return nil
	}

	gmailCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.GmailActivitiesTaskQueue,
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 3,
		},
	})

	var updatedUsers []sharedModel.GoogleWatchUser
	if err := workflow.ExecuteActivity(gmailCtx, "GmailWatchCall", users).Get(gmailCtx, &updatedUsers); err != nil {
		return err
	}

	if err := workflow.ExecuteActivity(pennywiseCtx, "UpdateGmailWatchState", updatedUsers).Get(pennywiseCtx, nil); err != nil {
		return err
	}

	workflow.GetLogger(ctx).Info("gmail watch refresh workflow completed", append(logFields, "count", len(updatedUsers))...)
	return nil
}
