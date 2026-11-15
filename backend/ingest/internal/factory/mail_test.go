package factory

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

func validConfig() MailConfig {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return MailConfig{
		Providers:      []string{"Gmail", "gmail"},
		CredentialsKey: base64.StdEncoding.EncodeToString(key),
		Gmail: GmailConfig{
			WebClientID: "id", WebClientSecret: "secret", ProjectID: "p", PubsubTopic: "t",
		},
	}
}

func TestNewMailProviders(t *testing.T) {
	ctx := context.Background()

	t.Run("none configured", func(t *testing.T) {
		p, err := NewMailProviders(ctx, MailConfig{})
		if err != nil || len(p) != 0 {
			t.Fatalf("got (%v, %v)", p, err)
		}
		if _, err := p.For("gmail"); !errs.HasCode(err, errs.CodeAdapterNotConfigured) {
			t.Fatalf("For = %v", err)
		}
	})

	t.Run("gmail", func(t *testing.T) {
		p, err := NewMailProviders(ctx, validConfig())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.For("gmail"); err != nil || len(p) != 1 {
			t.Fatalf("registry = %v, For err = %v", p, err)
		}
	})

	fails := map[string]func(*MailConfig){
		"unknown provider":      func(c *MailConfig) { c.Providers = []string{"outlook"} },
		"bad credentials key":   func(c *MailConfig) { c.CredentialsKey = "short" },
		"no client id":          func(c *MailConfig) { c.Gmail.WebClientID, c.Gmail.WebClientSecret = "", "" },
		"web id without secret": func(c *MailConfig) { c.Gmail.WebClientSecret = "" },
		"no pubsub topic":       func(c *MailConfig) { c.Gmail.PubsubTopic = "" },
	}
	for name, breakIt := range fails {
		t.Run(name+" fails startup", func(t *testing.T) {
			cfg := validConfig()
			breakIt(&cfg)
			if _, err := NewMailProviders(ctx, cfg); err == nil {
				t.Fatal("want error")
			}
		})
	}

	t.Run("android client alone is enough", func(t *testing.T) {
		cfg := validConfig()
		cfg.Gmail.WebClientID, cfg.Gmail.WebClientSecret, cfg.Gmail.AndroidClientID = "", "", "android"
		if _, err := NewMailProviders(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	})
}
