package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	ServiceName string
	// DatabaseDriver picks the adapter behind the repositories (default postgres).
	DatabaseDriver        string
	DatabaseURL           string
	JWTSecret             string
	Domain                string
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleAndroidClientID string
	CallbackURL           string
	RedisURL              string
	GmailServiceURL       string
	GmailServiceName      string
	CipherServiceURL      string
	CipherServiceName     string
	IngestServiceURL      string // INGEST_SERVICE_URL; unset turns email sync setup off
	IngestServiceName     string
	InternalAuthToken     string
	// Job queue. JOB_DRIVER unset turns the worker off.
	JobDriver    string
	JobBrokerURL string
	JobStoreURL  string
	JobQueues    []string // JOB_QUEUES, e.g. "pennywise"
	// ServiceTokens is SERVICE_TOKENS: "name:token,..." for each service that may call in.
	ServiceTokens       string
	DemoMode            bool
	NominatimURL        string
	GeocodeCountryCodes string
	DocumentStorage     string
	UploadsDir          string
	ExpoPushURL         string
	// LLMCredentialsKey seals user AI provider keys (base64, 32 bytes). Cipher
	// needs the same value to open them.
	LLMCredentialsKey string
	// SmsDecisionsLog is the JSON Lines file the SMS ingest stub appends
	// decisions verdicts and costs to.
	SmsDecisionsLog string
}

func Load() Config {
	_ = godotenv.Load(".env")
	env := os.Getenv("RAILWAY_ENVIRONMENT_NAME")
	if env == "" {
		env = "local"
	}
	return Config{
		Environment:           env,
		ServiceName:           "pennywise-api",
		DatabaseDriver:        os.Getenv("DATABASE_DRIVER"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		Domain:                os.Getenv("DOMAIN"),
		GoogleClientID:        os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleAndroidClientID: os.Getenv("GOOGLE_ANDROID_CLIENT_ID"),
		CallbackURL:           os.Getenv("CALLBACK_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),

		GmailServiceURL:  os.Getenv("GMAIL_SERVICE_URL"),
		GmailServiceName: "gmail-watch",

		CipherServiceURL:  os.Getenv("CIPHER_SERVICE_URL"),
		CipherServiceName: "cipher",

		IngestServiceURL:  os.Getenv("INGEST_SERVICE_URL"),
		IngestServiceName: "ingest",
		InternalAuthToken: os.Getenv("INTERNAL_AUTH_TOKEN"),

		JobDriver:    os.Getenv("JOB_DRIVER"),
		JobBrokerURL: os.Getenv("JOB_BROKER_URL"),
		JobStoreURL:  os.Getenv("JOB_STORE_URL"),
		JobQueues:    splitList(os.Getenv("JOB_QUEUES")),

		ServiceTokens: os.Getenv("SERVICE_TOKENS"),

		DemoMode: os.Getenv("DEMO_MODE") == "true",

		// reverse geocoding (empty = public OSM Nominatim)
		NominatimURL:        os.Getenv("NOMINATIM_URL"),
		GeocodeCountryCodes: os.Getenv("GEOCODE_COUNTRY_CODES"),
		// "local" keeps the old on-disk behaviour; anything else uses Postgres.
		DocumentStorage: os.Getenv("DOCUMENT_STORAGE"),

		UploadsDir: uploadsDir(),

		SmsDecisionsLog: envOr("SMS_DECISIONS_LOG", "./data/sms-decisions.jsonl"),

		// Expo push endpoint (empty = the public https://exp.host one)
		ExpoPushURL: os.Getenv("EXPO_PUSH_URL"),

		LLMCredentialsKey: os.Getenv("LLM_CREDENTIALS_KEY"),
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func uploadsDir() string {
	dir := os.Getenv("UPLOADS_DIR")
	if dir == "" {
		dir = "./data/uploads"
	}
	return dir
}
