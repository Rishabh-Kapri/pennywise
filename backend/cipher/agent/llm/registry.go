package llm

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// Target is one provider/model pair in a fallback chain.
type Target struct {
	Provider string
	Model    string
}

// llmRegistry is a stateless registry of pre-built LLM clients, one per provider.
// Clients are constructed once and reused safely across concurrent calls.
type llmRegistry struct {
	clients  map[string]*ObservedLLM // provider → raw client (e.g. *anthropicClient)
	defaults map[string]string       // provider → default model name
	order    []string                // fallback order; order[0] is the default provider
}

// RegistryEntry holds the raw LLM client and its default model for one provider.
type RegistryEntry struct {
	Client       *ObservedLLM
	DefaultModel string
}

// LLMResolver resolves a provider + model name to a ready-to-use ObservedLLM.
// It returns the resolved model name (applying defaults when the caller passes "")
// so the caller can set req.Model correctly. ctx carries the budget whose
// owner's providers are used (see internal/userllm).
type LLMResolver interface {
	Resolve(ctx context.Context, provider, model string) (*ObservedLLM, string, error)
	// Targets is the ordered fallback chain (provider + default model).
	Targets(ctx context.Context) ([]Target, error)
}

// NewLLMRegistry builds a registry from a map of provider → RegistryEntry.
// order lists the providers in fallback order; its first entry is used when
// Resolve is called with provider="". Providers in order without an entry are
// ignored.
func NewLLMRegistry(entries map[string]RegistryEntry, order []string) LLMResolver {
	clients := make(map[string]*ObservedLLM, len(entries))
	defaults := make(map[string]string, len(entries))
	for provider, entry := range entries {
		clients[provider] = entry.Client
		defaults[provider] = entry.DefaultModel
	}
	known := make([]string, 0, len(order))
	for _, provider := range order {
		if _, ok := clients[provider]; ok {
			known = append(known, provider)
		}
	}

	return &llmRegistry{
		clients:  clients,
		defaults: defaults,
		order:    known,
	}
}

// Resolve returns an ObservedLLM and the resolved model name for the given
// provider and model. Empty provider falls back to the registry default.
// Empty model falls back to the provider's default model.
func (r *llmRegistry) Resolve(_ context.Context, provider, model string) (*ObservedLLM, string, error) {
	if provider == "" && len(r.order) > 0 {
		provider = r.order[0]
	}

	client, ok := r.clients[provider]
	if !ok || client == nil {
		return nil, "", errs.New(errs.CodeLLMNotConfigured,
			"llm client not configured for provider %q", provider)
	}

	if model == "" {
		model = r.defaults[provider]
	}

	return client, model, nil
}

func (r *llmRegistry) Targets(context.Context) ([]Target, error) {
	targets := make([]Target, 0, len(r.order))
	for _, provider := range r.order {
		targets = append(targets, Target{Provider: provider, Model: r.defaults[provider]})
	}
	return targets, nil
}
