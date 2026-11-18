package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/llmusage"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
)

// AgentRuns is the part of the agent repository a pipeline job's run needs.
type AgentRuns interface {
	StartJobRun(ctx context.Context, params repository.StartJobRunParams) (*model.AgentRun, error)
	RecordRunResult(ctx context.Context, result repository.RecordAgentRunResult) error
	UpdateRunStatus(
		ctx context.Context,
		id uuid.UUID,
		budgetID uuid.UUID,
		status model.AgentRunStatus,
		errorMessage *string,
	) (*model.AgentRun, error)
}

// runFinishTimeout bounds closing a run after the handler returns.
const runFinishTimeout = 5 * time.Second

// runErrorLimit cuts the error saved on a failed run.
const runErrorLimit = 500

// withAgentRun records the job as an agent run (agent_key pipeline): the run
// is started (or, on a retry, resumed) before the handler, its id is saved on
// the job, every model call the handler makes is linked to it in llm_usage,
// and when the handler returns the run's totals are rolled up and its status
// set. Recording is best effort: it never fails or holds up the job.
func withAgentRun(deps WorkerDeps, h jobrunner.HandlerFunc) jobrunner.HandlerFunc {
	if deps.AgentRuns == nil {
		return h
	}
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		if j.BudgetID == nil {
			return h(ctx, j)
		}
		budgetID := *j.BudgetID
		run, err := deps.AgentRuns.StartJobRun(ctx, repository.StartJobRunParams{
			JobID:    j.ID,
			BudgetID: budgetID,
			AgentKey: model.AgentKeyPipeline,
			Metadata: map[string]any{"jobType": j.Type},
		})
		if err != nil {
			logger.Logger(ctx).Warn("couldn't start the job's agent run", "job_id", j.ID, "error", err)
			return h(ctx, j)
		}

		out, handlerErr := h(llmusage.WithAgentRun(ctx, run.ID), j)

		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runFinishTimeout)
		defer cancel()
		finishAgentRun(finishCtx, deps.AgentRuns, run.ID, budgetID, handlerErr)
		return out, handlerErr
	}
}

func finishAgentRun(ctx context.Context, runs AgentRuns, runID, budgetID uuid.UUID, handlerErr error) {
	status, stop := model.AgentRunStatusCompleted, model.StopReasonEndTurn
	var message *string
	switch {
	case handlerErr == nil:
	case errors.Is(handlerErr, context.Canceled):
		status, stop = model.AgentRunStatusCancelled, model.StopReasonError
	default:
		status, stop = model.AgentRunStatusFailed, model.StopReasonError
		text := handlerErr.Error()
		if len(text) > runErrorLimit {
			text = text[:runErrorLimit]
		}
		message = &text
	}
	log := logger.Logger(ctx)
	if err := runs.RecordRunResult(ctx, repository.RecordAgentRunResult{
		RunID: runID, BudgetID: budgetID, StopReason: string(stop),
	}); err != nil {
		log.Warn("couldn't record the job's agent run totals", "run_id", runID, "error", err)
	}
	if _, err := runs.UpdateRunStatus(ctx, runID, budgetID, status, message); err != nil {
		log.Warn("couldn't close the job's agent run", "run_id", runID, "error", err)
	}
}
