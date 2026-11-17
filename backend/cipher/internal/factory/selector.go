package factory

import (
	"context"
	"time"

	aliasAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/alias"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/bankexport"
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

// --------------------- Shared step machinery ----------------------
//
// Every pipeline factory works the same way: load its file from the catalog,
// check cipher can run every adapter in it, then per job pick the budget's
// attempts for a step and build an adapter (or a fallback chain) from them.
// Only the builder tables and the ports differ.

// builders are the adapters cipher can run for one step, by the adapter ID the
// pipeline file uses. O is the pipeline factory that builds them (it holds the
// deps), P the port the step returns.
type builders[O, P any] map[string]func(owner O, attempt pipeline.Attempt) (P, error)

func ids[O, P any](b builders[O, P]) map[string]bool {
	out := make(map[string]bool, len(b))
	for id := range b {
		out[id] = true
	}
	return out
}

// stepAdapters is every adapter ID cipher runs for one pipeline, by step.
type stepAdapters map[pipeline.StepID]map[string]bool

// knownAdapters is every adapter ID cipher runs, by pipeline and step. Two
// pipelines can share a step ID (both have "extract"), so it's keyed by
// pipeline first.
func knownAdapters() map[string]stepAdapters {
	return map[string]stepAdapters{
		pipeline.MessagePipeline:   messageKnownAdapters(),
		pipeline.StatementPipeline: statementKnownAdapters(),
	}
}

// notCipherSteps are steps another service runs; cipher skips them.
var notCipherSteps = map[string]map[pipeline.StepID]bool{
	pipeline.MessagePipeline:   {pipeline.StepCreate: true},        // go-pennywise-api
	pipeline.StatementPipeline: {pipeline.StepStatementSave: true}, // go-pennywise-api
}

// checkKnown fails when the pipeline file has a step or adapter cipher can't
// run, so a new one in the file fails at startup instead of on a job.
// Unavailable adapters can't be picked, so they needn't exist yet.
func checkKnown(p *pipeline.Pipeline) error {
	known, ok := knownAdapters()[p.ID]
	if !ok {
		return errs.New(errs.CodePipelineInvalid, "cipher can't run the %s pipeline", p.ID)
	}
	for _, step := range p.Steps {
		if notCipherSteps[p.ID][step.ID] {
			continue
		}
		adapters, ok := known[step.ID]
		if !ok {
			return errs.New(errs.CodePipelineInvalid, "cipher can't run %s step %q", p.ID, step.ID)
		}
		for _, adapter := range step.Adapters {
			if adapter.Unavailable || adapters[adapter.ID] {
				continue
			}
			return errs.New(
				errs.CodePipelineInvalid,
				"cipher has no %q adapter for %s step %q",
				adapter.ID,
				p.ID,
				step.ID,
			)
		}
	}
	return nil
}

// loadPipeline reads one pipeline from the embedded catalog and checks cipher
// can run it.
func loadPipeline(id string) (*pipeline.Pipeline, error) {
	catalog, err := pipeline.Load()
	if err != nil {
		return nil, err
	}
	p, ok := catalog.Pipeline(id)
	if !ok {
		return nil, errs.New(errs.CodePipelineInvalid, "no %s pipeline", id)
	}
	if err := checkKnown(p); err != nil {
		return nil, err
	}
	logger.Logger(context.Background()).Info(
		"pipeline loaded",
		"pipeline", p.ID,
		"version", p.Version,
		"steps", len(p.Steps),
	)
	return p, nil
}

// stepSource picks what a budget runs for each step of one pipeline: the
// file's default, the budget's override, and the owner's usable models.
type stepSource struct {
	pipeline      *pipeline.Pipeline
	pipelineSteps repository.PipelineStepRepository
	llmModels     repository.UserLLMModelRepository
}

// enabled reports whether the budget runs stepID: only an optional step can
// be turned off.
func (s stepSource) enabled(ctx context.Context, budgetID uuid.UUID, stepID pipeline.StepID) (bool, error) {
	step, ok := s.pipeline.Step(stepID)
	if !ok {
		return false, errs.New(errs.CodePipelineInvalid, "%s pipeline has no %s step", s.pipeline.ID, stepID)
	}
	if !step.Optional {
		return true, nil
	}
	overrides, err := s.pipelineSteps.ListByBudgetAndPipeline(ctx, budgetID, s.pipeline.ID)
	if err != nil {
		return false, err
	}
	for i := range overrides {
		if overrides[i].Step == string(stepID) {
			return pipeline.Enabled(step, &overrides[i]), nil
		}
	}
	return true, nil
}

func (s stepSource) attempts(
	ctx context.Context,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
) ([]pipeline.Attempt, error) {
	log := logger.Logger(ctx).With("pipeline", s.pipeline.ID, "step", stepID, "budgetId", budgetID)

	step, ok := s.pipeline.Step(stepID)
	if !ok {
		err := errs.New(errs.CodePipelineInvalid, "%s pipeline has no %s step", s.pipeline.ID, stepID)
		log.Error("step missing from the pipeline file", "error", err)
		return nil, err
	}
	log.Debug(
		"step config",
		"defaultAdapter", step.DefaultAdapter,
		"fallback", step.Fallback,
		"adapters", adapterIDs(step),
	)

	budgetOverrides, err := s.pipelineSteps.ListByBudgetAndPipeline(ctx, budgetID, s.pipeline.ID)
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

	models, err := s.llmModels.ListUsableForBudget(ctx, budgetID)
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

// buildStep turns a step's attempts into the adapter to run: the only one, or
// a chain when fallback gave it several. A model that can't be built (a broken
// key, say) is skipped so the others can still run.
func buildStep[O, P any](
	ctx context.Context,
	src stepSource,
	owner O,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
	adapters builders[O, P],
	chain func([]chainLink[P]) P,
) (P, error) {
	log := logger.Logger(ctx).With("pipeline", src.pipeline.ID, "step", stepID, "budgetId", budgetID)
	var zero P

	attempts, err := src.attempts(ctx, budgetID, stepID)
	if err != nil {
		return zero, err
	}

	var links []chainLink[P]
	var buildErr error
	for _, attempt := range attempts {
		name := describeAttempt(attempt)
		log.Debug("building adapter", "attempt", name, "maxTokens", attempt.MaxTokens)
		link, err := build(owner, adapters, stepID, attempt)
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

// buildSingle builds a step whose adapters use no model: one attempt, never a chain.
func buildSingle[O, P any](
	ctx context.Context,
	src stepSource,
	owner O,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
	adapters builders[O, P],
) (P, error) {
	attempts, err := src.attempts(ctx, budgetID, stepID)
	if err != nil {
		var zero P
		return zero, err
	}
	return build(owner, adapters, stepID, attempts[0])
}

// build makes the adapter an attempt names.
func build[O, P any](
	owner O,
	adapters builders[O, P],
	stepID pipeline.StepID,
	attempt pipeline.Attempt,
) (P, error) {
	b, ok := adapters[attempt.Adapter]
	if !ok {
		var zero P
		return zero, errs.New(errs.CodeAdapterNotConfigured, "unknown adapter for %s step %q", stepID, attempt.Adapter)
	}
	return b(owner, attempt)
}

// --------------------- Message Pipeline ----------------------

type MessagePipeline interface {
	Check(ctx context.Context, budgetID uuid.UUID) (ports.CheckTransaction, error)
	Extract(ctx context.Context, budgetID uuid.UUID) (ports.ExtractTxn, error)
	Verify(ctx context.Context, budgetID uuid.UUID) (ports.Verifier, error)
	Lookup(ctx context.Context, budgetID uuid.UUID) (ports.MemoryLookup, error)
	Predict(ctx context.Context, budgetID uuid.UUID) (ports.PredictTxn, error)
	// Enabled reports whether the budget runs stepID; only optional steps
	// (verify, lookup, predict) can be off.
	Enabled(ctx context.Context, budgetID uuid.UUID, stepID pipeline.StepID) (bool, error)
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
	steps stepSource
	deps  MessagePipelineDeps
}

type messageBuilders[P any] = builders[*messagePipeline, P]

var (
	checkAdapters = messageBuilders[ports.CheckTransaction]{
		"regex": func(*messagePipeline, pipeline.Attempt) (ports.CheckTransaction, error) {
			return regex.NewCheckTransaction(), nil
		},
		"decisions": func(p *messagePipeline, a pipeline.Attempt) (ports.CheckTransaction, error) { return p.decisions(a) },
		"llm":       func(p *messagePipeline, a pipeline.Attempt) (ports.CheckTransaction, error) { return p.llm(a) },
	}
	extractAdapters = messageBuilders[ports.ExtractTxn]{
		"llm": func(p *messagePipeline, a pipeline.Attempt) (ports.ExtractTxn, error) { return p.llm(a) },
	}
	verifyAdapters = messageBuilders[ports.Verifier]{
		"decisions": func(p *messagePipeline, a pipeline.Attempt) (ports.Verifier, error) { return p.decisions(a) },
	}
	lookupAdapters = messageBuilders[ports.MemoryLookup]{
		"aliases": func(p *messagePipeline, _ pipeline.Attempt) (ports.MemoryLookup, error) {
			return aliasAdapter.NewLookup(p.deps.Aliases), nil
		},
	}
	predictAdapters = messageBuilders[ports.PredictTxn]{
		"llm": func(p *messagePipeline, a pipeline.Attempt) (ports.PredictTxn, error) { return p.llm(a) },
	}
)

func messageKnownAdapters() stepAdapters {
	return stepAdapters{
		pipeline.StepCheck:   ids(checkAdapters),
		pipeline.StepExtract: ids(extractAdapters),
		pipeline.StepVerify:  ids(verifyAdapters),
		pipeline.StepLookup:  ids(lookupAdapters),
		pipeline.StepPredict: ids(predictAdapters),
	}
}

func NewMessagePipeline(deps MessagePipelineDeps) (MessagePipeline, error) {
	p, err := loadPipeline(pipeline.MessagePipeline)
	if err != nil {
		return nil, err
	}
	return &messagePipeline{
		steps: stepSource{pipeline: p, pipelineSteps: deps.PipelineSteps, llmModels: deps.LLMModels},
		deps:  deps,
	}, nil
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

// Enabled implements [MessagePipeline].
func (p *messagePipeline) Enabled(ctx context.Context, budgetID uuid.UUID, stepID pipeline.StepID) (bool, error) {
	return p.steps.enabled(ctx, budgetID, stepID)
}

// Check returns the adapter for this budget's check step.
func (p *messagePipeline) Check(ctx context.Context, budgetID uuid.UUID) (ports.CheckTransaction, error) {
	return buildStep(ctx, p.steps, p, budgetID, pipeline.StepCheck, checkAdapters,
		func(links []chainLink[ports.CheckTransaction]) ports.CheckTransaction { return checkChain{links} })
}

// Extract returns the adapter for this budget's extract step.
func (p *messagePipeline) Extract(ctx context.Context, budgetID uuid.UUID) (ports.ExtractTxn, error) {
	return buildStep(ctx, p.steps, p, budgetID, pipeline.StepExtract, extractAdapters,
		func(links []chainLink[ports.ExtractTxn]) ports.ExtractTxn { return extractChain{links} })
}

// Verify returns the adapter for this budget's verify step.
func (p *messagePipeline) Verify(ctx context.Context, budgetID uuid.UUID) (ports.Verifier, error) {
	return buildStep(ctx, p.steps, p, budgetID, pipeline.StepVerify, verifyAdapters,
		func(links []chainLink[ports.Verifier]) ports.Verifier { return verifyChain{links} })
}

// Lookup returns the adapter for this budget's lookup step. Its adapters use
// no model, so there is one attempt and never a chain.
func (p *messagePipeline) Lookup(ctx context.Context, budgetID uuid.UUID) (ports.MemoryLookup, error) {
	return buildSingle(ctx, p.steps, p, budgetID, pipeline.StepLookup, lookupAdapters)
}

// Predict returns the adapter for this budget's predict step.
func (p *messagePipeline) Predict(ctx context.Context, budgetID uuid.UUID) (ports.PredictTxn, error) {
	return buildStep(ctx, p.steps, p, budgetID, pipeline.StepPredict, predictAdapters,
		func(links []chainLink[ports.PredictTxn]) ports.PredictTxn { return predictChain{links} })
}

// --------------------- Statement Pipeline ----------------------

// StatementImportPipeline builds the readers for statement.json's steps. Both
// steps choose from the same readers: fallback reruns extract another way.
// Save isn't cipher's: go-pennywise-api runs it.
type StatementImportPipeline interface {
	// Extract returns the reader for this budget's extract step.
	Extract(ctx context.Context, budgetID uuid.UUID) (ports.StatementImport, error)
	// Fallback returns the reader for this budget's fallback step, run when
	// extract's rows don't check out.
	Fallback(ctx context.Context, budgetID uuid.UUID) (ports.StatementImport, error)
	// Enabled reports whether the budget runs stepID; only email and fallback
	// can be off.
	Enabled(ctx context.Context, budgetID uuid.UUID, stepID pipeline.StepID) (bool, error)
}

// StatementImportDeps is what the statement pipeline's adapters run with.
type StatementImportDeps struct {
	PipelineSteps repository.PipelineStepRepository
	LLMModels     repository.UserLLMModelRepository
	Clients       resolver.Resolver
	// LLMCallTimeout bounds each chat model call (CIPHER_LLM_CALL_TIMEOUT).
	LLMCallTimeout time.Duration
}

type statementImport struct {
	steps stepSource
	deps  StatementImportDeps
}

type statementBuilders[P any] = builders[*statementImport, P]

// statementReaders are the ways to read a statement, for either step. Each
// returns rows: glm_ocr reads page images, llm the PDF's text, export a CSV
// from the bank (no model; not offered for fallback).
var statementReaders = statementBuilders[ports.StatementImport]{
	"glm_ocr": func(s *statementImport, a pipeline.Attempt) (ports.StatementImport, error) { return s.localOCR(a) },
	"llm":     func(s *statementImport, a pipeline.Attempt) (ports.StatementImport, error) { return s.llm(a) },
	"export":  func(*statementImport, pipeline.Attempt) (ports.StatementImport, error) { return bankexport.New(), nil },
	// "docling": once it's built; the pipeline file marks it unavailable.
}

func statementKnownAdapters() stepAdapters {
	return stepAdapters{
		// no adapter to build: the worker only asks whether it's on
		pipeline.StepStatementEmail:    {"mailbox": true},
		pipeline.StepStatementExtract:  ids(statementReaders),
		pipeline.StepStatementFallback: ids(statementReaders),
	}
}

func NewStatementPipelineFactory(deps StatementImportDeps) (StatementImportPipeline, error) {
	p, err := loadPipeline(pipeline.StatementPipeline)
	if err != nil {
		return nil, err
	}
	return &statementImport{
		steps: stepSource{pipeline: p, pipelineSteps: deps.PipelineSteps, llmModels: deps.LLMModels},
		deps:  deps,
	}, nil
}

// localOCR reads pages with a vision model on the owner's local provider
// (glm-ocr on Ollama). localocr refuses a cloud client, so the chain skips one.
func (s *statementImport) localOCR(attempt pipeline.Attempt) (ports.StatementImport, error) {
	client, err := s.deps.Clients.ResolveLLM(*attempt.Model)
	if err != nil {
		return nil, err
	}
	return localocr.NewOCRAdapter(client, localocr.OCRConfig{
		Provider:  string(attempt.Model.Provider),
		Model:     attempt.Model.Slug,
		MaxTokens: attempt.MaxTokens,
	})
}

// llm reads the statement's text with the owner's chat model.
func (s *statementImport) llm(attempt pipeline.Attempt) (ports.StatementImport, error) {
	client, err := s.deps.Clients.ResolveLLM(*attempt.Model)
	if err != nil {
		return nil, err
	}
	return llmAdapter.NewLLMAdapter(llmAdapter.Config{
		Client:      client,
		Provider:    string(attempt.Model.Provider),
		Model:       attempt.Model.Slug,
		MaxTokens:   attempt.MaxTokens,
		CallTimeout: s.deps.LLMCallTimeout,
	}), nil
}

// Enabled implements [StatementImportPipeline].
func (s *statementImport) Enabled(ctx context.Context, budgetID uuid.UUID, stepID pipeline.StepID) (bool, error) {
	return s.steps.enabled(ctx, budgetID, stepID)
}

// Extract returns the reader for this budget's extract step.
func (s *statementImport) Extract(ctx context.Context, budgetID uuid.UUID) (ports.StatementImport, error) {
	return s.reader(ctx, budgetID, pipeline.StepStatementExtract)
}

// Fallback returns the reader for this budget's fallback step.
func (s *statementImport) Fallback(ctx context.Context, budgetID uuid.UUID) (ports.StatementImport, error) {
	return s.reader(ctx, budgetID, pipeline.StepStatementFallback)
}

func (s *statementImport) reader(
	ctx context.Context,
	budgetID uuid.UUID,
	stepID pipeline.StepID,
) (ports.StatementImport, error) {
	return buildStep(ctx, s.steps, s, budgetID, stepID, statementReaders,
		func(links []chainLink[ports.StatementImport]) ports.StatementImport {
			return statementChain{stepID, links}
		})
}
