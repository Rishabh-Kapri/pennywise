// Package userllm resolves model clients from the budget owner's own AI setup:
// their keys (user_llm_providers) and the models they run with them
// (user_llm_models). Nothing falls back to the server's own keys: a user with
// no usable chat model gets AI_PROVIDER_NOT_CONFIGURED.
package userllm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/decisions"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/resolver"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// ErrNotConfigured is what callers see when the owner has no usable provider.
var ErrNotConfigured = errs.New(errs.CodeAIProviderNotConfigured,
	"no AI provider configured: add one in Settings → AI")

type Resolver struct {
	models  repository.UserLLMModelRepository
	keys    repository.UserLLMProviderRepository
	clients resolver.Resolver
	tel     otelSDK.TelemetryProvider

	mu    sync.Mutex
	cache map[uuid.UUID]cachedRegistry // budget id → clients
}

type cachedRegistry struct {
	fingerprint string
	registry    llm.LLMResolver
}

// New builds a Resolver. clients builds each model's client from its key.
func New(
	models repository.UserLLMModelRepository,
	keys repository.UserLLMProviderRepository,
	clients resolver.Resolver,
	tel otelSDK.TelemetryProvider,
) *Resolver {
	return &Resolver{
		models:  models,
		keys:    keys,
		clients: clients,
		tel:     tel,
		cache:   map[uuid.UUID]cachedRegistry{},
	}
}

// Resolve returns the owner's chat client for provider. A provider the owner
// hasn't configured (including "") falls back to their default chat model, so
// hardcoded choices such as the memory observer's still work.
func (r *Resolver) Resolve(ctx context.Context, provider, model string) (*llm.ObservedLLM, string, error) {
	registry, err := r.registryFor(ctx)
	if err != nil {
		return nil, "", err
	}
	if provider != "" {
		if client, resolved, err := registry.Resolve(ctx, provider, model); err == nil {
			return client, resolved, nil
		}
	}
	return registry.Resolve(ctx, "", "")
}

func (r *Resolver) Targets(ctx context.Context) ([]llm.Target, error) {
	registry, err := r.registryFor(ctx)
	if err != nil {
		return nil, err
	}
	return registry.Targets(ctx)
}

// registryFor builds the owner's chat clients: one per key, using that key's
// first chat model, with the default chat model's key first.
func (r *Resolver) registryFor(ctx context.Context) (llm.LLMResolver, error) {
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "resolving AI provider without a budget", err)
	}
	rows, err := r.models.ListUsableForBudget(ctx, budgetID, model.LLMModelChat)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "loading AI models", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotConfigured
	}

	fingerprint := fingerprintOf(rows)
	r.mu.Lock()
	cached, ok := r.cache[budgetID]
	r.mu.Unlock()
	if ok && cached.fingerprint == fingerprint {
		return cached.registry, nil
	}

	entries := map[string]llm.RegistryEntry{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		name := string(row.Provider)
		if _, seen := entries[name]; seen {
			continue // rows come default first, so the key's first model wins
		}
		client, err := r.clients.ResolveLLM(row)
		if err != nil {
			// One broken key (e.g. undecryptable after a key rotation) must not
			// take the others down.
			logger.Logger(ctx).Warn("skipping AI provider", "provider", row.Provider, "error", err)
			continue
		}
		entries[name] = llm.RegistryEntry{Client: client, DefaultModel: row.Slug}
		order = append(order, name)
	}
	if len(order) == 0 {
		return nil, ErrNotConfigured
	}
	registry := llm.NewLLMRegistry(entries, order, r.tel)

	r.mu.Lock()
	r.cache[budgetID] = cachedRegistry{fingerprint: fingerprint, registry: registry}
	r.mu.Unlock()
	return registry, nil
}

// fingerprintOf changes whenever a model or its key is added, removed, edited
// or reordered.
func fingerprintOf(rows []model.UserLLMModel) string {
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "%s@%d/%d;", row.ID, row.UpdatedAt.UnixNano(), row.KeyUpdatedAt.UnixNano())
	}
	return b.String()
}

// Test makes a tiny call to a model with the owner's saved key for
// req.Provider: a one-word chat for a chat model, a one-question decision for a
// decision model. With req.ModelID set, the outcome is recorded on that model.
// A failing model is ok=false, not an error; errors are for a missing key or
// budget.
func (r *Resolver) Test(
	ctx context.Context,
	req model.TestUserLLMModelRequest,
) (*model.TestUserLLMModelResponse, error) {
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	key, err := r.keys.GetForBudget(ctx, budgetID, req.Provider)
	if err != nil {
		return nil, err
	}

	res := &model.TestUserLLMModelResponse{Model: req.Slug}
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	m := keyModel(key, req)
	switch req.Kind {
	case model.LLMModelChat:
		res.Model, err = r.testChat(callCtx, m)
	case model.LLMModelDecision:
		res.Model, err = r.testDecision(callCtx, m)
	case model.LLMModelEmbedding:
		res.Model, err = r.testEmbedding(callCtx, m)
	default:
		err = fmt.Errorf("unknown model kind %q", req.Kind)
	}
	cancel()
	res.LatencyMs = time.Since(started).Milliseconds()
	res.OK = err == nil
	if err != nil {
		res.Error = err.Error()
	}
	if res.Model == "" {
		res.Model = req.Slug
	}

	if req.ModelID != nil {
		if recErr := r.models.RecordVerification(ctx, budgetID, *req.ModelID, res.Error); recErr != nil {
			logger.Logger(ctx).Warn("recording AI model test", "error", recErr)
		}
	}
	return res, nil
}

// keyModel is the model req names, run on the saved key, for building a client
// before the model is saved.
func keyModel(key *model.UserLLMProvider, req model.TestUserLLMModelRequest) model.UserLLMModel {
	return model.UserLLMModel{
		ProviderID:      key.ID,
		Provider:        key.Provider,
		Kind:            req.Kind,
		Slug:            req.Slug,
		Endpoint:        req.Endpoint,
		APIKeyEncrypted: key.APIKeyEncrypted,
		BaseURL:         key.BaseURL,
		KeyUpdatedAt:    key.UpdatedAt,
	}
}

func (r *Resolver) testChat(ctx context.Context, m model.UserLLMModel) (string, error) {
	client, err := r.clients.ResolveLLM(m)
	if err != nil {
		return "", err
	}
	chatRes, err := client.Chat(ctx, model.ChatRequest{
		Provider:  string(m.Provider),
		Model:     m.Slug,
		MaxTokens: 64,
		Messages: []model.AgentMessage{{
			Role:    model.RoleUser,
			Content: []model.ContentBlock{{Type: "text", Text: "Reply with the single word: ok"}},
		}},
	}, nil)
	if err != nil {
		return "", err
	}
	if chatRes != nil {
		return chatRes.Model, nil
	}
	return "", nil
}

func (r *Resolver) testDecision(ctx context.Context, m model.UserLLMModel) (string, error) {
	if m.Endpoint == "" {
		return "", fmt.Errorf("a decision model needs an endpoint")
	}
	client, err := r.clients.ResolveDecisions(m)
	if err != nil {
		return "", err
	}
	return decisions.Probe(ctx, client, m.Slug)
}

// testEmbedding checks the model answers, with vectors of the size stored ones
// have.
func (r *Resolver) testEmbedding(ctx context.Context, m model.UserLLMModel) (string, error) {
	embedder, err := r.clients.ResolveEmbedder(m)
	if err != nil {
		return "", err
	}
	if _, err := embedder.Embed(ctx, m.Slug, "Rs. 100 debited at a cafe"); err != nil {
		return "", err
	}
	return m.Slug, nil
}
