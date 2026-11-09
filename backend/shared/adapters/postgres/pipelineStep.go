package postgres

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pipelineStepRepo struct {
	BaseRepository
}

func NewPipelineStepRepository(pool *pgxpool.Pool) repository.PipelineStepRepository {
	return &pipelineStepRepo{BaseRepository: NewBaseRepository(pool)}
}

const pipelineStepColumns = `budget_id, pipeline, step, adapter, model_id, fallback, updated_at`

func scanPipelineStep(row pgx.Row) (*model.PipelineStepOverride, error) {
	var o model.PipelineStepOverride
	if err := row.Scan(&o.BudgetID, &o.Pipeline, &o.Step, &o.Adapter, &o.ModelID, &o.Fallback, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *pipelineStepRepo) ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.PipelineStepOverride, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT `+pipelineStepColumns+`
		FROM budget_pipeline_steps
		WHERE budget_id = $1
	`, budgetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := []model.PipelineStepOverride{}
	for rows.Next() {
		o, err := scanPipelineStep(rows)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, *o)
	}
	return overrides, rows.Err()
}

func (r *pipelineStepRepo) ListByBudgetAndPipeline(
	ctx context.Context,
	budgetID uuid.UUID,
	pipeline string,
) ([]model.PipelineStepOverride, error) {
	rows, err := r.Executor(ctx).Query(ctx, `
		SELECT `+pipelineStepColumns+`
		FROM budget_pipeline_steps
		WHERE budget_id = $1 AND pipeline = $2
	`, budgetID, pipeline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := []model.PipelineStepOverride{}
	for rows.Next() {
		o, err := scanPipelineStep(rows)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, *o)
	}
	return overrides, rows.Err()
}

func (r *pipelineStepRepo) Upsert(ctx context.Context, in model.PipelineStepOverride) (*model.PipelineStepOverride, error) {
	return scanPipelineStep(r.Executor(ctx).QueryRow(ctx, `
		INSERT INTO budget_pipeline_steps (budget_id, pipeline, step, adapter, model_id, fallback)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (budget_id, pipeline, step) DO UPDATE SET
			adapter    = EXCLUDED.adapter,
			model_id   = EXCLUDED.model_id,
			fallback   = EXCLUDED.fallback,
			updated_at = now()
		RETURNING `+pipelineStepColumns,
		in.BudgetID, in.Pipeline, in.Step, in.Adapter, in.ModelID, in.Fallback,
	))
}

func (r *pipelineStepRepo) Delete(ctx context.Context, budgetID uuid.UUID, pipeline, step string) error {
	_, err := r.Executor(ctx).Exec(ctx, `
		DELETE FROM budget_pipeline_steps WHERE budget_id = $1 AND pipeline = $2 AND step = $3
	`, budgetID, pipeline, step)
	return err
}
