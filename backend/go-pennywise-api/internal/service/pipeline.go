package service

import (
	"context"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
	tc "go.temporal.io/sdk/client"
)

const (
	defaultPipelineRunsLimit = 20
	maxPipelineRunsLimit     = 100
)

type PipelineService interface {
	GetRuns(ctx context.Context, status string, limit int, offset int) ([]model.PipelineRun, error)
	GetRunDetail(ctx context.Context, runID uuid.UUID) (*model.PipelineRunDetail, error)
	RetryRun(ctx context.Context, runID uuid.UUID) (*model.PipelineRun, error)
}

type pipelineService struct {
	repo           repository.PipelineRunRepository
	temporalClient tc.Client // nil when Temporal is not configured
}

func NewPipelineService(repo repository.PipelineRunRepository, temporalClient tc.Client) PipelineService {
	return &pipelineService{repo: repo, temporalClient: temporalClient}
}

func (s *pipelineService) GetRuns(
	ctx context.Context,
	status string,
	limit int,
	offset int,
) ([]model.PipelineRun, error) {
	budgetId := utils.MustBudgetID(ctx)

	if limit <= 0 {
		limit = defaultPipelineRunsLimit
	}
	if limit > maxPipelineRunsLimit {
		limit = maxPipelineRunsLimit
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.GetRuns(ctx, budgetId, status, limit, offset)
}

func (s *pipelineService) GetRunDetail(
	ctx context.Context,
	runID uuid.UUID,
) (*model.PipelineRunDetail, error) {
	budgetId := utils.MustBudgetID(ctx)

	run, err := s.repo.GetRunByID(ctx, budgetId, runID)
	if err != nil {
		return nil, err
	}

	events, err := s.repo.GetRunEvents(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	return &model.PipelineRunDetail{Run: *run, Events: events}, nil
}

// RetryRun signals a workflow parked on a manual retry (parse or predict step).
// The run lookup is budget-scoped, so a user can only signal their own runs.
func (s *pipelineService) RetryRun(
	ctx context.Context,
	runID uuid.UUID,
) (*model.PipelineRun, error) {
	budgetId := utils.MustBudgetID(ctx)
	log := logger.Logger(ctx)

	run, err := s.repo.GetRunByID(ctx, budgetId, runID)
	if err != nil {
		return nil, err
	}

	if run.Status != model.PipelineRunStatusWaitingRetry {
		return nil, errs.New(errs.CodeInvalidArgument, "run is not waiting for a retry")
	}

	var signal string
	switch run.CurrentStep {
	case model.PipelineStepParse:
		signal = model.RetryEmailParseSignal
	case model.PipelineStepPredict:
		signal = model.RetryPredictSignal
	default:
		return nil, errs.New(errs.CodeInvalidArgument, "run step does not support retry")
	}

	if s.temporalClient == nil {
		return nil, errs.New(errs.CodeInternalError, "temporal is not configured")
	}

	// Parse/predict run in the child workflow when triggered via gmail push;
	// standalone (manual) runs park on their own workflow ID.
	targetWorkflowID := run.WorkflowID
	if run.ChildWorkflowID != nil && *run.ChildWorkflowID != "" {
		targetWorkflowID = *run.ChildWorkflowID
	}

	if err := s.temporalClient.SignalWorkflow(ctx, targetWorkflowID, "", signal, nil); err != nil {
		log.Error("error sending pipeline retry signal",
			"error", err, "workflowId", targetWorkflowID, "signal", signal)
		return nil, errs.Wrap(errs.CodeInternalError, "error sending retry signal", err)
	}

	log.Info("pipeline retry signal sent", "workflowId", targetWorkflowID, "signal", signal, "runId", run.ID)
	return run, nil
}
