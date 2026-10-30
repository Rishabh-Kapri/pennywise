package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type PipelineRunRepository interface {
	CreateRun(ctx context.Context, input model.StartPipelineRunInput) (*model.PipelineRun, error)
	UpdateRun(ctx context.Context, input model.ReportPipelineStatusInput) (*model.PipelineRun, error)
	AppendEvents(ctx context.Context, runID uuid.UUID, events []model.PipelineEventInput) error
	GetRuns(ctx context.Context, budgetID uuid.UUID, status string, limit int, offset int) ([]model.PipelineRun, error)
	GetRunByID(ctx context.Context, budgetID uuid.UUID, runID uuid.UUID) (*model.PipelineRun, error)
	GetRunEvents(ctx context.Context, runID uuid.UUID) ([]model.PipelineRunEvent, error)
}
