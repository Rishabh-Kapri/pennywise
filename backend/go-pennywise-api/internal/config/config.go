package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment           string
	ServiceName           string
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
	InternalAuthToken     string
	TemporalServerHost    string
	TemporalServerPort    string
	DemoMode              bool
	NominatimURL          string
	GeocodeCountryCodes   string
	DocumentStorage       string
	UploadsDir            string
	ExpoPushURL           string
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
		InternalAuthToken: os.Getenv("INTERNAL_AUTH_TOKEN"),

		TemporalServerHost: os.Getenv("TEMPORAL_SERVER_HOST"),
		TemporalServerPort: os.Getenv("TEMPORAL_SERVER_PORT"),

		DemoMode: os.Getenv("DEMO_MODE") == "true",

		// reverse geocoding (empty = public OSM Nominatim)
		NominatimURL:        os.Getenv("NOMINATIM_URL"),
		GeocodeCountryCodes: os.Getenv("GEOCODE_COUNTRY_CODES"),
		// "local" keeps the old on-disk behaviour; anything else uses Postgres.
		DocumentStorage: os.Getenv("DOCUMENT_STORAGE"),

		UploadsDir: uploadsDir(),

		// Expo push endpoint (empty = the public https://exp.host one)
		ExpoPushURL: os.Getenv("EXPO_PUSH_URL"),
	}
}

func uploadsDir() string {
	dir := os.Getenv("UPLOADS_DIR")
	if dir == "" {
		dir = "./data/uploads"
	}
	return dir
}
