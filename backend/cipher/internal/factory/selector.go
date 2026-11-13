package factory

import (
	"context"
	"time"

	aliasAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/alias"
	decisionsAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/decisions"
	llmAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/regex"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/resolver"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
)

type MessagePipeline interface {
	Check(ctx context.Context, budgetID uuid.UUID) (ports.CheckTransaction, error)
	Extract(ctx context.Context, budgetID uuid.UUID) (ports.Extract, error)
	Verify(ctx context.Context, budgetID uuid.UUID) (ports.Verifier, error)
	Lookup(ctx context.Context, budgetID uuid.UUID) (ports.MemoryLookup, error)
	Predict(ctx context.Context, budgetID uuid.UUID) (ports.Predictor, error)
}

// MessagePipelineDeps is what the message pipeline's adapters run with.
type MessagePipelineDeps struct {
	PipelineSteps repository.PipelineStepRepository
	LLMModels     repository.UserLLMModelRepository
	Categories    repository.CategoryRepository
	Aliases       repository.CounterpartyAliasRepository
	Clients       resolver.Resolver
	// LLMCallTimeout bounds each chat model call (CIPHER_LLM_CALL_TIMEOUT).
	LLMCallTimeout time.Duration
}

type messagePipeline struct {
	pipeline *pipeline.Pipeline
	deps     MessagePipelineDeps
}

// builders are the adapters cipher can run for one step, by the adapter ID
// that pipelines/message.json uses.
type builders[P any] map[string]func(p *messagePipeline, attempt pipeline.Attempt) (P, error)

var (
	checkAdapters = builders[ports.CheckTransaction]{
		"regex": func(*messagePipeline, pipeline.Attempt) (ports.CheckTransaction, error) {
			return regex.NewCheckTransaction(), nil
		},
		"decisions": func(p *messagePipeline, a pipeline.Attempt) (ports.CheckTransaction, error) { return p.decisions(a) },
		"llm":       func(p *messagePipeline, a pipeline.Attempt) (ports.CheckTransaction, error) { return p.llm(a) },
	}
	extractAdapters = builders[ports.Extract]{
		"llm": func(p *messagePipeline, a pipeline.Attempt) (ports.Extract, error) { return p.llm(a) },
	}
	verifyAdapters = builders[ports.Verifier]{
		"decisions": func(p *messagePipeline, a pipeline.Attempt) (ports.Verifier, error) { return p.decisions(a) },
	}
	lookupAdapters = builders[ports.MemoryLookup]{
		"aliases": func(p *messagePipeline, _ pipeline.Attempt) (ports.MemoryLookup, error) {
			return aliasAdapter.NewLookup(p.deps.Aliases), nil
		},
	}
	predictAdapters = builders[ports.Predictor]{
		"llm": func(p *messagePipeline, a pipeline.Attempt) (ports.Predictor, error) { return p.llm(a) },
	}
)

// knownAdapters is every adapter ID cipher runs, by step. Create isn't
// cipher's: go-pennywise-api runs it.
func knownAdapters() map[pipeline.StepID]map[string]bool {
	return map[pipeline.StepID]map[string]bool{
		pipeline.StepCheck:   ids(checkAdapters),
		pipeline.StepExtract: ids(extractAdapters),
		pipeline.StepVerify:  ids(verifyAdapters),
		pipeline.StepLookup:  ids(lookupAdapters),
		pipeline.StepPredict: ids(predictAdapters),
	}
}

func ids[P any](b builders[P]) map[string]bool {
	out := make(map[string]bool, len(b))
	for id := range b {
		out[id] = true
	}
	return out
}

// checkKnown fails when the pipeline file has a step or adapter cipher can't
// run, so a new one in the file fails at startup instead of on a message.
func checkKnown(p *pipeline.Pipeline) error {
	known := knownAdapters()
	for _, step := range p.Steps {
		if step.ID == pipeline.StepCreate {
			continue
		}
		adapters, ok := known[step.ID]
		if !ok {
			return errs.New(errs.CodePipelineInvalid, "cipher can't run %s step %q", p.ID, step.ID)
		}
		for _, adapter := range step.Adapters {
			if !adapters[adapter.ID] {
				return errs.New(errs.CodePipelineInvalid, "cipher has no %q adapter for %s step %q", adapter.ID, p.ID, step.ID)
			}
		}
	}
	return nil
}

func NewMessagePipeline(deps MessagePipelineDeps) (MessagePipeline, error) {
	catalog, err := pipeline.Load()
	if err != nil {
		return nil, err
	}
	pipelineConfig, ok := catalog.Pipeline(pipeline.MessagePipeline)
	if !ok {
		return nil, errs.New(errs.CodePipelineInvalid, "no %s pipeline", pipeline.MessagePipeline)
	}
	if err := checkKnown(pipelineConfig); err != nil {
		return nil, err
	}
	logger.Logger(context.Background()).Info(
		"message pipeline loaded",
		"pipeline", pipelineConfig.ID,
		"version", pipelineConfig.Version,
		"steps", len(pipelineConfig.Steps),
	)
	return &messagePipeline{pipeline: pipelineConfig, deps: deps}, nil
}
func (p *messagePipeline) attempts(
	ctx context.Context,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
) ([]pipeline.Attempt, error) {
	log := logger.Logger(ctx).With("step", stepID, "budgetId", budgetID)

	step, ok := p.pipeline.Step(stepID)
	if !ok {
		err := errs.New(errs.CodePipelineInvalid, "message pipeline has no %s step", stepID)
		log.Error("step missing from the pipeline file", "error", err)
		return nil, err
	}
	log.Debug(
		"step config",
		"defaultAdapter", step.DefaultAdapter,
		"fallback", step.Fallback,
		"adapters", adapterIDs(step),
	)

	budgetOverrides, err := p.deps.PipelineSteps.ListByBudgetAndPipeline(ctx, budgetID, pipeline.MessagePipeline)
	if err != nil {
		log.Error("loading the budget's overrides failed", "error", err)
		return nil, err
	}
	var override *sharedModel.PipelineStepOverride
	for i := range budgetOverrides {
		if budgetOverrides[i].Step == string(stepID) {
			override = &budgetOverrides[i]
			break
		}
	}
	log.Debug("budget override", "override", describeOverride(override))

	models, err := p.deps.LLMModels.ListUsableForBudget(ctx, budgetID)
	if err != nil {
		log.Error("loading the owner's models failed", "error", err)
		return nil, err
	}
	log.Debug("usable models", "count", len(models), "models", describeModels(models))

	view := pipeline.Resolve(step, override, models)
	if view.Warning != "" {
		log.Warn("step can't use what was chosen for it", "warning", view.Warning)
	}

	attempts := pipeline.Attempts(step, override, models)
	if len(attempts) == 0 {
		err := errs.New(
			errs.CodeAIProviderNotConfigured,
			"%s needs a model; add one in Settings -> AI",
			step.Label,
		)
		log.Error("step has nothing to run", "error", err)
		return nil, err
	}
	log.Debug("attempts", "fallback", view.Effective.Fallback, "attempts", describeAttempts(attempts))
	return attempts, nil
}

func (p *messagePipeline) llm(attempt pipeline.Attempt) (llmAdapter.LLM, error) {
	client, err := p.deps.Clients.ResolveLLM(*attempt.Model)
	if err != nil {
		return nil, err
	}
	return llmAdapter.NewLLMAdapter(llmAdapter.Config{
		Client:      client,
		Provider:    string(attempt.Model.Provider),
		Model:       attempt.Model.Slug,
		MaxTokens:   attempt.MaxTokens,
		Categories:  p.deps.Categories,
		CallTimeout: p.deps.LLMCallTimeout,
	}), nil
}

func (p *messagePipeline) decisions(attempt pipeline.Attempt) (decisionsAdapter.DecisionAdapter, error) {
	client, err := p.deps.Clients.ResolveDecisions(*attempt.Model)
	if err != nil {
		return nil, err
	}
	return decisionsAdapter.NewDecisionsAdapter(client, attempt.Model.Slug), nil
}

// buildStep turns a step's attempts into the adapter to run: the only one, or
// a chain when fallback gave it several. A model that can't be built (a broken
// key, say) is skipped so the others can still run.
func buildStep[P any](
	ctx context.Context,
	p *messagePipeline,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
	adapters builders[P],
	chain func([]chainLink[P]) P,
) (P, error) {
	log := logger.Logger(ctx).With("step", stepID, "budgetId", budgetID)
	var zero P

	attempts, err := p.attempts(ctx, budgetID, stepID)
	if err != nil {
		return zero, err
	}

	var links []chainLink[P]
	var buildErr error
	for _, attempt := range attempts {
		name := describeAttempt(attempt)
		log.Debug("building adapter", "attempt", name, "maxTokens", attempt.MaxTokens)
		link, err := build(p, adapters, stepID, attempt)
		if err != nil {
			log.Warn("skipping model", "attempt", name, "error", err)
			buildErr = err
			continue
		}
		links = append(links, chainLink[P]{name: name, model: attemptModel(attempt), port: link})
	}

	switch len(links) {
	case 0:
		log.Error("no adapter could be built", "attempts", describeAttempts(attempts), "error", buildErr)
		return zero, buildErr
	case 1:
		log.Info("step ready", "runs", links[0].name)
		return links[0].port, nil
	default:
		log.Info("step ready with fallback", "runs", linkNames(links))
		return chain(links), nil
	}
}

// build makes the adapter an attempt names.
func build[P any](p *messagePipeline, adapters builders[P], stepID pipeline.StepID, attempt pipeline.Attempt) (P, error) {
	b, ok := adapters[attempt.Adapter]
	if !ok {
		var zero P
		return zero, errs.New(errs.CodeAdapterNotConfigured, "unknown adapter for %s step %q", stepID, attempt.Adapter)
	}
	return b(p, attempt)
}

// Check returns the adapter for this budget's check step.
func (p *messagePipeline) Check(ctx context.Context, budgetID uuid.UUID) (ports.CheckTransaction, error) {
	return buildStep(ctx, p, budgetID, pipeline.StepCheck, checkAdapters,
		func(links []chainLink[ports.CheckTransaction]) ports.CheckTransaction { return checkChain{links} })
}

// Extract returns the adapter for this budget's extract step.
func (p *messagePipeline) Extract(ctx context.Context, budgetID uuid.UUID) (ports.Extract, error) {
	return buildStep(ctx, p, budgetID, pipeline.StepExtract, extractAdapters,
		func(links []chainLink[ports.Extract]) ports.Extract { return extractChain{links} })
}

// Verify returns the adapter for this budget's verify step.
func (p *messagePipeline) Verify(ctx context.Context, budgetID uuid.UUID) (ports.Verifier, error) {
	return buildStep(ctx, p, budgetID, pipeline.StepVerify, verifyAdapters,
		func(links []chainLink[ports.Verifier]) ports.Verifier { return verifyChain{links} })
}

// Lookup returns the adapter for this budget's lookup step. Its adapters use
// no model, so there is one attempt and never a chain.
func (p *messagePipeline) Lookup(ctx context.Context, budgetID uuid.UUID) (ports.MemoryLookup, error) {
	attempts, err := p.attempts(ctx, budgetID, pipeline.StepLookup)
	if err != nil {
		return nil, err
	}
	return build(p, lookupAdapters, pipeline.StepLookup, attempts[0])
}

// Predict returns the adapter for this budget's predict step.
func (p *messagePipeline) Predict(ctx context.Context, budgetID uuid.UUID) (ports.Predictor, error) {
	return buildStep(ctx, p, budgetID, pipeline.StepPredict, predictAdapters,
		func(links []chainLink[ports.Predictor]) ports.Predictor { return predictChain{links} })
}
