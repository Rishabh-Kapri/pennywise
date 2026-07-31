package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// DefaultLLMCallTimeout bounds a single LLM/embedding round-trip. A cold local
// ollama model can take 1-2 minutes, so this must stay comfortably above that.
const DefaultLLMCallTimeout = 3 * time.Minute

// LLMTarget is one provider+model pair in an ordered fallback chain. An empty
// Model defers to the provider's registry default.
type LLMTarget struct {
	Provider string
	Model    string
}

// defaultEmailPipelineTargets preserves the pre-config behaviour: local ollama
// only. Set EMAIL_PIPELINE_PROVIDERS to add cloud fallbacks.
var defaultEmailPipelineTargets = []LLMTarget{{Provider: "ollama", Model: "gemma4:12b"}}

// defaultEmailEmbeddingTargets likewise defaults to local ollama. Every target
// in this chain must serve the same embedding model — see EmailEmbeddingTargets.
var defaultEmailEmbeddingTargets = []LLMTarget{{Provider: "ollama", Model: "bge-m3"}}

// parseLLMTargets reads an ordered, comma-separated provider list where each
// entry is "provider" or "provider=model", e.g.
//
//	ollama=gemma4:12b,openrouter=google/gemini-2.5-flash
//
// Only the first "=" splits, so model names containing "=" , ":" or "/" survive
// intact. Returns nil for empty or entirely malformed input so callers can fall
// back to the default chain.
func parseLLMTargets(raw string) []LLMTarget {
	var targets []LLMTarget

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		provider, model, _ := strings.Cut(entry, "=")
		provider = strings.TrimSpace(provider)
		if provider == "" {
			continue
		}

		targets = append(targets, LLMTarget{
			Provider: provider,
			Model:    strings.TrimSpace(model),
		})
	}

	return targets
}

type Config struct {
	Environment          string
	DatabaseURL          string
	RedisURL             string
	OllamaURL            string
	MLPServiceURL        string
	PennywiseServiceURL  string
	OpenAIAPIKey         string
	AnthropicAPIKey      string
	OpenRouterAPIKey     string
	DefaultAgentProvider string // "anthropic", "openai", "openrouter", or "ollama"
	InternalAuthToken    string
	TemporalServerHost   string
	TemporalServerPort   string
	Port                 string
	// LLMCallTimeout bounds each individual LLM/embedding HTTP call.
	LLMCallTimeout time.Duration
	// EmailPipelineTargets is the ordered provider chain the email parse/predict
	// steps use. Each target is tried in turn until one answers, so a local
	// ollama outage falls through to a cloud provider instead of stalling the
	// pipeline.
	EmailPipelineTargets []LLMTarget
	// EmailEmbeddingTargets is the ordered chain for semantic-search embeddings.
	// Every target MUST serve the same model as the stored vectors (bge-m3, 1024
	// dims) — a different model would put query vectors in a different space and
	// silently invalidate every stored pgvector row.
	EmailEmbeddingTargets []LLMTarget
	// AgentTitleModel is the "provider/model" used to generate conversation
	// titles. Empty means "use the resolver's default provider and model", which
	// is what keeps titles working on whatever provider is actually configured.
	AgentTitleModel string
	// AgentMaxTurns and AgentMaxToolCalls bound a single chat run. Zero means use
	// the agent's built-in defaults.
	AgentMaxTurns     int
	AgentMaxToolCalls int
	// AgentReadOnlyDatabaseURL is a least-privilege connection used only for the
	// agent's read-only SQL tools, so row-level security can isolate budgets at
	// the database rather than trusting the prompt. Empty falls back to
	// DatabaseURL, which disables that isolation — see main.go.
	AgentReadOnlyDatabaseURL string
	// AgentTimezone is the IANA zone the agent resolves "today" in. Defaults to
	// the process's local zone, which the Dockerfile sets to Asia/Kolkata. Set
	// this when running somewhere that does not, or "yesterday" queries drift by
	// a day at the boundary.
	AgentTimezone string
}

// envInt reads a positive integer env var, returning 0 when unset or malformed
// so callers fall back to their own defaults.
func envInt(key string) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func Load() Config {
	_ = godotenv.Load(".env")

	env := os.Getenv("RAILWAY_ENVIRONMENT_NAME")
	if env == "" {
		env = "local"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5160"
	}

	llmCallTimeout := DefaultLLMCallTimeout
	if raw := os.Getenv("CIPHER_LLM_CALL_TIMEOUT"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			llmCallTimeout = parsed
		}
	}

	emailPipelineTargets := parseLLMTargets(os.Getenv("EMAIL_PIPELINE_PROVIDERS"))
	if len(emailPipelineTargets) == 0 {
		emailPipelineTargets = defaultEmailPipelineTargets
	}

	emailEmbeddingTargets := parseLLMTargets(os.Getenv("EMAIL_EMBEDDING_PROVIDERS"))
	if len(emailEmbeddingTargets) == 0 {
		emailEmbeddingTargets = defaultEmailEmbeddingTargets
	}

	return Config{
		Environment:              env,
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RedisURL:                 os.Getenv("REDIS_URL"),
		OllamaURL:                os.Getenv("OLLAMA_URL"),
		MLPServiceURL:            os.Getenv("MLP_SERVICE_URL"),
		PennywiseServiceURL:      os.Getenv("PENNYWISE_SERVICE_URL"),
		OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
		AnthropicAPIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		OpenRouterAPIKey:         os.Getenv("OPENROUTER_API_KEY"),
		DefaultAgentProvider:     os.Getenv("AGENT_PROVIDER"),
		InternalAuthToken:        os.Getenv("INTERNAL_AUTH_TOKEN"),
		TemporalServerHost:       os.Getenv("TEMPORAL_SERVER_HOST"),
		TemporalServerPort:       os.Getenv("TEMPORAL_SERVER_PORT"),
		Port:                     port,
		AgentTitleModel:          strings.TrimSpace(os.Getenv("AGENT_TITLE_MODEL")),
		AgentMaxTurns:            envInt("AGENT_MAX_TURNS"),
		AgentMaxToolCalls:        envInt("AGENT_MAX_TOOL_CALLS"),
		AgentReadOnlyDatabaseURL: strings.TrimSpace(os.Getenv("AGENT_DB_URL")),
		AgentTimezone:            strings.TrimSpace(os.Getenv("AGENT_TIMEZONE")),
		LLMCallTimeout:           llmCallTimeout,
		EmailPipelineTargets:     emailPipelineTargets,
		EmailEmbeddingTargets:    emailEmbeddingTargets,
	}
}
