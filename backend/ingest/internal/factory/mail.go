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

type MailConfig struct {
	// Providers is MAIL_PROVIDERS: which mail adapters to build (e.g. "gmail").
	// Empty builds none.
	Providers []string
	// CredentialsKey is CREDENTIALS_KEY: the base64 32-byte key that seals
	// Connection.Credentials. Changing it makes stored connections unreadable.
	CredentialsKey string
	Gmail          GmailConfig
}

// GmailConfig is the Gmail adapter's settings. Leave Box unset: the factory
// makes it from CredentialsKey.
type GmailConfig = gmail.Config

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

// NewMailProviders builds every provider named in cfg.Providers. Each one
// checks its settings now, so a misconfigured deploy fails at startup.
func NewMailProviders(ctx context.Context, cfg MailConfig) (MailProviderRegistry, error) {
	providers := MailProviderRegistry{}
	if len(cfg.Providers) == 0 {
		logger.Logger(ctx).Info("no mail providers configured (MAIL_PROVIDERS)")
		return providers, nil
	}

	box, err := utils.NewSecretBox(cfg.CredentialsKey)
	if err != nil {
		return nil, errs.Wrap(errs.CodeMailNotConfigured, "CREDENTIALS_KEY must be a base64 32-byte key", err)
	}

	for _, name := range cfg.Providers {
		name = strings.ToLower(name)
		if _, built := providers[name]; built {
			continue
		}
		var provider ports.MailProvider
		switch name {
		case ports.ProviderGmail:
			gmailCfg := cfg.Gmail
			gmailCfg.Box = box
			provider, err = gmail.New(gmailCfg)
		default:
			err = errs.New(errs.CodeAdapterNotConfigured, "unknown mail provider %q in MAIL_PROVIDERS", name)
		}
		if err != nil {
			return nil, err
		}
		providers[name] = provider
		logger.Logger(ctx).Info("mail provider configured", "provider", name)
	}
	return providers, nil
}
