package pipeline

import (
	"fmt"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

// ModelRef names one of the owner's models.
type ModelRef struct {
	ID       uuid.UUID             `json:"id"`
	Kind     model.LLMModelKind    `json:"kind"`
	Provider model.LLMProviderName `json:"provider"`
	Slug     string                `json:"slug"`
}

func refOf(m model.UserLLMModel) *ModelRef {
	return &ModelRef{ID: m.ID, Kind: m.Kind, Provider: m.Provider, Slug: m.Slug}
}

// Choice is what a step runs with. Model is set only when the adapter uses a
// model and the owner has one of its kind.
type Choice struct {
	Adapter string    `json:"adapter"`
	Model   *ModelRef `json:"model,omitempty"`
	// Fallback: when Model fails, try the owner's other models of its kind, by
	// priority. Always false for adapters that don't use a model.
	Fallback bool `json:"fallback"`
}

// OverrideView is a stored override as the UI sees it.
type OverrideView struct {
	Adapter   *string    `json:"adapter"`
	ModelID   *uuid.UUID `json:"modelId"`
	Fallback  *bool      `json:"fallback"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// StepView is a pipeline step with a budget's override applied.
type StepView struct {
	Step
	Override  *OverrideView `json:"override"`
	Effective Choice        `json:"effective"`
	// Warning says why a stored override isn't being used, or that the step
	// needs a model the owner hasn't set up.
	Warning string `json:"warning,omitempty"`
}

// PipelineView is one pipeline with a budget's overrides applied.
type PipelineView struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Version     int        `json:"version"`
	Steps       []StepView `json:"steps"`
}

// CatalogView is every pipeline for one budget.
type CatalogView struct {
	// Defaults are the owner's default model of each kind they have set up,
	// shared by every pipeline.
	Defaults  map[model.LLMModelKind]*ModelRef `json:"defaults"`
	Pipelines []PipelineView                   `json:"pipelines"`
}

const (
	warnAdapterGone = "The way of running this step you chose is no longer offered, so it uses the default."
	warnModelGone   = "The model you chose for this step was disabled or removed, so it uses your default."
)

func warnNoModel(kind model.LLMModelKind) string {
	return fmt.Sprintf("This step needs a %s model, and none is set up. Add one in Settings → AI.", kind)
}

// DefaultOf returns the owner's default model of kind. models come from
// UserLLMModelRepository.ListUsableForBudget (usable keys only).
func DefaultOf(models []model.UserLLMModel, kind model.LLMModelKind) (model.UserLLMModel, bool) {
	var first *model.UserLLMModel
	for i := range models {
		if models[i].Kind != kind {
			continue
		}
		if models[i].IsDefault {
			return models[i], true
		}
		if first == nil {
			first = &models[i]
		}
	}
	// The default or its key may be disabled; then the first usable one by
	// priority stands in.
	if first != nil {
		return *first, true
	}
	return model.UserLLMModel{}, false
}

// Resolve applies override (nil = none) to step. models are the owner's usable
// models. It never fails: anything in the override that no longer fits falls
// back to the default, with a warning.
func Resolve(step Step, override *model.PipelineStepOverride, models []model.UserLLMModel) StepView {
	view := StepView{Step: step, Effective: Choice{Adapter: step.DefaultAdapter}}
	var warnings []string

	if override != nil {
		view.Override = &OverrideView{
			Adapter:   override.Adapter,
			ModelID:   override.ModelID,
			Fallback:  override.Fallback,
			UpdatedAt: override.UpdatedAt,
		}
		if override.Adapter != nil {
			if a, ok := step.Adapter(*override.Adapter); ok && !a.Unavailable {
				view.Effective.Adapter = a.ID
			} else {
				warnings = append(warnings, warnAdapterGone)
			}
		}
	}

	adapter, _ := step.Adapter(view.Effective.Adapter)
	if adapter.UsesModel {
		view.Effective.Fallback = step.Fallback
		if override != nil && override.Fallback != nil {
			view.Effective.Fallback = *override.Fallback
		}
		if override != nil && override.ModelID != nil {
			if m, ok := findModel(models, *override.ModelID); ok && m.Kind == adapter.ModelKind {
				view.Effective.Model = refOf(m)
			} else {
				warnings = append(warnings, warnModelGone)
			}
		}
		if view.Effective.Model == nil {
			if m, ok := DefaultOf(models, adapter.ModelKind); ok {
				view.Effective.Model = refOf(m)
			} else {
				warnings = append(warnings, warnNoModel(adapter.ModelKind))
			}
		}
	}

	view.Warning = strings.Join(warnings, " ")
	return view
}

// Attempt is one try at running a step: an adapter and, for a model adapter,
// the model to call with it.
type Attempt struct {
	Adapter string
	Model   *model.UserLLMModel // nil for adapters that don't use a model
	// MaxTokens caps the model's output, from the pipeline file. 0 = no cap set.
	MaxTokens int
}

// Attempts lists what to run for a step, in order; the runner moves to the next
// one when an attempt fails. models are the owner's usable models, as
// UserLLMModelRepository.ListUsableForBudget orders them (default first, then
// by priority).
//
// The first attempt is Resolve's choice, so a model picked for the step goes
// ahead of the default. With fallback on, the owner's other models of the same
// kind follow. It returns nil when the step needs a model and the owner has
// none of that kind.
func Attempts(step Step, override *model.PipelineStepOverride, models []model.UserLLMModel) []Attempt {
	view := Resolve(step, override, models)
	adapter, ok := step.Adapter(view.Effective.Adapter)
	if !ok {
		return nil
	}
	if !adapter.UsesModel {
		return []Attempt{{Adapter: adapter.ID}}
	}
	if view.Effective.Model == nil {
		return nil
	}
	first, ok := findModel(models, view.Effective.Model.ID)
	if !ok {
		return nil
	}

	attempts := []Attempt{{Adapter: adapter.ID, Model: &first, MaxTokens: adapter.MaxTokens}}
	if !view.Effective.Fallback {
		return attempts
	}
	for i := range models {
		if models[i].Kind == adapter.ModelKind && models[i].ID != first.ID {
			attempts = append(attempts, Attempt{Adapter: adapter.ID, Model: &models[i], MaxTokens: adapter.MaxTokens})
		}
	}
	return attempts
}

// BuildView resolves every step of p for a budget. overrides may include
// other pipelines' rows; they're ignored.
func BuildView(p *Pipeline, overrides []model.PipelineStepOverride, models []model.UserLLMModel) PipelineView {
	byStep := map[string]*model.PipelineStepOverride{}
	for i := range overrides {
		if overrides[i].Pipeline == p.ID {
			byStep[overrides[i].Step] = &overrides[i]
		}
	}
	view := PipelineView{
		ID:          p.ID,
		Label:       p.Label,
		Description: p.Description,
		Version:     p.Version,
		Steps:       make([]StepView, 0, len(p.Steps)),
	}
	for _, s := range p.Steps {
		view.Steps = append(view.Steps, Resolve(s, byStep[string(s.ID)], models))
	}
	return view
}

// BuildCatalogView resolves every pipeline for a budget.
func BuildCatalogView(c *Catalog, overrides []model.PipelineStepOverride, models []model.UserLLMModel) CatalogView {
	view := CatalogView{
		Defaults:  map[model.LLMModelKind]*ModelRef{},
		Pipelines: make([]PipelineView, 0, len(c.Pipelines)),
	}
	for _, kind := range []model.LLMModelKind{model.LLMModelChat, model.LLMModelDecision} {
		if m, ok := DefaultOf(models, kind); ok {
			view.Defaults[kind] = refOf(m)
		}
	}
	for _, p := range c.Pipelines {
		view.Pipelines = append(view.Pipelines, BuildView(p, overrides, models))
	}
	return view
}

func findModel(models []model.UserLLMModel, id uuid.UUID) (model.UserLLMModel, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return model.UserLLMModel{}, false
}

// ValidateOverride checks a budget's requested change to step before it's
// stored. Unlike Resolve, it rejects instead of falling back: the user is
// there to fix it. Every rejection has code errs.CodePipelineInvalidOverride
// and a Message written for the user.
func ValidateOverride(step Step, o model.PipelineStepOverride, models []model.UserLLMModel) error {
	invalid := func(format string, args ...any) error {
		return errs.New(errs.CodePipelineInvalidOverride, format, args...)
	}
	if !step.Editable() {
		return invalid("%s has nothing to change", step.Label)
	}

	adapterID := step.DefaultAdapter
	if o.Adapter != nil {
		a, ok := step.Adapter(*o.Adapter)
		if !ok {
			return invalid("%s can't run with %q", step.Label, *o.Adapter)
		}
		if a.Unavailable {
			return invalid("%s isn't available yet", a.Label)
		}
		adapterID = a.ID
	}
	adapter, _ := step.Adapter(adapterID)

	if o.Fallback != nil && !adapter.UsesModel {
		return invalid("%s doesn't use a model, so it has nothing to fall back to", adapter.Label)
	}

	if o.ModelID != nil {
		if !adapter.UsesModel {
			return invalid("%s doesn't use a model", adapter.Label)
		}
		m, ok := findModel(models, *o.ModelID)
		if !ok {
			return invalid("that model isn't set up in Settings → AI")
		}
		if m.Kind != adapter.ModelKind {
			return invalid(
				"%s needs a %s model, and %s is a %s model",
				adapter.Label,
				adapter.ModelKind,
				m.Slug,
				m.Kind,
			)
		}
	}
	return nil
}
