package jobrunner

import (
	"context"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	jobPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"

	"github.com/robfig/cron/v3"
)

const (
	schedulerInterval = time.Minute
	schedulerBatch    = 100
)

// NextCronRun is the first time after after that a five-field cron expression
// fires in timezone.
func NextCronRun(expression, timezone string, after time.Time) (time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, errs.Wrap(errs.CodeJobInvalidOption, "schedule timezone", err)
	}
	spec, err := cron.ParseStandard(expression)
	if err != nil {
		return time.Time{}, errs.Wrap(errs.CodeJobInvalidOption, "schedule cron", err)
	}
	return spec.Next(after.In(location)), nil
}

// RunScheduler queues a job for each due schedule until ctx is cancelled.
// Several can run at once: a tick is enqueued at most once.
func RunScheduler(ctx context.Context, store jobPort.ScheduleStore, broker jobPort.Broker) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()
	for {
		fireDue(ctx, store, broker, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// fireDue queues each due schedule's tick, then moves it to its next run. A
// crash between the two is safe: the next pass finds the tick already queued.
func fireDue(ctx context.Context, store jobPort.ScheduleStore, broker jobPort.Broker, now time.Time) {
	log := logger.Logger(ctx)
	due, err := store.ListDueSchedules(ctx, now, schedulerBatch)
	if err != nil {
		if ctx.Err() == nil {
			log.Error("list due schedules failed", "error", err)
		}
		return
	}
	for _, s := range due {
		next, err := NextCronRun(s.Cron, s.Timezone, now)
		if err != nil {
			log.Error("schedule has an invalid time", "schedule", s.Name, "error", err)
			continue
		}
		if _, err := broker.Enqueue(ctx, s.Type, s.Payload,
			jobPort.WithQueue(s.Queue),
			jobPort.WithBudgetID(s.BudgetID),
			jobPort.WithSchedule(s.ID, s.NextRunAt),
		); err != nil {
			log.Error("queue scheduled job failed", "schedule", s.Name, "error", err)
			continue
		}
		if err := store.AdvanceSchedule(ctx, s.ID, s.NextRunAt, next); err != nil {
			log.Error("advance schedule failed", "schedule", s.Name, "error", err)
		}
	}
}
