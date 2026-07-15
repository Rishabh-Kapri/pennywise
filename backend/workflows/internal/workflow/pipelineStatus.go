package workflow

import (
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Pipeline status reporting is observability only: every helper here swallows
// activity errors so a failed status write can never fail the pipeline itself.

// withPipelineReportOptions targets the pipeline status activities hosted by
// go-pennywise-api (the service that owns the pipeline_runs table).
func withPipelineReportOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           sharedModel.PennywiseActivitiesTaskQueue,
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 3,
		},
		Summary: "Record pipeline run progress for the UI",
	})
}

// pipelineReporter carries the identifiers every status report needs. A zero
// runID disables reporting (e.g. when StartPipelineRun itself failed).
type pipelineReporter struct {
	runID    uuid.UUID
	budgetID uuid.UUID
}

func startPipelineRun(ctx workflow.Context, input sharedModel.StartPipelineRunInput) pipelineReporter {
	var runID uuid.UUID
	err := workflow.ExecuteActivity(withPipelineReportOptions(ctx), "StartPipelineRun", input).
		Get(ctx, &runID)
	if err != nil {
		workflow.GetLogger(ctx).Warn("failed to start pipeline run tracking", "error", err)
		return pipelineReporter{}
	}
	return pipelineReporter{runID: runID, budgetID: input.BudgetID}
}

func (r pipelineReporter) enabled() bool {
	return r.runID != uuid.Nil
}

func (r pipelineReporter) report(ctx workflow.Context, input sharedModel.ReportPipelineStatusInput) {
	if !r.enabled() {
		return
	}
	input.RunID = r.runID
	input.BudgetID = r.budgetID
	err := workflow.ExecuteActivity(withPipelineReportOptions(ctx), "ReportPipelineStatus", input).
		Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).
			Warn("failed to report pipeline status", "step", input.CurrentStep, "error", err)
	}
}

// reportFailed marks the whole run failed with a timeline event for the step.
func (r pipelineReporter) reportFailed(ctx workflow.Context, step sharedModel.PipelineStep, err error) {
	msg := err.Error()
	r.report(ctx, sharedModel.ReportPipelineStatusInput{
		RunStatus:   sharedModel.PipelineRunStatusFailed,
		CurrentStep: step,
		Error:       &msg,
		Events: []sharedModel.PipelineEventInput{{
			Step:   step,
			Status: sharedModel.PipelineEventFailed,
			Detail: map[string]any{"error": msg},
		}},
	})
}

// reportWaitingRetry marks the run parked on a manual retry signal.
func (r pipelineReporter) reportWaitingRetry(
	ctx workflow.Context,
	step sharedModel.PipelineStep,
	retrySignal string,
	err error,
) {
	msg := err.Error()
	r.report(ctx, sharedModel.ReportPipelineStatusInput{
		RunStatus:   sharedModel.PipelineRunStatusWaitingRetry,
		CurrentStep: step,
		Error:       &msg,
		Events: []sharedModel.PipelineEventInput{{
			Step:   step,
			Status: sharedModel.PipelineEventWaitingRetry,
			Detail: map[string]any{"error": msg, "retrySignal": retrySignal},
		}},
	})
}

// reportRetrySignaled flips the run back to running after a manual retry.
func (r pipelineReporter) reportRetrySignaled(ctx workflow.Context, step sharedModel.PipelineStep) {
	clearError := ""
	r.report(ctx, sharedModel.ReportPipelineStatusInput{
		RunStatus:   sharedModel.PipelineRunStatusRunning,
		CurrentStep: step,
		Error:       &clearError,
		Events: []sharedModel.PipelineEventInput{{
			Step:   step,
			Status: sharedModel.PipelineEventRetrySignaled,
		}},
	})
}
