// Package userllm resolves model clients from the budget owner's own AI setup:
// their keys (user_llm_providers) and the models they run with them
// (user_llm_models). Nothing falls back to the server's own keys: a user with
// no usable chat model gets AI_PROVIDER_NOT_CONFIGURED.
package userllm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm/providers"
	decisions "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/decisions"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// ErrNotConfigured is what callers see when the owner has no usable provider.
var ErrNotConfigured = errs.New(errs.CodeAIProviderNotConfigured,
	"no AI provider configured: add one in Settings → AI")

type Resolver struct {
	models     repository.UserLLMModelRepository
	keys       repository.UserLLMProviderRepository
	box        *utils.SecretBox // nil when LLM_CREDENTIALS_KEY is unset
	tel        otelSDK.TelemetryProvider
	userURLCli *http.Client

	mu    sync.Mutex
	cache map[uuid.UUID]cachedRegistry // budget id → clients
}

type cachedRegistry struct {
	fingerprint string
	registry    llm.LLMResolver
}

// New builds a Resolver. allowPrivateURLs lets user Ollama URLs reach private
// addresses; only turn it on for single-tenant self-hosted deployments.
func New(
	models repository.UserLLMModelRepository,
	keys repository.UserLLMProviderRepository,
	box *utils.SecretBox,
	tel otelSDK.TelemetryProvider,
	allowPrivateURLs bool,
) *Resolver {
	return &Resolver{
		models:     models,
		keys:       keys,
		box:        box,
		tel:        tel,
		userURLCli: transport.NewUserURLClient(allowPrivateURLs),
		cache:      map[uuid.UUID]cachedRegistry{},
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

func (r *Resolver) HTTPClient() *http.Client {
	return r.userURLCli
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
		client, err := r.buildClient(row.Provider, row.APIKeyEncrypted, row.BaseURL)
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

func (r *Resolver) openKey(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	if r.box == nil {
		return "", utils.ErrSecretBoxKeyMissing
	}
	key, err := r.box.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("decrypting key: %w", err)
	}
	return key, nil
}

func (r *Resolver) buildClient(
	provider model.LLMProviderName,
	sealedKey []byte,
	baseURL string,
) (*llm.ObservedLLM, error) {
	apiKey, err := r.openKey(sealedKey)
	if err != nil {
		return nil, err
	}
	providerType := "cloud"

	var client llm.LLM
	switch provider {
	case model.LLMProviderOpenAI:
		client, err = providers.NewOpenAIClientWithKey(apiKey)
	case model.LLMProviderAnthropic:
		client, err = providers.NewAnthropicClientWithKey("chat", apiKey)
	case model.LLMProviderOpenRouter:
		client, err = providers.NewOpenRouterClientWithKey(apiKey)
	case model.LLMProviderOllama:
		client, err = providers.NewUserOllamaClient(baseURL, apiKey, r.userURLCli)
		providerType = "local"
	default:
		err = fmt.Errorf("unsupported provider %q", provider)
	}
	if err != nil {
		return nil, err
	}
	return llm.NewObservedLLM(client, r.tel, providerType), nil
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
	switch req.Kind {
	case model.LLMModelChat:
		res.Model, err = r.testChat(callCtx, key, req.Slug)
	case model.LLMModelDecision:
		res.Model, err = r.testDecision(callCtx, key, req.Slug, req.Endpoint)
	case model.LLMModelEmbedding:
		res.Model, err = r.testEmbedding(callCtx, key, req.Slug)
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

func (r *Resolver) testChat(ctx context.Context, key *model.UserLLMProvider, slug string) (string, error) {
	client, err := r.buildClient(key.Provider, key.APIKeyEncrypted, key.BaseURL)
	if err != nil {
		return "", err
	}
	chatRes, err := client.Chat(ctx, model.ChatRequest{
		Provider:  string(key.Provider),
		Model:     slug,
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

func (r *Resolver) testDecision(
	ctx context.Context,
	key *model.UserLLMProvider,
	slug, endpoint string,
) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("a decision model needs an endpoint")
	}
	apiKey, err := r.openKey(key.APIKeyEncrypted)
	if err != nil {
		return "", err
	}
	// The endpoint is user-given: dial it through the client that refuses
	// private addresses unless the server allows them.
	client := decisions.NewUserClient(endpoint, apiKey, slug, r.userURLCli)
	return client.Probe(ctx)
}

// testEmbedding checks the model answers, with vectors of the size stored ones
// have.
func (r *Resolver) testEmbedding(ctx context.Context, key *model.UserLLMProvider, slug string) (string, error) {
	apiKey, err := r.openKey(key.APIKeyEncrypted)
	if err != nil {
		return "", err
	}
	embedder, err := client.NewUserEmbedder(key.Provider, key.BaseURL, apiKey, r.userURLCli)
	if err != nil {
		return "", err
	}
	if _, err := embedder.Embed(ctx, slug, "Rs. 100 debited at a cafe"); err != nil {
		return "", err
	}
	return slug, nil
}

// Model returns the owner's model of kind with id, or their default of that
// kind when id is nil or no longer usable, plus its decrypted key.
func (r *Resolver) Model(
	ctx context.Context,
	kind model.LLMModelKind,
	id *uuid.UUID,
) (model.UserLLMModel, string, error) {
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return model.UserLLMModel{}, "", err
	}
	all, err := r.models.ListUsableForBudget(ctx, budgetID, kind)
	if err != nil {
		return model.UserLLMModel{}, "", err
	}
	var picked *model.UserLLMModel
	for i := range all {
		if id != nil && all[i].ID == *id {
			picked = &all[i]
			break
		}
		if picked == nil {
			picked = &all[i] // rows come default first
		}
	}
	if picked == nil {
		return model.UserLLMModel{}, "", ErrNotConfigured
	}
	apiKey, err := r.openKey(picked.APIKeyEncrypted)
	if err != nil {
		return model.UserLLMModel{}, "", err
	}
	return *picked, apiKey, nil
}
