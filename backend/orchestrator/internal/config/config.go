package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	OllamaURL   string
	MLPApiURL   string
	Port        string
}

func Load() Config {
	_ = godotenv.Load(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "5160"
	}

	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		OllamaURL:   os.Getenv("OLLAMA_URL"),
		MLPApiURL:   os.Getenv("MLP_API"),
		Port:        port,
	}
}
