package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
}

func Load() Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error while loading .env file")
	}
	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
}
