package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment        string
	ServiceName        string
	DatabaseURL        string
	JWTSecret          string
	Domain             string
	GoogleClientID     string
	GoogleClientSecret string
	CallbackURL        string
	GmailServiceURL    string
	GmailServiceName   string
	CipherServiceURL   string
	CipherServiceName  string
	InternalAuthToken  string
	TemporalServerHost string
	TemporalServerPort string
}

func Load() Config {
	_ = godotenv.Load(".env")
	env := os.Getenv("RAILWAY_ENVIRONMENT_NAME")
	if env == "" {
		env = "local"
	}
	return Config{
		Environment:        env,
		ServiceName:        "pennywise-api",
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		Domain:             os.Getenv("DOMAIN"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		CallbackURL:        os.Getenv("CALLBACK_URL"),

		GmailServiceURL:  os.Getenv("GMAIL_SERVICE_URL"),
		GmailServiceName: "gmail-watch",

		CipherServiceURL:  os.Getenv("CIPHER_SERVICE_URL"),
		CipherServiceName: "cipher",
		InternalAuthToken: os.Getenv("INTERNAL_AUTH_TOKEN"),

		TemporalServerHost: os.Getenv("TEMPORAL_SERVER_HOST"),
		TemporalServerPort: os.Getenv("TEMPORAL_SERVER_PORT"),
	}
}
