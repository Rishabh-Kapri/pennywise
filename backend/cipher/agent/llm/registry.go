package llm

import (
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
)

// llmRegistry is a stateless registry of pre-built LLM clients, one per provider.
// Clients are constructed once at startup and reused safely across concurrent calls.
type llmRegistry struct {
	clients         map[string]*ObservedLLM // provider → raw client (e.g. *anthropicClient)
	defaults        map[string]string      // provider → default model name
	defaultProvider string
	telemetry       otelSDK.TelemetryProvider
}

// RegistryEntry holds the raw LLM client and its default model for one provider.
type RegistryEntry struct {
	Client       *ObservedLLM
	DefaultModel string
}

// NewLLMRegistry builds a registry from a map of provider → RegistryEntry.
// defaultProvider is used when Resolve is called with provider="".
// tel is wired into every ObservedLLM returned by Resolve.
func NewLLMRegistry(
	entries map[string]RegistryEntry,
	defaultProvider string,
	tel otelSDK.TelemetryProvider,
) (LLMResolver, error) {
	if _, ok := entries[defaultProvider]; !ok {
		return nil, errs.New(errs.CodeInternalError,
			"default provider %q is not present in registry entries", defaultProvider)
	}

	clients := make(map[string]*ObservedLLM, len(entries))
	defaults := make(map[string]string, len(entries))
	for provider, entry := range entries {
		clients[provider] = entry.Client
		defaults[provider] = entry.DefaultModel
	}

	return &llmRegistry{
		clients:         clients,
		defaults:        defaults,
		defaultProvider: defaultProvider,
		telemetry:       tel,
	}, nil
}

// Resolve returns an ObservedLLM and the resolved model name for the given
// provider and model. Empty provider falls back to the registry default.
// Empty model falls back to the provider's default model.
func (r *llmRegistry) Resolve(provider, model string) (*ObservedLLM, string, error) {
	if provider == "" {
		provider = r.defaultProvider
	}

	client, ok := r.clients[provider]
	if !ok || client == nil {
		return nil, "", errs.New(errs.CodeLLMNotConfigured,
			"llm client not configured for provider %q", provider)
	}

	if model == "" {
		model = r.defaults[provider]
	}

	return NewObservedLLM(client, r.telemetry), model, nil
}
