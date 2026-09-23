package providers

import (
	"net/url"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// NewLumoClient connects to lumo-tamer through its Responses API.
func NewLumoClient() (llm.LLM, error) {
	cfg := config.Load()
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.LumoBaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errs.New(errs.CodeInternalError, "LUMO_BASE_URL must be an absolute HTTP(S) URL without query or fragment")
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return newResponsesClient("lumo", baseURL, cfg.LumoAPIKey), nil
}
