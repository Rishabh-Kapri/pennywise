package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	GoogleClientId                  string
	GoogleClientSecret              string
	GoogleApplicationCredentialsJson string
	CallbackUrl                     string
	ProjectID                       string
	PubsubTopic                     string
	SubscriptionName                string
	DatabaseURL                     string
	MLPApi                          string
	PennywiseApi                    string
	NtfyTopic                       string
}

func LoadConfig() *Config {
	_ = godotenv.Load(".env")

	return &Config{
		GoogleClientId:                  os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:              os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleApplicationCredentialsJson: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON"),
		CallbackUrl:                     os.Getenv("CALLBACK_URL"),
		ProjectID:                       os.Getenv("PROJECT_ID"),
		PubsubTopic:                     os.Getenv("PUBSUB_TOPIC"),
		SubscriptionName:                os.Getenv("SUB_NAME"),
		DatabaseURL:                     os.Getenv("DATABASE_URL"),
		MLPApi:                          os.Getenv("MLP_API"),
		PennywiseApi:                    os.Getenv("PENNYWISE_API"),
		NtfyTopic:                       os.Getenv("NTFY_TOPIC"),
	}
}
