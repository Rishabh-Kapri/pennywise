package jobrunner

import (
	"context"
	"errors"
	"testing"
	"time"

	jobPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
)

type fakeSchedules struct {
	jobPort.ScheduleStore
	due      []jobPort.JobSchedule
	advanced map[uuid.UUID]time.Time
}

func (f *fakeSchedules) ListDueSchedules(context.Context, time.Time, int) ([]jobPort.JobSchedule, error) {
	return f.due, nil
}

func (f *fakeSchedules) AdvanceSchedule(_ context.Context, id uuid.UUID, _, to time.Time) error {
	if f.advanced == nil {
		f.advanced = map[uuid.UUID]time.Time{}
	}
	f.advanced[id] = to
	return nil
}

type scheduleBroker struct {
	jobPort.Broker
	got []jobPort.EnqueueOptions
	err error
}

func (b *scheduleBroker) Enqueue(
	_ context.Context, jobType string, payload []byte, opts ...jobPort.EnqueueOption,
) (jobPort.EnqueueResult, error) {
	o, err := jobPort.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return jobPort.EnqueueResult{}, err
	}
	b.got = append(b.got, o)
	return jobPort.EnqueueResult{JobID: uuid.New()}, b.err
}

func TestFireDueQueuesTheTickAndMovesOn(t *testing.T) {
	now := time.Date(2026, 11, 8, 3, 1, 0, 0, time.UTC)
	tick := time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC)
	id := uuid.New()
	store := &fakeSchedules{due: []jobPort.JobSchedule{{
		ID: id, Name: "s", Type: "t", Queue: "q", Cron: "0 3 * * *", Timezone: "UTC", NextRunAt: tick,
	}}}
	broker := &scheduleBroker{}

	fireDue(context.Background(), store, broker, now)

	if len(broker.got) != 1 || broker.got[0].Queue != "q" || *broker.got[0].ScheduleID != id || !broker.got[0].ScheduledFor.Equal(tick) {
		t.Fatalf("queued %+v", broker.got)
	}
	if want := tick.Add(24 * time.Hour); !store.advanced[id].Equal(want) {
		t.Fatalf("advanced to %v, want %v", store.advanced[id], want)
	}
}

func TestFireDueDoesNotAdvanceWhenQueueingFails(t *testing.T) {
	id := uuid.New()
	store := &fakeSchedules{due: []jobPort.JobSchedule{{ID: id, Type: "t", Queue: "q", Cron: "0 3 * * *", Timezone: "UTC"}}}
	fireDue(context.Background(), store, &scheduleBroker{err: errors.New("down")}, time.Now())
	if _, moved := store.advanced[id]; moved {
		t.Fatal("a tick that wasn't queued must run again")
	}
}

func TestFireDueSkipsABadCronAndKeepsGoing(t *testing.T) {
	good := uuid.New()
	store := &fakeSchedules{due: []jobPort.JobSchedule{
		{ID: uuid.New(), Type: "t", Queue: "q", Cron: "nonsense", Timezone: "UTC"},
		{ID: good, Type: "t", Queue: "q", Cron: "0 3 * * *", Timezone: "UTC"},
	}}
	broker := &scheduleBroker{}
	fireDue(context.Background(), store, broker, time.Now())
	if len(broker.got) != 1 {
		t.Fatalf("queued %d", len(broker.got))
	}
	if _, moved := store.advanced[good]; !moved {
		t.Fatal("the good schedule should move on")
	}
}
