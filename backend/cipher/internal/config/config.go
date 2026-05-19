package config

import (
	"os"

	"github.com/joho/godotenv"
)

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

	return Config{
		Environment:          env,
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		RedisURL:             os.Getenv("REDIS_URL"),
		OllamaURL:            os.Getenv("OLLAMA_URL"),
		MLPServiceURL:        os.Getenv("MLP_SERVICE_URL"),
		PennywiseServiceURL:  os.Getenv("PENNYWISE_SERVICE_URL"),
		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		AnthropicAPIKey:      os.Getenv("ANTHROPIC_API_KEY"),
		OpenRouterAPIKey:     os.Getenv("OPENROUTER_API_KEY"),
		DefaultAgentProvider: os.Getenv("AGENT_PROVIDER"),
		InternalAuthToken:    os.Getenv("INTERNAL_AUTH_TOKEN"),
		TemporalServerHost:   os.Getenv("TEMPORAL_SERVER_HOST"),
		TemporalServerPort:   os.Getenv("TEMPORAL_SERVER_PORT"),
		Port:                 port,
	}
}
