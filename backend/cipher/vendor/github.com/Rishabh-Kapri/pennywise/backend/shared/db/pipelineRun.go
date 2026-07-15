package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PipelineRunRepository interface {
	BaseRepositoryInterface
	CreateRun(ctx context.Context, input model.StartPipelineRunInput) (*model.PipelineRun, error)
	UpdateRun(ctx context.Context, tx pgx.Tx, input model.ReportPipelineStatusInput) (*model.PipelineRun, error)
	AppendEvents(ctx context.Context, tx pgx.Tx, runID uuid.UUID, events []model.PipelineEventInput) error
	GetRuns(ctx context.Context, budgetID uuid.UUID, status string, limit int, offset int) ([]model.PipelineRun, error)
	GetRunByID(ctx context.Context, budgetID uuid.UUID, runID uuid.UUID) (*model.PipelineRun, error)
	GetRunEvents(ctx context.Context, runID uuid.UUID) ([]model.PipelineRunEvent, error)
}

type pipelineRunRepo struct {
	BaseRepository
}

func NewPipelineRunRepository(pool *pgxpool.Pool) PipelineRunRepository {
	return &pipelineRunRepo{BaseRepository: NewBaseRepository(pool)}
}

const pipelineRunColumns = `
	id, budget_id, workflow_id, workflow_run_id, child_workflow_id,
	trigger, email_account, status, current_step, error,
	emails_fetched, emails_skipped, transactions_created,
	started_at, updated_at, completed_at`

func scanPipelineRun(row pgx.Row) (*model.PipelineRun, error) {
	var run model.PipelineRun
	err := row.Scan(
		&run.ID,
		&run.BudgetID,
		&run.WorkflowID,
		&run.WorkflowRunID,
		&run.ChildWorkflowID,
		&run.Trigger,
		&run.EmailAccount,
		&run.Status,
		&run.CurrentStep,
		&run.Error,
		&run.EmailsFetched,
		&run.EmailsSkipped,
		&run.TransactionsCreated,
		&run.StartedAt,
		&run.UpdatedAt,
		&run.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// CreateRun inserts the run row. Idempotent on (workflow_id, workflow_run_id)
// so a retried StartPipelineRun activity returns the existing row.
func (r *pipelineRunRepo) CreateRun(
	ctx context.Context,
	input model.StartPipelineRunInput,
) (*model.PipelineRun, error) {
	return scanPipelineRun(r.Executor(nil).QueryRow(
		ctx,
		`INSERT INTO pipeline_runs (budget_id, workflow_id, workflow_run_id, trigger, email_account)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		 ON CONFLICT (workflow_id, workflow_run_id) DO UPDATE SET updated_at = now()
		 RETURNING `+pipelineRunColumns,
		input.BudgetID,
		input.WorkflowID,
		input.WorkflowRunID,
		input.Trigger,
		input.EmailAccount,
	))
}

// UpdateRun applies the non-zero run-level fields of the report and returns the
// updated row. Error semantics: nil pointer leaves the column unchanged, empty
// string clears it.
func (r *pipelineRunRepo) UpdateRun(
	ctx context.Context,
	tx pgx.Tx,
	input model.ReportPipelineStatusInput,
) (*model.PipelineRun, error) {
	return scanPipelineRun(r.Executor(tx).QueryRow(
		ctx,
		`UPDATE pipeline_runs SET
			status = COALESCE(NULLIF($3, '')::pipeline_run_status, status),
			current_step = COALESCE(NULLIF($4, ''), current_step),
			child_workflow_id = COALESCE(NULLIF($5, ''), child_workflow_id),
			error = CASE WHEN $6::text IS NULL THEN error ELSE NULLIF($6::text, '') END,
			emails_fetched = COALESCE($7, emails_fetched),
			emails_skipped = COALESCE($8, emails_skipped),
			transactions_created = COALESCE($9, transactions_created),
			completed_at = CASE
				WHEN NULLIF($3, '') IN ('completed', 'failed') THEN now()
				ELSE completed_at
			END,
			updated_at = now()
		 WHERE id = $1 AND budget_id = $2
		 RETURNING `+pipelineRunColumns,
		input.RunID,
		input.BudgetID,
		string(input.RunStatus),
		string(input.CurrentStep),
		input.ChildWorkflowID,
		input.Error,
		input.EmailsFetched,
		input.EmailsSkipped,
		input.TransactionsCreated,
	))
}

func (r *pipelineRunRepo) AppendEvents(
	ctx context.Context,
	tx pgx.Tx,
	runID uuid.UUID,
	events []model.PipelineEventInput,
) error {
	for _, event := range events {
		_, err := r.Executor(tx).Exec(
			ctx,
			// clock_timestamp() keeps rows appended in one transaction ordered
			// by actual insertion time instead of sharing the transaction time
			`INSERT INTO pipeline_run_events (run_id, step, status, message_id, detail, created_at)
			 VALUES ($1, $2, $3, NULLIF($4, ''), $5, clock_timestamp())`,
			runID,
			event.Step,
			event.Status,
			event.MessageID,
			event.Detail,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *pipelineRunRepo) GetRuns(
	ctx context.Context,
	budgetID uuid.UUID,
	status string,
	limit int,
	offset int,
) ([]model.PipelineRun, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT `+pipelineRunColumns+`
		 FROM pipeline_runs
		 WHERE budget_id = $1 AND ($2 = '' OR status::text = $2)
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`,
		budgetID,
		status,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []model.PipelineRun
	for rows.Next() {
		run, err := scanPipelineRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

func (r *pipelineRunRepo) GetRunByID(
	ctx context.Context,
	budgetID uuid.UUID,
	runID uuid.UUID,
) (*model.PipelineRun, error) {
	return scanPipelineRun(r.Executor(nil).QueryRow(
		ctx,
		`SELECT `+pipelineRunColumns+`
		 FROM pipeline_runs
		 WHERE id = $1 AND budget_id = $2`,
		runID,
		budgetID,
	))
}

func (r *pipelineRunRepo) GetRunEvents(
	ctx context.Context,
	runID uuid.UUID,
) ([]model.PipelineRunEvent, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT id, run_id, step, status, message_id, detail, created_at
		 FROM pipeline_run_events
		 WHERE run_id = $1
		 ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.PipelineRunEvent
	for rows.Next() {
		var event model.PipelineRunEvent
		err := rows.Scan(
			&event.ID,
			&event.RunID,
			&event.Step,
			&event.Status,
			&event.MessageID,
			&event.Detail,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
