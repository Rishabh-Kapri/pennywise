package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	GoogleClientId                   string
	GoogleClientSecret               string
	GoogleApplicationCredentialsJson string
	CallbackUrl                      string
	ProjectID                        string
	PubsubTopic                      string
	SubscriptionName                 string
	DatabaseURL                      string
	MLPServiceURL                    string
	PennywiseServiceURL              string
	CipherServiceURL                 string
	NtfyTopic                        string
	TemporalServerHost               string
	TemporalServerPort               string
	Port                             string
}

func LoadConfig() *Config {
	_ = godotenv.Load(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "5170"
	}

	return &Config{
		GoogleClientId:                   os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:               os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleApplicationCredentialsJson: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON"),
		CallbackUrl:                      os.Getenv("CALLBACK_URL"),
		ProjectID:                        os.Getenv("PROJECT_ID"),
		PubsubTopic:                      os.Getenv("PUBSUB_TOPIC"),
		SubscriptionName:                 os.Getenv("SUB_NAME"),
		DatabaseURL:                      os.Getenv("DATABASE_URL"),
		MLPServiceURL:                    os.Getenv("MLP_SERVICE_URL"),
		PennywiseServiceURL:              os.Getenv("PENNYWISE_SERVICE_URL"),
		CipherServiceURL:                 os.Getenv("CIPHER_SERVICE_URL"),
		NtfyTopic:                        os.Getenv("NTFY_TOPIC"),
		TemporalServerHost:               os.Getenv("TEMPORAL_SERVER_HOST"),
		TemporalServerPort:               os.Getenv("TEMPORAL_SERVER_PORT"),
		Port:                             port,
	}
}
