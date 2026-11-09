package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// PipelineStepRepository stores budgets' overrides of the system pipelines
// (table budget_pipeline_steps). A budget has at most one row per pipeline
// step.
type PipelineStepRepository interface {
	// ListByBudget returns the budget's overrides in every pipeline.
	ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.PipelineStepOverride, error)
	// ListByBudgetAndPipeline returns the budget's overrides for specific pipeline.
	ListByBudgetAndPipeline(ctx context.Context, budgetID uuid.UUID, pipeline string) ([]model.PipelineStepOverride, error)
	// Upsert writes the whole row for (in.BudgetID, in.Pipeline, in.Step).
	Upsert(ctx context.Context, in model.PipelineStepOverride) (*model.PipelineStepOverride, error)
	// Delete removes the row. Deleting a row that doesn't exist is not an error.
	Delete(ctx context.Context, budgetID uuid.UUID, pipeline, step string) error
}
