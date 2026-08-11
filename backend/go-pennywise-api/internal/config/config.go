package config

import (
	"os"
	"strings"

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
	// S3-compatible object storage for receipts. On Railway these come from the
	// bucket's Credentials tab. When the bucket is unset the API falls back to
	// UploadsDir on local disk.
	S3Endpoint     string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3Region       string
	S3UsePathStyle bool
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
		S3Endpoint:          os.Getenv("AWS_ENDPOINT_URL"),
		S3Bucket:            os.Getenv("AWS_S3_BUCKET_NAME"),
		S3AccessKey:         os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretKey:         os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3Region:            os.Getenv("AWS_DEFAULT_REGION"),
		// Railway serves virtual-hosted style; set "path" only for MinIO and friends.
		S3UsePathStyle: strings.EqualFold(os.Getenv("AWS_S3_URL_STYLE"), "path"),

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
