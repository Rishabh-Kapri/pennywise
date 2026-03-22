package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	JWTSecret      string
	GoogleClientID string
	Domain         string
}

func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		Domain:         os.Getenv("DOMAIN"),
	}
}
