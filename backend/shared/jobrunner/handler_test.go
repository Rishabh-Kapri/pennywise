package jobrunner

import (
	"context"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// recordingBroker records the options of the last Enqueue.
type recordingBroker struct {
	job.Broker
	jobType string
	opts    job.EnqueueOptions
	err     error
}

func (b *recordingBroker) Enqueue(
	_ context.Context, jobType string, payload []byte, opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	b.jobType = jobType
	o, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	b.opts = o
	return job.EnqueueResult{JobID: uuid.New()}, b.err
}

func TestBudgetContext(t *testing.T) {
	id := uuid.New()
	ctx, got, err := BudgetContext(context.Background(), &job.Job{BudgetID: &id})
	if err != nil || got != id {
		t.Fatalf("got %v, %v", got, err)
	}
	if fromCtx, err := utils.BudgetIDFromContext(ctx); err != nil || fromCtx != id {
		t.Fatalf("budget in ctx: %v, %v", fromCtx, err)
	}
	if _, _, err := BudgetContext(context.Background(), &job.Job{}); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("no budget: %v", err)
	}
}

func TestDecodePayload(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	got, err := DecodePayload[payload](&job.Job{Payload: []byte(`{"name":"a"}`)})
	if err != nil || got.Name != "a" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := DecodePayload[payload](&job.Job{Payload: []byte(`nope`)}); !errs.HasCode(err, errs.CodeJobInvalidPayload) {
		t.Fatalf("bad payload: %v", err)
	}
}

func TestEnqueueNextKeepsChain(t *testing.T) {
	budget, root := uuid.New(), uuid.New()

	for name, tc := range map[string]struct {
		rootID *uuid.UUID
		parent job.Job
	}{
		"parent is the root": {parent: job.Job{ID: uuid.New(), BudgetID: &budget}},
		"parent has a root":  {parent: job.Job{ID: uuid.New(), BudgetID: &budget, RootJobID: &root}, rootID: &root},
	} {
		t.Run(name, func(t *testing.T) {
			b := &recordingBroker{}
			if _, err := EnqueueNext(context.Background(), b, &tc.parent, "next", "q", "key", map[string]string{}); err != nil {
				t.Fatal(err)
			}
			want := tc.parent.ID
			if tc.rootID != nil {
				want = *tc.rootID
			}
			if b.opts.Queue != "q" || b.opts.IdempotencyKey != "key" || *b.opts.BudgetID != budget ||
				*b.opts.RootJobID != want || *b.opts.ParentID != tc.parent.ID {
				t.Fatalf("%+v", b.opts)
			}
		})
	}
}

func TestEnqueueNextFailure(t *testing.T) {
	b := &recordingBroker{err: errs.New(errs.CodeInternalError, "down")}
	_, err := EnqueueNext(context.Background(), b, &job.Job{ID: uuid.New()}, "next", "q", "key", struct{}{})
	if !errs.HasCode(err, errs.CodeJobEnqueueFailed) {
		t.Fatalf("got %v", err)
	}
}
