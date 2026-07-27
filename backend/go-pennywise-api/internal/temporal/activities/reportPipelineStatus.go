package temporal

import (
	"context"
	"log/slog"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.temporal.io/sdk/activity"
)

// PipelineStatusActivity persists pipeline run progress reported by the
// email-to-transaction workflows and broadcasts each change to the budget's
// websocket clients. It is observability only — callers treat failures as
// non-fatal so a status write can never break the pipeline itself.
type PipelineStatusActivity struct {
	PipelineRunRepo  repository.PipelineRunRepository
	WebsocketService service.WebsocketService
}

func (a *PipelineStatusActivity) StartPipelineRun(
	ctx context.Context,
	input sharedModel.StartPipelineRunInput,
) (uuid.UUID, error) {
	ctx = utils.WithServiceName(ctx, "pennywise-api")
	log := pipelineActivityLogger(ctx)

	if input.BudgetID == uuid.Nil {
		return uuid.Nil, errs.New(errs.CodeInvalidArgument, "budget id is required")
	}
	if input.WorkflowID == "" || input.WorkflowRunID == "" {
		return uuid.Nil, errs.New(errs.CodeInvalidArgument, "workflow id and run id are required")
	}

	run, err := a.PipelineRunRepo.CreateRun(ctx, input)
	if err != nil {
		log.Error("failed to create pipeline run", "error", err)
		return uuid.Nil, err
	}

	a.broadcast(ctx, run)
	log.Info("pipeline run started", "pipelineRunId", run.ID, "trigger", run.Trigger)
	return run.ID, nil
}

func (a *PipelineStatusActivity) ReportPipelineStatus(
	ctx context.Context,
	input sharedModel.ReportPipelineStatusInput,
) error {
	ctx = utils.WithServiceName(ctx, "pennywise-api")
	log := pipelineActivityLogger(ctx)

	if input.RunID == uuid.Nil || input.BudgetID == uuid.Nil {
		return errs.New(errs.CodeInvalidArgument, "run id and budget id are required")
	}

	var run *sharedModel.PipelineRun
	err := utils.WithTx(ctx, a.PipelineRunRepo.GetDB(), func(tx pgx.Tx) error {
		var err error
		run, err = a.PipelineRunRepo.UpdateRun(ctx, tx, input)
		if err != nil {
			return err
		}
		return a.PipelineRunRepo.AppendEvents(ctx, tx, input.RunID, input.Events)
	})
	if err != nil {
		log.Error("failed to report pipeline status", "pipelineRunId", input.RunID, "error", err)
		return err
	}

	a.broadcast(ctx, run)
	return nil
}

func (a *PipelineStatusActivity) broadcast(ctx context.Context, run *sharedModel.PipelineRun) {
	if a.WebsocketService == nil || run == nil {
		return
	}
	if err := a.WebsocketService.SendNotification(
		ctx,
		run.BudgetID,
		sharedModel.EventPipelineUpdate,
		run,
	); err != nil {
		logger.Logger(ctx).Warn("failed to broadcast pipeline update", "pipelineRunId", run.ID, "error", err)
	}
}

func pipelineActivityLogger(ctx context.Context) *slog.Logger {
	activityInfo := activity.GetInfo(ctx)
	return logger.Logger(ctx).With(
		"workflow_id", activityInfo.WorkflowExecution.ID,
		"workflow_run_id", activityInfo.WorkflowExecution.RunID,
		"activity_id", activityInfo.ActivityID,
		"activity_type", activityInfo.ActivityType.Name,
	)
}
