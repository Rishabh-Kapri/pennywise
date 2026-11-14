package factory

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	decisionsAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/decisions"
	llmAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/localocr"
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
	Predict(ctx context.Context, budgetID uuid.UUID) (ports.Predictor, error)
}

// data for message pipeline adapters
type messagePipeline struct {
	pipeline         *pipeline.Pipeline
	pipelineStepRepo repository.PipelineStepRepository
	llmModelsRepo    repository.UserLLMModelRepository
	categories       repository.CategoryRepository
	resolver         resolver.Resolver
}

func NewMessagePipeline(
	pipelineStepsRepo repository.PipelineStepRepository,
	llmModelsRepo repository.UserLLMModelRepository,
	categories repository.CategoryRepository,
	resolver resolver.Resolver,
) (MessagePipeline, error) {
	catalog, err := pipeline.Load()
	if err != nil {
		return nil, err
	}
	pipelineConfig, ok := catalog.Pipeline(pipeline.MessagePipeline)
	if !ok {
		return nil, errs.New(errs.CodePipelineInvalid, "no %s pipeline", pipeline.MessagePipeline)
	}
	logger.Logger(context.Background()).Info(
		"message pipeline loaded",
		"pipeline", pipelineConfig.ID,
		"version", pipelineConfig.Version,
		"steps", len(pipelineConfig.Steps),
	)

	return &messagePipeline{
		pipeline:         pipelineConfig,
		pipelineStepRepo: pipelineStepsRepo,
		llmModelsRepo:    llmModelsRepo,
		categories:       categories,
		resolver:         resolver,
	}, nil
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

	budgetOverrides, err := p.pipelineStepRepo.ListByBudgetAndPipeline(ctx, budgetID, pipeline.MessagePipeline)
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

	models, err := p.llmModelsRepo.ListUsableForBudget(ctx, budgetID)
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
	client, err := p.resolver.ResolveLLM(*attempt.Model)
	if err != nil {
		return nil, err
	}
	return llmAdapter.NewLLMAdapter(llmAdapter.Config{
		Client:     client,
		Provider:   string(attempt.Model.Provider),
		Model:      attempt.Model.Slug,
		MaxTokens:  attempt.MaxTokens,
		Categories: p.categories,
	}), nil
}

func (p *messagePipeline) decisions(attempt pipeline.Attempt) (decisionsAdapter.DecisionAdapter, error) {
	client, err := p.resolver.ResolveDecisions(*attempt.Model)
	if err != nil {
		return nil, err
	}
	return decisionsAdapter.NewDecisionsAdapter(client, attempt.Model.Slug), nil
}

func unknownAdapter(step pipeline.StepID, adapter string) error {
	return errs.New(errs.CodeAdapterNotConfigured, "unknown adapter for %s step %q", step, adapter)
}

// buildStep turns a step's attempts into the adapter to run: the only one, or
// a chain when fallback gave it several. A model that can't be built (a broken
// key, say) is skipped so the others can still run.
func buildStep[P any](
	ctx context.Context,
	p *messagePipeline,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
	build func(pipeline.Attempt) (P, error),
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
		link, err := build(attempt)
		if err != nil {
			log.Warn("skipping model", "attempt", name, "error", err)
			buildErr = err
			continue
		}
		links = append(links, chainLink[P]{name: name, port: link})
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

// Check returns the adapter for this budget's check step.
func (p *messagePipeline) Check(ctx context.Context, budgetID uuid.UUID) (ports.CheckTransaction, error) {
	return buildStep(
		ctx, p, budgetID, pipeline.StepCheck,
		func(attempt pipeline.Attempt) (ports.CheckTransaction, error) {
			switch attempt.Adapter {
			case "regex":
				return regex.NewCheckTransaction(), nil
			case "decisions":
				return p.decisions(attempt)
			case "llm":
				return p.llm(attempt)
			}
			return nil, unknownAdapter(pipeline.StepCheck, attempt.Adapter)
		},
		func(links []chainLink[ports.CheckTransaction]) ports.CheckTransaction { return checkChain{links} },
	)
}

// Extract returns the adapter for this budget's extract step.
func (p *messagePipeline) Extract(ctx context.Context, budgetID uuid.UUID) (ports.Extract, error) {
	return buildStep(
		ctx, p, budgetID, pipeline.StepExtract,
		func(attempt pipeline.Attempt) (ports.Extract, error) {
			if attempt.Adapter == "llm" {
				return p.llm(attempt)
			}
			return nil, unknownAdapter(pipeline.StepExtract, attempt.Adapter)
		},
		func(links []chainLink[ports.Extract]) ports.Extract { return extractChain{links} },
	)
}

// Verify returns the adapter for this budget's verify step.
func (p *messagePipeline) Verify(ctx context.Context, budgetID uuid.UUID) (ports.Verifier, error) {
	return buildStep(
		ctx, p, budgetID, pipeline.StepVerify,
		func(attempt pipeline.Attempt) (ports.Verifier, error) {
			if attempt.Adapter == "decisions" {
				return p.decisions(attempt)
			}
			return nil, unknownAdapter(pipeline.StepVerify, attempt.Adapter)
		},
		func(links []chainLink[ports.Verifier]) ports.Verifier { return verifyChain{links} },
	)
}

// Predict returns the adapter for this budget's predict step.
func (p *messagePipeline) Predict(ctx context.Context, budgetID uuid.UUID) (ports.Predictor, error) {
	return buildStep(
		ctx, p, budgetID, pipeline.StepPredict,
		func(attempt pipeline.Attempt) (ports.Predictor, error) {
			if attempt.Adapter == "llm" {
				return p.llm(attempt)
			}
			return nil, unknownAdapter(pipeline.StepPredict, attempt.Adapter)
		},
		func(links []chainLink[ports.Predictor]) ports.Predictor { return predictChain{links} },
	)
}

type StatementImport interface {
	Extract(ctx context.Context, budgetID uuid.UUID) (ports.StatementImport, error)
}

type statementImport struct {
	cfg           OCRConfig
	resolver      resolver.Resolver
	llmModelsRepo repository.UserLLMModelRepository
}

type OCRConfig struct {
	Driver    string // llm | local
	Provider  string // empty for local
	Model     string // empty for local
	MaxTokens int
}

// Statement extraction requires glm-ocr from ollama to be configured to work if ollama is selected.
// Can't change the extraction config without re-deployment for now
func NewStatementFactory(
	cfg OCRConfig,
	resolver resolver.Resolver,
	llmModelsRepo repository.UserLLMModelRepository,
) StatementImport {
	return &statementImport{cfg, resolver, llmModelsRepo}
}

func (s *statementImport) Extract(ctx context.Context, budgetID uuid.UUID) (ports.StatementImport, error) {
	if s.cfg.Driver == "local" {
		return nil, nil
	}

	models, err := s.llmModelsRepo.ListUsableForBudget(ctx, budgetID, sharedModel.LLMModelChat)
	if err != nil {
		return nil, errs.Wrap(errs.CodeStatementExtractFailed, "cannot get model", err)
	}

	var client *llm.ObservedLLM
	for _, m := range models {
		if m.Provider == sharedModel.LLMProviderName(s.cfg.Provider) && m.Slug == s.cfg.Model {
			client, err = s.resolver.ResolveLLM(m)
			if err != nil {
				return nil, errs.Wrap(errs.CodeStatementExtractFailed, "cannot resolve llm client", err)
			}
			break
		}
	}
	if client == nil {
		return nil, errs.New(errs.CodeStatementExtractFailed,
			"reading statements needs the %s model %q, and this budget's owner hasn't added it (or it's disabled) in AI settings",
			s.cfg.Provider, s.cfg.Model)
	}

	switch client.Type {
	case "local":
		return localocr.NewOCRAdapter(
			client,
			localocr.OCRConfig{Provider: s.cfg.Provider, Model: s.cfg.Model, MaxTokens: s.cfg.MaxTokens},
		)
	case "cloud":
		return nil, errs.New(errs.CodeAdapterNotConfigured, "no adatper for ocr for cloud llm")
	default:
		return nil, nil
	}
}
