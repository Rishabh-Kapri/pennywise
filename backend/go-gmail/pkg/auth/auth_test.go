package auth

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestGetOauth2Config(t *testing.T) {
	cfg := &config.Config{
		GoogleClientId:     "test-client-id",
		GoogleClientSecret: "test-client-secret",
		CallbackUrl:        "http://localhost/callback",
	}
	svc := NewService(cfg)
	oauth2Cfg := svc.GetOauth2Config()

	if oauth2Cfg.ClientID != cfg.GoogleClientId {
		t.Errorf("expected ClientID %q, got %q", cfg.GoogleClientId, oauth2Cfg.ClientID)
	}
	if oauth2Cfg.ClientSecret != cfg.GoogleClientSecret {
		t.Errorf("expected ClientSecret %q, got %q", cfg.GoogleClientSecret, oauth2Cfg.ClientSecret)
	}
	if oauth2Cfg.RedirectURL != cfg.CallbackUrl {
		t.Errorf("expected RedirectURL %q, got %q", cfg.CallbackUrl, oauth2Cfg.RedirectURL)
	}
	if len(oauth2Cfg.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(oauth2Cfg.Scopes))
	}
}

func TestGetTokenFromRefresh_InvalidToken(t *testing.T) {
	// With empty credentials, token exchange should fail (no real OAuth server)
	cfg := &config.Config{
		GoogleClientId:     "bad-client-id",
		GoogleClientSecret: "bad-secret",
		CallbackUrl:        "http://localhost/callback",
	}
	svc := NewService(cfg)
	_, err := svc.GetTokenFromRefresh("invalid-refresh-token", model.GoogleOAuthClientTypeWeb)
	if err == nil {
		t.Error("expected error for invalid refresh token, got nil")
	}
}
