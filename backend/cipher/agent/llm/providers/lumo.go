package providers

import (
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

// lumoResponsesPath is where lumo-tamer serves the OpenAI Responses API.
const lumoResponsesPath = "/v1/responses"

// LumoDefaultModel is the tier lumo-tamer advertises when the caller does not
// pick one; "lumo" lets Proton route between Lite and Max itself.
const LumoDefaultModel = "lumo"

// normalizeLumoBaseURL accepts the value people naturally copy out of an
// OpenAI-compatible client — the base URL *including* /v1 — as well as the bare
// host, and returns the host without a trailing slash or /v1, since the request
// path already carries it. Without this, a pasted ".../v1" would produce
// ".../v1/v1/responses" and 404 at runtime with no obvious cause.
func normalizeLumoBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	trimmed = strings.TrimSuffix(trimmed, "/v1")
	return strings.TrimRight(trimmed, "/")
}

// NewLumoClient talks to a lumo-tamer instance, which fronts Proton Lumo with an
// OpenAI-compatible API. It is the same Responses wire shape OpenRouter serves,
// so it reuses responsesClient — only the host, path and auth differ.
//
// Lumo has a much smaller context window than the hosted providers (lumo-tamer
// warns around ~22.5K tokens) and its tool support is emulated, so it suits the
// email pipeline's short extraction/classification prompts better than a
// long-running tool-calling agent run.
func NewLumoClient() (llm.LLM, error) {
	cfg := config.Load()

	baseURL := normalizeLumoBaseURL(cfg.LumoBaseURL)
	if baseURL == "" {
		return nil, errs.New(errs.CodeInternalError, "no lumo base url found")
	}

	headers := map[string][]string{
		"content-type": {"application/json"},
	}
	// lumo-tamer rejects every non-health request without the bearer key, but the
	// key lives in its config rather than being mandatory, so allow an unset one
	// instead of failing startup for a deployment that left it open.
	if cfg.LumoAPIKey != "" {
		headers["Authorization"] = []string{fmt.Sprintf("Bearer %s", cfg.LumoAPIKey)}
	}

	httpTransport := httpclient.NewHttpTransport(baseURL)

	return &responsesClient{
		provider: "lumo",
		path:     lumoResponsesPath,
		httpClient: transport.NewClient(
			"lumo",
			httpTransport,
			transport.WithDefaultHeaders(headers),
			transport.WithPropagateInternalHeaders(false),
		),
	}, nil
}
