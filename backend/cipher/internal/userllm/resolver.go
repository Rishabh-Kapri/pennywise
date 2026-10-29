// Package userllm resolves LLM clients from the AI providers the budget owner
// configured (user_llm_providers). Nothing falls back to the server's own
// keys: a user with no enabled provider gets AI_PROVIDER_NOT_CONFIGURED.
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
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// ErrNotConfigured is what callers see when the owner has no usable provider.
var ErrNotConfigured = errs.New(errs.CodeAIProviderNotConfigured,
	"no AI provider configured: add one in Settings → AI")

type Resolver struct {
	repo       repository.UserLLMProviderRepository
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
	repo repository.UserLLMProviderRepository,
	box *utils.SecretBox,
	tel otelSDK.TelemetryProvider,
	allowPrivateURLs bool,
) *Resolver {
	return &Resolver{
		repo:       repo,
		box:        box,
		tel:        tel,
		userURLCli: newUserURLClient(allowPrivateURLs),
		cache:      map[uuid.UUID]cachedRegistry{},
	}
}

// Resolve returns the owner's client for provider. A provider the owner hasn't
// configured (including "") falls back to their first provider and its model,
// so hardcoded choices such as the memory observer's still work.
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

func (r *Resolver) registryFor(ctx context.Context) (llm.LLMResolver, error) {
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "resolving AI provider without a budget", err)
	}
	rows, err := r.repo.ListEnabledForBudget(ctx, budgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "loading AI providers", err)
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
		client, err := r.buildClient(row)
		if err != nil {
			// One broken row (e.g. undecryptable after a key rotation) must not
			// take the others down.
			logger.Logger(ctx).Warn("skipping AI provider", "provider", row.Provider, "error", err)
			continue
		}
		entries[string(row.Provider)] = llm.RegistryEntry{Client: client, DefaultModel: row.Model}
		order = append(order, string(row.Provider))
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

// fingerprintOf changes whenever a row is added, removed, edited or reordered.
func fingerprintOf(rows []model.UserLLMProvider) string {
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "%s@%d;", row.ID, row.UpdatedAt.UnixNano())
	}
	return b.String()
}

func (r *Resolver) buildClient(row model.UserLLMProvider) (*llm.ObservedLLM, error) {
	apiKey := ""
	if len(row.APIKeyEncrypted) > 0 {
		if r.box == nil {
			return nil, utils.ErrSecretBoxKeyMissing
		}
		key, err := r.box.Open(row.APIKeyEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypting key: %w", err)
		}
		apiKey = key
	}

	var (
		client llm.LLM
		err    error
	)
	switch row.Provider {
	case model.LLMProviderOpenAI:
		client, err = providers.NewOpenAIClientWithKey(apiKey)
	case model.LLMProviderAnthropic:
		client, err = providers.NewAnthropicClientWithKey("chat", apiKey)
	case model.LLMProviderOpenRouter:
		client, err = providers.NewOpenRouterClientWithKey(apiKey)
	case model.LLMProviderOllama:
		client, err = providers.NewUserOllamaClient(row.BaseURL, apiKey, r.userURLCli)
	default:
		err = fmt.Errorf("unsupported provider %q", row.Provider)
	}
	if err != nil {
		return nil, err
	}
	return llm.NewObservedLLM(client, r.tel), nil
}

// Test makes a tiny call with the owner's saved config for provider, enabled
// or not, and records the outcome on the row.
func (r *Resolver) Test(ctx context.Context, provider model.LLMProviderName) (*model.TestUserLLMProviderResponse, error) {
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	row, err := r.repo.GetForBudget(ctx, budgetID, provider)
	if err != nil {
		return nil, err
	}

	res := &model.TestUserLLMProviderResponse{Model: row.Model}
	started := time.Now()
	client, err := r.buildClient(*row)
	if err == nil {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		var chatRes *model.ChatResponse
		chatRes, err = client.Chat(callCtx, model.ChatRequest{
			Provider:  string(row.Provider),
			Model:     row.Model,
			MaxTokens: 64,
			Messages: []model.AgentMessage{{
				Role:    model.RoleUser,
				Content: []model.ContentBlock{{Type: "text", Text: "Reply with the single word: ok"}},
			}},
		}, nil)
		cancel()
		if err == nil && chatRes != nil && chatRes.Model != "" {
			res.Model = chatRes.Model
		}
	}
	res.LatencyMs = time.Since(started).Milliseconds()
	res.OK = err == nil
	if err != nil {
		res.Error = err.Error()
	}

	if recErr := r.repo.RecordVerification(ctx, budgetID, provider, res.Error); recErr != nil {
		logger.Logger(ctx).Warn("recording AI provider test", "error", recErr)
	}
	return res, nil
}
