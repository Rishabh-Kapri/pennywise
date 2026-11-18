package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/llmusage"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
)

type fakeAgentRuns struct {
	runID    uuid.UUID
	startErr error
	started  []repository.StartJobRunParams
	recorded []repository.RecordAgentRunResult
	statuses []model.AgentRunStatus
	errors   []*string
}

func (f *fakeAgentRuns) StartJobRun(_ context.Context, p repository.StartJobRunParams) (*model.AgentRun, error) {
	f.started = append(f.started, p)
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &model.AgentRun{ID: f.runID}, nil
}

func (f *fakeAgentRuns) RecordRunResult(_ context.Context, r repository.RecordAgentRunResult) error {
	f.recorded = append(f.recorded, r)
	return nil
}

func (f *fakeAgentRuns) UpdateRunStatus(
	_ context.Context, _ uuid.UUID, _ uuid.UUID, status model.AgentRunStatus, msg *string,
) (*model.AgentRun, error) {
	f.statuses = append(f.statuses, status)
	f.errors = append(f.errors, msg)
	return &model.AgentRun{ID: f.runID}, nil
}

func TestWithAgentRunLinksCallsAndCompletes(t *testing.T) {
	runs := &fakeAgentRuns{runID: uuid.New()}
	budgetID := uuid.New()
	j := &job.Job{ID: uuid.New(), BudgetID: &budgetID, Type: model.JobTypeMessageExtract}

	var seen uuid.UUID
	h := withAgentRun(WorkerDeps{AgentRuns: runs}, func(ctx context.Context, _ *job.Job) (json.RawMessage, error) {
		seen, _ = llmusage.AgentRunFrom(ctx)
		return json.RawMessage(`{}`), nil
	})
	if _, err := h(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	if seen != runs.runID {
		t.Fatalf("model calls not linked to the run: got %v", seen)
	}
	if len(runs.started) != 1 || runs.started[0].JobID != j.ID || runs.started[0].AgentKey != model.AgentKeyPipeline {
		t.Fatalf("run not started for the job: %+v", runs.started)
	}
	if len(runs.recorded) != 1 || runs.recorded[0].StopReason != string(model.StopReasonEndTurn) {
		t.Fatalf("totals not rolled up: %+v", runs.recorded)
	}
	if len(runs.statuses) != 1 || runs.statuses[0] != model.AgentRunStatusCompleted {
		t.Fatalf("status = %v, want COMPLETED", runs.statuses)
	}
}

func TestWithAgentRunFailsWithTheHandler(t *testing.T) {
	runs := &fakeAgentRuns{runID: uuid.New()}
	budgetID := uuid.New()
	j := &job.Job{ID: uuid.New(), BudgetID: &budgetID}
	want := errors.New("extract failed")

	h := withAgentRun(WorkerDeps{AgentRuns: runs}, func(context.Context, *job.Job) (json.RawMessage, error) {
		return nil, want
	})
	if _, err := h(context.Background(), j); !errors.Is(err, want) {
		t.Fatalf("err = %v, want the handler's", err)
	}
	if runs.statuses[0] != model.AgentRunStatusFailed || runs.errors[0] == nil || *runs.errors[0] != want.Error() {
		t.Fatalf("status %v error %v, want FAILED with the handler's error", runs.statuses, runs.errors)
	}
	if runs.recorded[0].StopReason != string(model.StopReasonError) {
		t.Fatalf("stop reason = %q", runs.recorded[0].StopReason)
	}
}

func TestWithAgentRunStillRunsWhenStartFails(t *testing.T) {
	runs := &fakeAgentRuns{startErr: errors.New("db down")}
	budgetID := uuid.New()
	ran := false
	h := withAgentRun(WorkerDeps{AgentRuns: runs}, func(ctx context.Context, _ *job.Job) (json.RawMessage, error) {
		ran = true
		if _, ok := llmusage.AgentRunFrom(ctx); ok {
			t.Fatal("no run should be linked")
		}
		return nil, nil
	})
	if _, err := h(context.Background(), &job.Job{ID: uuid.New(), BudgetID: &budgetID}); err != nil {
		t.Fatal(err)
	}
	if !ran || len(runs.statuses) != 0 {
		t.Fatalf("ran=%v statuses=%v", ran, runs.statuses)
	}
}
