package memory

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// jobLog is the local-testing job.JobLog: it prints each event to the
// console instead of storing it. Nothing is kept, so there is no history to
// query afterwards.
type jobLog struct{}

func NewMemoryJobLogAdapter() job.JobLog {
	return jobLog{}
}

// Append implements [job.JobLog]. It never fails.
func (jobLog) Append(ctx context.Context, e job.JobEvent) error {
	attrs := []any{
		"job_id", e.JobID,
		"event", e.Type,
		"status", e.Status,
		"attempt", e.Attempt,
		"actor", e.Actor,
	}
	if e.Error != "" {
		attrs = append(attrs, "error", e.Error)
	}
	if e.DurationMs != nil {
		attrs = append(attrs, "duration_ms", *e.DurationMs)
	}
	if e.NextRunAt != nil {
		attrs = append(attrs, "next_run_at", *e.NextRunAt)
	}
	logger.Logger(ctx).Info("job event", attrs...)
	return nil
}
