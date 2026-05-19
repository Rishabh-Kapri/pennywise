package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear relevant env vars so defaults kick in
	os.Unsetenv("PORT")
	os.Unsetenv("RAILWAY_ENVIRONMENT_NAME")

	cfg := LoadConfig()

	if cfg.Port != "5170" {
		t.Errorf("expected default port 5170, got %s", cfg.Port)
	}
	if cfg.Environment != "local" {
		t.Errorf("expected default environment 'local', got %s", cfg.Environment)
	}
}

func TestLoadConfig_EnvVarsSet(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("RAILWAY_ENVIRONMENT_NAME", "production")
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	os.Setenv("GOOGLE_ANDROID_CLIENT_ID", "test-android-client-id")
	os.Setenv("CALLBACK_URL", "http://localhost/callback")
	os.Setenv("PROJECT_ID", "my-project")
	os.Setenv("PUBSUB_TOPIC", "my-topic")
	os.Setenv("SUB_NAME", "my-sub")
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("MLP_SERVICE_URL", "http://mlp:8000")
	os.Setenv("PENNYWISE_SERVICE_URL", "http://api:5151")
	os.Setenv("CIPHER_SERVICE_URL", "http://cipher:5160")
	os.Setenv("INTERNAL_AUTH_TOKEN", "secret-token")
	os.Setenv("NTFY_TOPIC", "alerts")
	os.Setenv("TEMPORAL_SERVER_HOST", "temporal")
	os.Setenv("TEMPORAL_SERVER_PORT", "7233")
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS_JSON", `{"type":"service_account"}`)

	defer func() {
		for _, k := range []string{
			"PORT", "RAILWAY_ENVIRONMENT_NAME", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_ANDROID_CLIENT_ID",
			"CALLBACK_URL", "PROJECT_ID", "PUBSUB_TOPIC", "SUB_NAME", "DATABASE_URL",
			"MLP_SERVICE_URL", "PENNYWISE_SERVICE_URL", "CIPHER_SERVICE_URL",
			"INTERNAL_AUTH_TOKEN", "NTFY_TOPIC", "TEMPORAL_SERVER_HOST", "TEMPORAL_SERVER_PORT",
			"GOOGLE_APPLICATION_CREDENTIALS_JSON",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg := LoadConfig()

	checks := map[string]string{
		"Port":                             "9090",
		"Environment":                      "production",
		"GoogleClientId":                   "test-client-id",
		"GoogleClientSecret":               "test-client-secret",
		"GoogleAndroidClientId":            "test-android-client-id",
		"CallbackUrl":                      "http://localhost/callback",
		"ProjectID":                        "my-project",
		"PubsubTopic":                      "my-topic",
		"SubscriptionName":                 "my-sub",
		"DatabaseURL":                      "postgres://localhost/db",
		"MLPServiceURL":                    "http://mlp:8000",
		"PennywiseServiceURL":              "http://api:5151",
		"CipherServiceURL":                 "http://cipher:5160",
		"InternalAuthToken":                "secret-token",
		"NtfyTopic":                        "alerts",
		"TemporalServerHost":               "temporal",
		"TemporalServerPort":               "7233",
		"GoogleApplicationCredentialsJson": `{"type":"service_account"}`,
	}

	cfgMap := map[string]string{
		"Port":                             cfg.Port,
		"Environment":                      cfg.Environment,
		"GoogleClientId":                   cfg.GoogleClientId,
		"GoogleClientSecret":               cfg.GoogleClientSecret,
		"GoogleAndroidClientId":            cfg.GoogleAndroidClientId,
		"CallbackUrl":                      cfg.CallbackUrl,
		"ProjectID":                        cfg.ProjectID,
		"PubsubTopic":                      cfg.PubsubTopic,
		"SubscriptionName":                 cfg.SubscriptionName,
		"DatabaseURL":                      cfg.DatabaseURL,
		"MLPServiceURL":                    cfg.MLPServiceURL,
		"PennywiseServiceURL":              cfg.PennywiseServiceURL,
		"CipherServiceURL":                 cfg.CipherServiceURL,
		"InternalAuthToken":                cfg.InternalAuthToken,
		"NtfyTopic":                        cfg.NtfyTopic,
		"TemporalServerHost":               cfg.TemporalServerHost,
		"TemporalServerPort":               cfg.TemporalServerPort,
		"GoogleApplicationCredentialsJson": cfg.GoogleApplicationCredentialsJson,
	}

	for field, want := range checks {
		if got := cfgMap[field]; got != want {
			t.Errorf("Config.%s: expected %q, got %q", field, want, got)
		}
	}
}
