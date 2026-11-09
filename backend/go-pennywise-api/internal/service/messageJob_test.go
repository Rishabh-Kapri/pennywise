package service

import (
	"context"
	"errors"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type fakeRunRepo struct {
	repository.PipelineRunRepository
	existing *model.PipelineRun
	started  model.StartPipelineRunInput
	updated  *model.ReportPipelineStatusInput
	events   []model.PipelineEventInput
	err      error
}

func (r *fakeRunRepo) CreateRun(_ context.Context, in model.StartPipelineRunInput) (*model.PipelineRun, error) {
	r.started = in
	if r.existing != nil {
		return r.existing, nil
	}
	return &model.PipelineRun{ID: uuid.New(), BudgetID: in.BudgetID}, nil
}

func (r *fakeRunRepo) UpdateRun(_ context.Context, in model.ReportPipelineStatusInput) (*model.PipelineRun, error) {
	r.updated = &in
	return &model.PipelineRun{ID: in.RunID, BudgetID: in.BudgetID}, r.err
}

func (r *fakeRunRepo) AppendEvents(_ context.Context, _ uuid.UUID, events []model.PipelineEventInput) error {
	r.events = events
	return nil
}

type fakeRunNotifier struct {
	WebsocketService
	events []string
	err    error
}

func (n *fakeRunNotifier) SendNotification(_ context.Context, _ uuid.UUID, name string, _ any) error {
	n.events = append(n.events, name)
	return n.err
}

func reportContext() (context.Context, uuid.UUID) {
	budget := uuid.New()
	return utils.WithBudgetID(context.Background(), budget), budget
}

func TestReportPipelineOutcomes(t *testing.T) {
	txn := uuid.New()
	tests := []struct {
		name    string
		payload model.PipelineReportPayload
		status  model.PipelineRunStatus
		step    model.PipelineStep
		last    model.PipelineStep
		lastIs  model.PipelineEventStatus
		errText string
	}{
		{
			name:    "not a transaction",
			payload: model.PipelineReportPayload{Outcome: model.MessageOutcomeNotTransaction},
			status:  model.PipelineRunStatusCompleted,
			step:    model.PipelineStepDone,
			last:    model.PipelineStepParse,
			lastIs:  model.PipelineEventSkipped,
		},
		{
			name:    "unverified",
			payload: model.PipelineReportPayload{Outcome: model.MessageOutcomeUnverified, Reason: "digits not in email"},
			status:  model.PipelineRunStatusFailed,
			step:    model.PipelineStepParse,
			last:    model.PipelineStepParse,
			lastIs:  model.PipelineEventFailed,
			errText: "digits not in email",
		},
		{
			name:    "no prediction",
			payload: model.PipelineReportPayload{Outcome: model.MessageOutcomeNoPrediction},
			status:  model.PipelineRunStatusFailed,
			step:    model.PipelineStepPredict,
			last:    model.PipelineStepPredict,
			lastIs:  model.PipelineEventFailed,
			errText: string(model.MessageOutcomeNoPrediction),
		},
		{
			name: "transaction created",
			payload: model.PipelineReportPayload{
				Outcome:        model.MessageOutcomeTransactionCreated,
				TransactionIDs: []uuid.UUID{txn},
			},
			status: model.PipelineRunStatusCompleted,
			step:   model.PipelineStepDone,
			last:   model.PipelineStepCreateTxns,
			lastIs: model.PipelineEventSucceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := reportContext()
			repo, ws := &fakeRunRepo{}, &fakeRunNotifier{}
			svc := NewMessageJobService(MessageJobDeps{PipelineRuns: repo, Websocket: ws, DBTxn: fakeDBTxn{}})

			if err := svc.ReportPipeline(ctx, tt.payload); err != nil {
				t.Fatal(err)
			}
			if repo.updated.RunStatus != tt.status || repo.updated.CurrentStep != tt.step {
				t.Fatalf("run %s at %s", repo.updated.RunStatus, repo.updated.CurrentStep)
			}
			if tt.errText != "" && (repo.updated.Error == nil || *repo.updated.Error != tt.errText) {
				t.Fatalf("error %v", repo.updated.Error)
			}
			if repo.events[0].Step != model.PipelineStepFetchEmails {
				t.Fatalf("first event %+v", repo.events[0])
			}
			last := repo.events[len(repo.events)-1]
			if last.Step != tt.last || last.Status != tt.lastIs {
				t.Fatalf("last event %+v", last)
			}
			if len(ws.events) != 1 || ws.events[0] != model.EventPipelineUpdate {
				t.Fatalf("broadcast %v", ws.events)
			}
		})
	}
}

func TestReportPipelineRunIsPerMessage(t *testing.T) {
	ctx, budget := reportContext()
	repo := &fakeRunRepo{}
	svc := NewMessageJobService(MessageJobDeps{PipelineRuns: repo, Websocket: &fakeRunNotifier{}, DBTxn: fakeDBTxn{}})
	msg := model.ReceivedMessage{Source: "email", Provider: "gmail", ExternalID: "m1"}

	err := svc.ReportPipeline(ctx, model.PipelineReportPayload{Message: msg, Outcome: model.MessageOutcomeNotTransaction})
	if err != nil {
		t.Fatal(err)
	}
	if repo.started.WorkflowID != msg.IdempotencyKey(budget) || repo.started.Trigger != model.PipelineTriggerGmailPush {
		t.Fatalf("%+v", repo.started)
	}
}

func TestReportPipelineRepeatChangesNothing(t *testing.T) {
	ctx, _ := reportContext()
	done := time.Now()
	repo := &fakeRunRepo{existing: &model.PipelineRun{ID: uuid.New(), CompletedAt: &done}}
	ws := &fakeRunNotifier{}
	svc := NewMessageJobService(MessageJobDeps{PipelineRuns: repo, Websocket: ws, DBTxn: fakeDBTxn{}})

	err := svc.ReportPipeline(ctx, model.PipelineReportPayload{Outcome: model.MessageOutcomeNotTransaction})
	if err != nil {
		t.Fatal(err)
	}
	if repo.updated != nil || repo.events != nil || ws.events != nil {
		t.Fatalf("repeat wrote: %+v %v %v", repo.updated, repo.events, ws.events)
	}
}

func TestReportPipelineFailures(t *testing.T) {
	ctx, _ := reportContext()

	repo := &fakeRunRepo{err: errors.New("db down")}
	svc := NewMessageJobService(MessageJobDeps{PipelineRuns: repo, Websocket: &fakeRunNotifier{}, DBTxn: fakeDBTxn{}})
	err := svc.ReportPipeline(ctx, model.PipelineReportPayload{Outcome: model.MessageOutcomeNotTransaction})
	if !errs.HasCode(err, errs.CodePipelineReportFailed) {
		t.Fatalf("save failure: %v", err)
	}

	err = svc.ReportPipeline(ctx, model.PipelineReportPayload{Outcome: "weird"})
	if !errs.HasCode(err, errs.CodeInvalidArgument) {
		t.Fatalf("unknown outcome: %v", err)
	}

	ws := &fakeRunNotifier{err: errors.New("no socket")}
	svc = NewMessageJobService(MessageJobDeps{PipelineRuns: &fakeRunRepo{}, Websocket: ws, DBTxn: fakeDBTxn{}})
	err = svc.ReportPipeline(ctx, model.PipelineReportPayload{Outcome: model.MessageOutcomeNotTransaction})
	if err != nil {
		t.Fatalf("broadcast failure must not fail the job: %v", err)
	}
}
