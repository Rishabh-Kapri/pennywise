// Package resolver builds the client for one of a user's saved models from its
// key: chat, embedding or decisions. It is the only place that decrypts a key
// or picks a provider's client; userllm and the pipeline factory ask it.
package resolver

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/decisions"
	decisionProviders "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/decisions/providers"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	llmProviders "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm/providers"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type resolver struct {
	box               *utils.SecretBox
	httpClient        *http.Client // SSRF-safe
	tel               otelSDK.TelemetryProvider
	mu                sync.Mutex
	llmRegistry       map[string]*llm.ObservedLLM
	decisionsRegistry map[string]decisions.Decisions
	embedderRegistry  map[string]client.Embedder
}

type Resolver interface {
	ResolveLLM(m sharedModel.UserLLMModel) (*llm.ObservedLLM, error)
	ResolveEmbedder(m sharedModel.UserLLMModel) (client.Embedder, error)
	ResolveDecisions(m sharedModel.UserLLMModel) (decisions.Decisions, error)
}

func NewResolver(box *utils.SecretBox, httpClient *http.Client, tel otelSDK.TelemetryProvider) Resolver {
	return &resolver{
		box:               box,
		httpClient:        httpClient,
		tel:               tel,
		mu:                sync.Mutex{},
		llmRegistry:       make(map[string]*llm.ObservedLLM),
		decisionsRegistry: make(map[string]decisions.Decisions),
		embedderRegistry:  make(map[string]client.Embedder),
	}
}

func (r *resolver) cacheKey(m sharedModel.UserLLMModel) string {
	// A new key, URL or endpoint is a new client.
	key := fmt.Sprintf("%s|%s|%s|%s|%d", m.Provider, m.ProviderID, m.Endpoint, m.BaseURL, m.KeyUpdatedAt.UnixNano())
	return base64.StdEncoding.EncodeToString([]byte(key))
}

func (r *resolver) ResolveLLM(m sharedModel.UserLLMModel) (*llm.ObservedLLM, error) {
	key := r.cacheKey(m)

	r.mu.Lock()
	defer r.mu.Unlock()

	if client, ok := r.llmRegistry[key]; ok {
		return client, nil
	}

	apikey, err := r.openKey(m)
	if err != nil {
		return nil, err
	}

	var client llm.LLM
	switch m.Provider {
	case sharedModel.LLMProviderOpenAI:
		client, err = llmProviders.NewOpenAIClientWithKey(apikey)
	case sharedModel.LLMProviderAnthropic:
		client, err = llmProviders.NewAnthropicClientWithKey(apikey)
	case sharedModel.LLMProviderOpenRouter:
		client, err = llmProviders.NewOpenRouterClientWithKey(apikey)
	case sharedModel.LLMProviderOllama:
		client, err = llmProviders.NewUserOllamaClient(m.BaseURL, apikey, r.httpClient)
	default:
		err = errs.New(errs.CodeAIProviderNotConfigured, "unsupported provider %q", m.Provider)
	}
	if err != nil {
		return nil, err
	}

	observed := llm.NewObservedLLM(client, r.tel)
	r.llmRegistry[key] = observed
	return observed, nil
}

// openKey decrypts the model's key. A self-hosted Ollama may have none.
func (r *resolver) openKey(m sharedModel.UserLLMModel) (string, error) {
	if len(m.APIKeyEncrypted) == 0 {
		return "", nil
	}
	if r.box == nil {
		return "", utils.ErrSecretBoxKeyMissing
	}
	key, err := r.box.Open(m.APIKeyEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypting key: %w", err)
	}
	return key, nil
}

func (r *resolver) ResolveEmbedder(m sharedModel.UserLLMModel) (client.Embedder, error) {
	key := r.cacheKey(m)

	r.mu.Lock()
	defer r.mu.Unlock()

	if embedder, ok := r.embedderRegistry[key]; ok {
		return embedder, nil
	}

	apikey, err := r.openKey(m)
	if err != nil {
		return nil, err
	}
	embedder, err := client.NewUserEmbedder(m.Provider, m.BaseURL, apikey, r.httpClient)
	if err != nil {
		return nil, err
	}
	r.embedderRegistry[key] = embedder
	return embedder, nil
}

func (r *resolver) ResolveDecisions(m sharedModel.UserLLMModel) (decisions.Decisions, error) {
	key := r.cacheKey(m)

	r.mu.Lock()
	defer r.mu.Unlock()

	if client, ok := r.decisionsRegistry[key]; ok {
		return client, nil
	}

	apikey, err := r.openKey(m)
	if err != nil {
		return nil, err
	}

	// Any key can be used with a decisions endpoint, whichever provider it's for.
	client, err := decisionProviders.NewOpenRouterClient(m.Endpoint, apikey, r.httpClient)
	if err != nil {
		return nil, err
	}
	r.decisionsRegistry[key] = client
	return client, nil
}
