package postgres

import (
	"context"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type jobSchedules struct {
	BaseRepository
}

func NewPgJobScheduleAdapter(pool *pgxpool.Pool) job.ScheduleStore {
	return &jobSchedules{BaseRepository: NewBaseRepository(pool)}
}

func (s *jobSchedules) EnsureSchedule(ctx context.Context, in job.JobSchedule) error {
	_, err := s.Executor(ctx).Exec(ctx, `
		INSERT INTO job_schedules (name, budget_id, type, queue, payload, cron, timezone, enabled, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8)
		ON CONFLICT (name) DO UPDATE SET
			budget_id = EXCLUDED.budget_id,
			type = EXCLUDED.type,
			queue = EXCLUDED.queue,
			payload = EXCLUDED.payload,
			cron = EXCLUDED.cron,
			timezone = EXCLUDED.timezone,
			next_run_at = CASE WHEN job_schedules.enabled AND job_schedules.deleted_at IS NULL
				THEN job_schedules.next_run_at ELSE EXCLUDED.next_run_at END,
			enabled = true,
			deleted_at = NULL,
			updated_at = now()
	`, in.Name, in.BudgetID, in.Type, in.Queue, in.Payload, in.Cron, in.Timezone, in.NextRunAt)
	if err != nil {
		return errs.Wrap(errs.CodeJobUpdateFailed, "ensure job schedule", err, errs.WithDetail("name", in.Name))
	}
	return nil
}

func (s *jobSchedules) DisableSchedule(ctx context.Context, name string) error {
	_, err := s.Executor(ctx).Exec(ctx,
		`UPDATE job_schedules SET enabled = false, updated_at = now() WHERE name = $1`, name)
	if err != nil {
		return errs.Wrap(errs.CodeJobUpdateFailed, "disable job schedule", err, errs.WithDetail("name", name))
	}
	return nil
}

func (s *jobSchedules) ListDueSchedules(ctx context.Context, now time.Time, limit int) ([]job.JobSchedule, error) {
	rows, err := s.Executor(ctx).Query(ctx, `
		SELECT id, name, budget_id, type, queue, payload, cron, timezone, enabled, next_run_at, created_at, updated_at
		FROM job_schedules
		WHERE enabled AND deleted_at IS NULL AND next_run_at <= $1
		ORDER BY next_run_at
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list due schedules", err)
	}
	defer rows.Close()

	due := []job.JobSchedule{}
	for rows.Next() {
		var d job.JobSchedule
		if err := rows.Scan(&d.ID, &d.Name, &d.BudgetID, &d.Type, &d.Queue, &d.Payload, &d.Cron,
			&d.Timezone, &d.Enabled, &d.NextRunAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, errs.Wrap(errs.CodeJobLookupFailed, "scan due schedule", err)
		}
		due = append(due, d)
	}
	if err := rows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list due schedules", err)
	}
	return due, nil
}

func (s *jobSchedules) AdvanceSchedule(ctx context.Context, id uuid.UUID, from, to time.Time) error {
	_, err := s.Executor(ctx).Exec(ctx, `
		UPDATE job_schedules SET next_run_at = $3, updated_at = now()
		WHERE id = $1 AND next_run_at = $2`, id, from, to)
	if err != nil {
		return errs.Wrap(errs.CodeJobUpdateFailed, "advance job schedule", err)
	}
	return nil
}

// ListSchedules implements [job.ScheduleStore].
func (s *jobSchedules) ListSchedules(ctx context.Context, scope job.ScheduleScope) ([]job.ScheduleOverview, error) {
	rows, err := s.Executor(ctx).Query(ctx, `
		SELECT s.id, s.name, s.budget_id, s.type, s.queue, s.payload, s.cron, s.timezone, s.enabled,
			s.next_run_at, s.created_at, s.updated_at,
			r.id, r.status::text, r.scheduled_for, r.completed_at, r.last_error
		FROM job_schedules s
		LEFT JOIN LATERAL (
			SELECT id, status, scheduled_for, completed_at, last_error
			FROM jobs
			WHERE schedule_id = s.id AND deleted_at IS NULL
			ORDER BY scheduled_for DESC
			LIMIT 1
		) r ON true
		WHERE s.deleted_at IS NULL
			AND (s.budget_id = $1 OR (s.budget_id IS NULL AND s.name = ANY($2::text[])))
		ORDER BY s.enabled DESC, s.next_run_at, s.name`, scope.BudgetID, scope.SystemNames)
	if err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list schedules", err)
	}
	defer rows.Close()

	list := []job.ScheduleOverview{}
	for rows.Next() {
		var (
			o            job.ScheduleOverview
			runID        *uuid.UUID
			status       *string
			scheduledFor *time.Time
			completedAt  *time.Time
			lastError    *string
		)
		if err := rows.Scan(&o.ID, &o.Name, &o.BudgetID, &o.Type, &o.Queue, &o.Payload, &o.Cron, &o.Timezone,
			&o.Enabled, &o.NextRunAt, &o.CreatedAt, &o.UpdatedAt,
			&runID, &status, &scheduledFor, &completedAt, &lastError); err != nil {
			return nil, errs.Wrap(errs.CodeJobLookupFailed, "scan schedule", err)
		}
		if runID != nil {
			o.LastRun = &job.ScheduleRun{
				JobID:        *runID,
				Status:       job.JobStatus(*status),
				ScheduledFor: *scheduledFor,
				CompletedAt:  completedAt,
			}
			if lastError != nil {
				o.LastRun.LastError = *lastError
			}
		}
		list = append(list, o)
	}
	if err := rows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list schedules", err)
	}
	return list, nil
}
