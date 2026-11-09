package job

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeBroker struct {
	Broker  // unimplemented methods panic
	enqueue EnqueueResult
	err     error
}

func (f *fakeBroker) Enqueue(context.Context, string, []byte, ...EnqueueOption) (EnqueueResult, error) {
	return f.enqueue, f.err
}

func (f *fakeBroker) MarkCompleted(context.Context, Claim, json.RawMessage, time.Duration) error {
	return f.err
}

func (f *fakeBroker) MarkPending(context.Context, *Job, Claim, string, time.Time) error { return f.err }

type fakeLog struct {
	events []JobEvent
	err    error
	ctxErr error
}

func (l *fakeLog) Append(ctx context.Context, e JobEvent) error {
	l.ctxErr = ctx.Err()
	l.events = append(l.events, e)
	return l.err
}

func TestObservedBrokerRecordsOnlyRealChanges(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()

	log := &fakeLog{}
	o := NewObservedBroker(&fakeBroker{enqueue: EnqueueResult{JobID: id}}, log)
	if _, err := o.Enqueue(ctx, "email.extract", nil); err != nil {
		t.Fatal(err)
	}
	if len(log.events) != 1 || log.events[0].Type != EventEnqueued || log.events[0].JobID != id {
		t.Fatalf("new job: events = %+v", log.events)
	}

	log = &fakeLog{}
	o = NewObservedBroker(&fakeBroker{enqueue: EnqueueResult{JobID: id, Duplicate: true}}, log)
	_, _ = o.Enqueue(ctx, "email.extract", nil)
	if len(log.events) != 0 {
		t.Fatalf("duplicate enqueue recorded %+v", log.events)
	}

	log = &fakeLog{}
	o = NewObservedBroker(&fakeBroker{err: errors.New("claim lost")}, log)
	_ = o.MarkCompleted(ctx, Claim{JobID: id}, nil, time.Second)
	if len(log.events) != 0 {
		t.Fatalf("failed call recorded %+v", log.events)
	}
}

func TestObservedBrokerHistoryIsBestEffort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // e.g. shutdown: the state change happened, history must still be written

	log := &fakeLog{err: errors.New("db down")}
	o := NewObservedBroker(&fakeBroker{}, log)
	runAt := time.Now().Add(time.Minute)
	if err := o.MarkPending(ctx, &Job{Attempt: 2, MaxAttempts: 3}, Claim{JobID: uuid.New(), WorkerID: "w-1", Attempt: 2}, "boom", runAt); err != nil {
		t.Fatalf("append failure leaked into the broker call: %v", err)
	}
	if log.ctxErr != nil {
		t.Fatalf("append ran on a cancelled ctx: %v", log.ctxErr)
	}
	e := log.events[0]
	if e.Type != EventRetryScheduled || e.Error != "boom" || e.NextRunAt == nil || !e.NextRunAt.Equal(runAt) ||
		e.Attempt != 2 || e.Actor != "w-1" {
		t.Fatalf("event = %+v", e)
	}
}
