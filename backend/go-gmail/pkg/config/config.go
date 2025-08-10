package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	GoogleClientId         string
	GoogleClientSecret     string
	CallbackUrl            string
	GoogleCloudSecretsFile string
	ProjectID              string
	PubsubTopic            string
	SubscriptionName       string
	DatabaseURL            string
	MLPApi                 string
	PennywiseApi           string
	NtfyTopic              string
}

func LoadConfig() *Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error while loading .env: %v", err.Error())
	}
	return &Config{
		GoogleClientId:         os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:     os.Getenv("GOOGLE_CLIENT_SECRET"),
		CallbackUrl:            os.Getenv("CALLBACK_URL"),
		GoogleCloudSecretsFile: os.Getenv("GCLOUD_SECRETS_FILE"),
		ProjectID:              os.Getenv("PROJECT_ID"),
		PubsubTopic:            os.Getenv("PUBSUB_TOPIC"),
		SubscriptionName:       os.Getenv("SUB_NAME"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		MLPApi:                 os.Getenv("MLP_API"),
		PennywiseApi:           os.Getenv("PENNYWISE_API"),
		NtfyTopic:              os.Getenv("NTFY_TOPIC"),
	}
}
