package postgres

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
)

func TestJobSchedules(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := NewPgJobScheduleAdapter(pool)
	broker := NewPgJobBrokerAdapter(pool)
	name := "test:" + uuid.NewString()
	queue := "test-" + uuid.NewString()
	first := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	schedule := job.JobSchedule{
		Name: name, Type: "t", Queue: queue, Payload: []byte(`{"a":1}`), Cron: "0 3 * * *", Timezone: "UTC", NextRunAt: first,
	}

	find := func() *job.JobSchedule {
		t.Helper()
		due, err := store.ListDueSchedules(ctx, time.Now(), 1000)
		if err != nil {
			t.Fatal(err)
		}
		for i := range due {
			if due[i].Name == name {
				return &due[i]
			}
		}
		return nil
	}

	if err := store.EnsureSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	found := find()
	if found == nil || found.Queue != queue || string(found.Payload) != `{"a": 1}` {
		t.Fatalf("found %+v", found)
	}

	// Ensuring one that's on keeps its next run.
	later := schedule
	later.NextRunAt = first.Add(48 * time.Hour)
	if err := store.EnsureSchedule(ctx, later); err != nil {
		t.Fatal(err)
	}
	if find() == nil {
		t.Fatal("ensure moved an enabled schedule's next run")
	}

	// A disabled one isn't due, and resuming it starts from the new time.
	if err := store.DisableSchedule(ctx, name); err != nil {
		t.Fatal(err)
	}
	if find() != nil {
		t.Fatal("a disabled schedule is due")
	}
	if err := store.EnsureSchedule(ctx, later); err != nil {
		t.Fatal(err)
	}
	if find() != nil {
		t.Fatal("resuming should use the new next run")
	}
	if err := store.DisableSchedule(ctx, "no-such-schedule"); err != nil {
		t.Fatal(err)
	}

	// A tick is queued once, and advancing is guarded by the expected time.
	resumed := schedule
	if err := store.DisableSchedule(ctx, name); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchedule(ctx, resumed); err != nil {
		t.Fatal(err)
	}
	due := find()
	opts := []job.EnqueueOption{job.WithQueue(queue), job.WithSchedule(due.ID, due.NextRunAt)}
	one, err := broker.Enqueue(ctx, "t", []byte(`{}`), opts...)
	if err != nil {
		t.Fatal(err)
	}
	two, err := broker.Enqueue(ctx, "t", []byte(`{}`), opts...)
	if err != nil || !two.Duplicate || two.JobID != one.JobID {
		t.Fatalf("second tick: %+v, %v", two, err)
	}
	next := time.Now().Add(time.Hour)
	if err := store.AdvanceSchedule(ctx, due.ID, due.NextRunAt.Add(time.Second), next); err != nil {
		t.Fatal(err)
	}
	if find() == nil {
		t.Fatal("a stale advance moved the schedule")
	}
	if err := store.AdvanceSchedule(ctx, due.ID, due.NextRunAt, next); err != nil {
		t.Fatal(err)
	}
	if find() != nil {
		t.Fatal("advance didn't move the schedule")
	}
}

func TestListSchedules(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := NewPgJobScheduleAdapter(pool)
	broker := NewPgJobBrokerAdapter(pool)
	mine, other := newSeed(t, pool).budgetID, newSeed(t, pool).budgetID
	tick := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)

	ensure := func(name string, budget *uuid.UUID, next time.Time) {
		t.Helper()
		if err := store.EnsureSchedule(ctx, job.JobSchedule{
			Name: name, BudgetID: budget, Type: "t", Queue: "q", Payload: []byte(`{}`),
			Cron: "0 3 * * *", Timezone: "UTC", NextRunAt: next,
		}); err != nil {
			t.Fatal(err)
		}
	}
	system, off, own, foreign, unrelated := "sys:"+uuid.NewString(), "off:"+uuid.NewString(),
		"own:"+uuid.NewString(), "foreign:"+uuid.NewString(), "unrelated:"+uuid.NewString()
	ensure(system, nil, tick.Add(2*time.Hour))
	ensure(off, nil, tick.Add(time.Minute))
	ensure(own, &mine, tick.Add(3*time.Hour))
	ensure(foreign, &other, tick)
	ensure(unrelated, nil, tick)
	if err := store.DisableSchedule(ctx, off); err != nil {
		t.Fatal(err)
	}

	scope := job.ScheduleScope{BudgetID: mine, SystemNames: []string{system, off}}
	list, err := store.ListSchedules(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(list))
	for i := range list {
		names[i] = list[i].Name
	}
	if want := []string{system, own, off}; !slices.Equal(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	if list[0].LastRun != nil {
		t.Fatalf("no run yet, got %+v", list[0].LastRun)
	}

	// The latest tick is the one reported, whichever was queued last.
	for _, at := range []time.Time{tick, tick.Add(-24 * time.Hour)} {
		if _, err := broker.Enqueue(ctx, "t", []byte(`{}`), job.WithQueue("q"), job.WithSchedule(list[0].ID, at)); err != nil {
			t.Fatal(err)
		}
	}
	list, err = store.ListSchedules(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	run := list[0].LastRun
	if run == nil || !run.ScheduledFor.Equal(tick) || run.Status != job.QueuedJobStatus {
		t.Fatalf("last run = %+v", run)
	}

	empty, err := store.ListSchedules(ctx, job.ScheduleScope{BudgetID: other})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 1 || empty[0].Name != foreign {
		t.Fatalf("other budget sees %+v", empty)
	}
}
