package otelSDK

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds the configuration required to initialize the OpenTelemetry SDK.
// It dictates the service identity and whether telemetry export is enabled.
type Config struct {
	ServiceName         string // The name of the service (e.g., "cipher", "go-gmail")
	ServiceVersion      string // The version of the service (e.g., "1.0.0")
	Environment         string // Deployment environment (e.g., "production", "development", "local")
	OtelSdkDisabled     string
	OtelTracesExporter  string
	OtelMetricsExporter string
	OtelLogsExporter    string
	LangfuseSecretKey   string
	LangfusePublicKey   string
	LangfuseURL         string
}

// Load reads environment variables and constructs a Config object.
// It attempts to load variables from a .env file if present.
// Defaults: ServiceName="otel", ServiceVersion="0.0.1", Enabled=false.
func Load() *Config {
	_ = godotenv.Load(".env")
	env := os.Getenv("RAILWAY_ENVIRONMENT_NAME")
	if env == "" {
		env = "local"
	}
	serviceName := "otel"
	if os.Getenv("SERVICE_NAME") != "" {
		serviceName = os.Getenv("SERVICE_NAME")
	}

	serviceVersion := "0.0.1"
	if os.Getenv("SERVICE_VERSION") != "" {
		serviceVersion = os.Getenv("SERVICE_VERSION")
	}

	return &Config{
		ServiceName:         serviceName,
		ServiceVersion:      serviceVersion,
		Environment:         env,
		OtelSdkDisabled:     os.Getenv("OTEL_SDK_DISABLED"),
		OtelTracesExporter:  os.Getenv("OTEL_TRACES_EXPORTER"),
		OtelMetricsExporter: os.Getenv("OTEL_METRICS_EXPORTER"),
		OtelLogsExporter:    os.Getenv("OTEL_LOGS_EXPORTER"),
		LangfuseSecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),
		LangfusePublicKey:   os.Getenv("LANGFUSE_PUBLIC_KEY"),
		LangfuseURL:         os.Getenv("LANGFUSE_URL"),
	}
}
