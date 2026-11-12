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
}
