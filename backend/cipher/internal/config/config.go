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

type Config struct {
	Environment string
	ServiceName string
	// DatabaseDriver picks the adapter behind the repositories (default postgres).
	DatabaseDriver      string
	DatabaseURL         string
	RedisURL            string
	OllamaURL           string
	MLPServiceURL       string
	PennywiseServiceURL string
	LumoBaseURL         string
	LumoAPIKey          string
	OpenAIAPIKey        string
	TypesafeAPIKey      string
	InternalAuthToken   string
	// ServiceTokens is SERVICE_TOKENS: "name:token,..." for each service that may call in.
	ServiceTokens string
	Port          string
	// LLMCallTimeout bounds each individual LLM/embedding HTTP call.
	LLMCallTimeout time.Duration
	// LLMCredentialsKey opens user AI provider keys (user_llm_providers). Must
	// match the API's. Chat models (pipeline, statements, agent) come only from
	// those user providers; the server's own keys serve embeddings alone.
	LLMCredentialsKey string
	// AllowPrivateLLMURLs lets a user's Ollama URL reach private addresses.
	// Only for single-tenant self-hosting.
	AllowPrivateLLMURLs bool
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

	// Adapters

	// Job
	JobDriver    string
	JobBrokerURL string
	JobStoreURL  string
	JobQueues    []string // JOB_QUEUES, e.g. "cipher,cipher-backfill"
	// MessageReviewPark is MESSAGE_REVIEW_PARK: send low-confidence messages to
	// a person. Off, the pipeline only logs what it would have parked.
	MessageReviewPark bool

	// Statements
	StatementImportDriver    string
	StatementImportProvider  string
	StatementImportModel     string
	StatementImportMaxTokens int
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
	maxTokens := 8192
	if os.Getenv("STATEMENT_IMPORT_MAX_TOKENS") != "" {
		num, err := strconv.Atoi(os.Getenv("STATEMENT_IMPORT_MAX_TOKENS"))
		if err == nil {
			maxTokens = num
		}
	}

	return Config{
		Environment:              env,
		ServiceName:              os.Getenv("SERVICE_NAME"),
		DatabaseDriver:           os.Getenv("DATABASE_DRIVER"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		RedisURL:                 os.Getenv("REDIS_URL"),
		OllamaURL:                os.Getenv("OLLAMA_URL"),
		MLPServiceURL:            os.Getenv("MLP_SERVICE_URL"),
		PennywiseServiceURL:      os.Getenv("PENNYWISE_SERVICE_URL"),
		OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
		TypesafeAPIKey:           os.Getenv("TYPESAFE_API_KEY"),
		LumoBaseURL:              strings.TrimSpace(os.Getenv("LUMO_BASE_URL")),
		LumoAPIKey:               os.Getenv("LUMO_API_KEY"),
		InternalAuthToken:        os.Getenv("INTERNAL_AUTH_TOKEN"),
		ServiceTokens:            os.Getenv("SERVICE_TOKENS"),
		Port:                     port,
		AgentTitleModel:          strings.TrimSpace(os.Getenv("AGENT_TITLE_MODEL")),
		AgentMaxTurns:            envInt("AGENT_MAX_TURNS"),
		AgentMaxToolCalls:        envInt("AGENT_MAX_TOOL_CALLS"),
		AgentReadOnlyDatabaseURL: strings.TrimSpace(os.Getenv("AGENT_DB_URL")),
		AgentTimezone:            strings.TrimSpace(os.Getenv("AGENT_TIMEZONE")),
		LLMCallTimeout:           llmCallTimeout,
		LLMCredentialsKey:        os.Getenv("LLM_CREDENTIALS_KEY"),
		AllowPrivateLLMURLs:      os.Getenv("ALLOW_PRIVATE_LLM_URLS") == "true",

		JobDriver:         os.Getenv("JOB_DRIVER"),
		JobBrokerURL:      os.Getenv("JOB_BROKER_URL"),
		JobStoreURL:       os.Getenv("JOB_STORE_URL"),
		JobQueues:         splitList(os.Getenv("JOB_QUEUES")),
		MessageReviewPark: strings.EqualFold(strings.TrimSpace(os.Getenv("MESSAGE_REVIEW_PARK")), "true"),

		StatementImportDriver:    os.Getenv("STATEMENT_IMPORT_DRIVER"),
		StatementImportProvider:  os.Getenv("STATEMENT_IMPORT_PROVIDER"),
		StatementImportModel:     os.Getenv("STATEMENT_IMPORT_MODEL"),
		StatementImportMaxTokens: maxTokens,
	}
}

// splitList splits a comma-separated setting, trimming spaces and dropping
// empty parts, so an unset variable gives an empty list, not [""].
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
