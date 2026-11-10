// Package config reads ingest's settings from the environment.
package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	// Environment is RAILWAY_ENVIRONMENT_NAME; "local" when unset.
	Environment string

	DatabaseDriver string
	DatabaseURL    string

	// JWTSecret verifies user access tokens. Empty: only API keys work.
	JWTSecret string
	// InternalAuthToken is the token ingest sends to other services.
	InternalAuthToken string
	// ServiceTokens is SERVICE_TOKENS: "name:token,..." for each service that may call in.
	ServiceTokens string

	// Where accepted messages are queued (see shared/factory.ConnectJob).
	// RedisURL carries job updates to the API for live screens. Unset = off.
	RedisURL     string
	JobDriver    string
	JobBrokerURL string
	JobStoreURL  string
	JobQueues    []string

	CredentialsKey string // CredentialsKey

	// Mail providers (see internal/factory.NewMailProviders).
	MailProviders         string // MAIL_PROVIDERS, e.g. "gmail"
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleAndroidClientID string
	GoogleProjectID       string // PROJECT_ID
	GooglePubsubTopic     string // PUBSUB_TOPIC

	// Gmail push receiver (internal/pubsub). Off unless SUB_NAME is set.
	GooglePubsubSubscription string // SUB_NAME
	GoogleCredentialsJSON    string // GOOGLE_APPLICATION_CREDENTIALS_JSON
}

// Load reads .env if present, then the environment.
func Load() Config {
	_ = godotenv.Load()
	return Config{
		Port:              getEnv("PORT", "5180"),
		Environment:       getEnv("RAILWAY_ENVIRONMENT_NAME", "local"),
		DatabaseDriver:    os.Getenv("DATABASE_DRIVER"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		InternalAuthToken: os.Getenv("INTERNAL_AUTH_TOKEN"),
		ServiceTokens:     os.Getenv("SERVICE_TOKENS"),
		RedisURL:          os.Getenv("REDIS_URL"),
		JobDriver:         os.Getenv("JOB_DRIVER"),
		JobBrokerURL:      os.Getenv("JOB_BROKER_URL"),
		JobStoreURL:       os.Getenv("JOB_STORE_URL"),

		CredentialsKey: os.Getenv("CREDENTIALS_KEY"),

		MailProviders:         os.Getenv("MAIL_PROVIDERS"),
		GoogleClientID:        os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleAndroidClientID: os.Getenv("GOOGLE_ANDROID_CLIENT_ID"),
		GoogleProjectID:       os.Getenv("PROJECT_ID"),
		GooglePubsubTopic:     os.Getenv("PUBSUB_TOPIC"),

		GooglePubsubSubscription: os.Getenv("SUB_NAME"),
		GoogleCredentialsJSON:    os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON"),
		JobQueues:                splitList(os.Getenv("JOB_QUEUES")),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
