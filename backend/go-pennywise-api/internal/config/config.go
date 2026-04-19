package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
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
}

func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
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
	}
}
