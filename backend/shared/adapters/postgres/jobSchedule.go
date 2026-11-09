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
