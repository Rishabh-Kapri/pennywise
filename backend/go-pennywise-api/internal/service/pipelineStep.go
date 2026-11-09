package service

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

// ErrUnknownPipelineStep: the pipeline or step ID isn't in the system pipelines.
var ErrUnknownPipelineStep = errors.New("unknown pipeline or step")

// UpdatePipelineStepRequest is the PUT body. nil = follow the default.
type UpdatePipelineStepRequest struct {
	Adapter  *string    `json:"adapter"`
	ModelID  *uuid.UUID `json:"modelId"`
	Fallback *bool      `json:"fallback"`
}

// PipelineStepService serves the system pipelines with a budget's overrides
// applied, and stores those overrides.
type PipelineStepService interface {
	List(ctx context.Context, budgetID uuid.UUID) (pipeline.CatalogView, error)
	Update(
		ctx context.Context,
		budgetID uuid.UUID,
		pipelineID string,
		step pipeline.StepID,
		req UpdatePipelineStepRequest,
	) (pipeline.StepView, error)
	Reset(ctx context.Context, budgetID uuid.UUID, pipelineID string, step pipeline.StepID) (pipeline.StepView, error)
}

type pipelineStepService struct {
	catalog   *pipeline.Catalog
	overrides repository.PipelineStepRepository
	models    repository.UserLLMModelRepository
}

func NewPipelineStepService(
	catalog *pipeline.Catalog,
	overrides repository.PipelineStepRepository,
	models repository.UserLLMModelRepository,
) PipelineStepService {
	return &pipelineStepService{catalog: catalog, overrides: overrides, models: models}
}

func (s *pipelineStepService) List(ctx context.Context, budgetID uuid.UUID) (pipeline.CatalogView, error) {
	overrides, err := s.overrides.ListByBudget(ctx, budgetID)
	if err != nil {
		return pipeline.CatalogView{}, err
	}
	models, err := s.models.ListUsableForBudget(ctx, budgetID)
	if err != nil {
		return pipeline.CatalogView{}, err
	}
	return pipeline.BuildCatalogView(s.catalog, overrides, models), nil
}

func (s *pipelineStepService) step(pipelineID string, stepID pipeline.StepID) (pipeline.Step, error) {
	p, ok := s.catalog.Pipeline(pipelineID)
	if !ok {
		return pipeline.Step{}, ErrUnknownPipelineStep
	}
	step, ok := p.Step(stepID)
	if !ok {
		return pipeline.Step{}, ErrUnknownPipelineStep
	}
	return step, nil
}

func (s *pipelineStepService) Update(
	ctx context.Context,
	budgetID uuid.UUID,
	pipelineID string,
	stepID pipeline.StepID,
	req UpdatePipelineStepRequest,
) (pipeline.StepView, error) {
	step, err := s.step(pipelineID, stepID)
	if err != nil {
		return pipeline.StepView{}, err
	}
	in := model.PipelineStepOverride{
		BudgetID: budgetID,
		Pipeline: pipelineID,
		Step:     string(stepID),
		Adapter:  blankToNil(req.Adapter),
		ModelID:  req.ModelID,
		Fallback: req.Fallback,
	}
	// Choosing the default adapter is the same as not choosing one, so the
	// step keeps following the master if its default changes.
	if in.Adapter != nil && *in.Adapter == step.DefaultAdapter {
		in.Adapter = nil
	}
	if in.ModelID != nil && *in.ModelID == uuid.Nil {
		in.ModelID = nil
	}
	if in.Fallback != nil && *in.Fallback == step.Fallback {
		in.Fallback = nil
	}
	if in.Adapter == nil && in.ModelID == nil && in.Fallback == nil {
		return s.Reset(ctx, budgetID, pipelineID, stepID)
	}

	models, err := s.models.ListUsableForBudget(ctx, budgetID)
	if err != nil {
		return pipeline.StepView{}, err
	}
	if err := pipeline.ValidateOverride(step, in, models); err != nil {
		return pipeline.StepView{}, err
	}
	saved, err := s.overrides.Upsert(ctx, in)
	if err != nil {
		return pipeline.StepView{}, err
	}
	return pipeline.Resolve(step, saved, models), nil
}

func (s *pipelineStepService) Reset(
	ctx context.Context,
	budgetID uuid.UUID,
	pipelineID string,
	stepID pipeline.StepID,
) (pipeline.StepView, error) {
	step, err := s.step(pipelineID, stepID)
	if err != nil {
		return pipeline.StepView{}, err
	}
	if err := s.overrides.Delete(ctx, budgetID, pipelineID, string(stepID)); err != nil {
		return pipeline.StepView{}, err
	}
	models, err := s.models.ListUsableForBudget(ctx, budgetID)
	if err != nil {
		return pipeline.StepView{}, err
	}
	return pipeline.Resolve(step, nil, models), nil
}

func blankToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}
