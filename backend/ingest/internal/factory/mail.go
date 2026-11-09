// Package factory builds ingest's adapters from config. It is the only
// package outside internal/adapters that imports an adapter; main gets ports
// from here.
package factory

import (
	"context"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/adapters/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const ProviderGmail = "gmail"

type MailConfig struct {
	// Providers is MAIL_PROVIDERS: which mail adapters to build, comma
	// separated (e.g. "gmail"). Empty builds none.
	Providers string
	// CredentialsKey is MAIL_CREDENTIALS_KEY: the base64 32-byte key that
	// seals Connection.Credentials. Changing it makes stored connections
	// unreadable.
	CredentialsKey string
	Gmail          GmailConfig
}

type GmailConfig struct {
	WebClientID     string
	WebClientSecret string
	AndroidClientID string
	ProjectID       string
	PubsubTopic     string
}

// MailProviderRegistry is the registry of built mail adapters, keyed by Connection.Provider.
type MailProviderRegistry map[string]ports.MailProvider

// For returns the adapter for a connection's provider.
func (m MailProviderRegistry) For(provider string) (ports.MailProvider, error) {
	p, ok := m[provider]
	if !ok {
		return nil, errs.New(errs.CodeAdapterNotConfigured, "no mail provider configured for %q", provider)
	}
	return p, nil
}

// NewMailProviders builds every provider named in cfg.Providers. It checks
// each one's settings now, so a misconfigured deploy fails at startup, not on
// the first email.
func NewMailProviders(ctx context.Context, cfg MailConfig) (MailProviderRegistry, error) {
	names := parseNames(cfg.Providers)
	providers := MailProviderRegistry{}
	if len(names) == 0 {
		logger.Logger(ctx).Info("no mail providers configured (MAIL_PROVIDERS)")
		return providers, nil
	}

	box, err := utils.NewSecretBox(cfg.CredentialsKey)
	if err != nil {
		return nil, errs.Wrap(errs.CodeMailNotConfigured, "MAIL_CREDENTIALS_KEY must be a base64 32-byte key", err)
	}

	for _, name := range names {
		switch name {
		case ProviderGmail:
			g := cfg.Gmail
			hasWeb := g.WebClientID != "" && g.WebClientSecret != ""
			if !hasWeb && g.AndroidClientID == "" {
				return nil, errs.New(errs.CodeMailNotConfigured,
					"gmail needs GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET, or GOOGLE_ANDROID_CLIENT_ID")
			}
			if g.ProjectID == "" || g.PubsubTopic == "" {
				return nil, errs.New(errs.CodeMailNotConfigured, "gmail needs PROJECT_ID and PUBSUB_TOPIC")
			}
			providers[name] = gmail.New(gmail.Config{
				WebClientID:     g.WebClientID,
				WebClientSecret: g.WebClientSecret,
				AndroidClientID: g.AndroidClientID,
				ProjectID:       g.ProjectID,
				PubsubTopic:     g.PubsubTopic,
				Box:             box,
			})
		default:
			return nil, errs.New(errs.CodeAdapterNotConfigured, "unknown mail provider %q in MAIL_PROVIDERS", name)
		}
		logger.Logger(ctx).Info("mail provider configured", "provider", name)
	}
	return providers, nil
}

func parseNames(raw string) []string {
	var names []string
	seen := map[string]bool{}
	for _, n := range strings.Split(raw, ",") {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}
