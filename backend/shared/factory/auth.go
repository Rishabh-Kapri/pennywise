package factory

import (
	"context"
	"strings"

	authAdapter "github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/auth"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

// ServiceAuthConfig configures which services may call this one.
type ServiceAuthConfig struct {
	// Tokens is SERVICE_TOKENS: "name:token,name:token", one entry per
	// service that may call in.
	Tokens string
	// OwnToken is this service's INTERNAL_AUTH_TOKEN. Only used when Tokens
	// is empty: then the old single shared token is accepted, so deploys keep
	// working until SERVICE_TOKENS is set.
	OwnToken string
}

// NewServiceAuthenticator returns the authenticator InternalRequestAuth uses
// to verify X-Internal-Token.
func NewServiceAuthenticator(ctx context.Context, cfg ServiceAuthConfig) (auth.Authenticator, error) {
	tokens, err := parseServiceTokens(cfg.Tokens)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		logger.Logger(ctx).Warn("SERVICE_TOKENS not set: accepting the shared INTERNAL_AUTH_TOKEN; callers can't be told apart")
		// Empty name: the caller is verified, but its identity is unknown.
		tokens = map[string]string{"": cfg.OwnToken}
	}
	return authAdapter.NewServiceTokenAdapter(tokens), nil
}

// parseServiceTokens reads "name:token,name:token". Spaces around entries
// are ignored.
func parseServiceTokens(raw string) (map[string]string, error) {
	tokens := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, token, ok := strings.Cut(entry, ":")
		name, token = strings.TrimSpace(name), strings.TrimSpace(token)
		if !ok || name == "" || token == "" {
			// Never echo the entry: it holds a token.
			return nil, errs.New(errs.CodeInvalidArgument, "SERVICE_TOKENS entries must be name:token")
		}
		if _, dup := tokens[name]; dup {
			return nil, errs.New(errs.CodeInvalidArgument, "SERVICE_TOKENS lists %q twice", name)
		}
		tokens[name] = token
	}
	return tokens, nil
}

type AuthConfig struct {
	// JWTSecret verifies user access tokens. Empty disables JWT auth.
	JWTSecret string
}

// NewAuthenticators returns the authenticators for middleware.Authenticate,
// in the order they are tried: an API key wins over a bearer token.
//
// They take repository ports, not a connection, so they work with whichever
// database adapter ConnectDB picked (or a fake in tests).
func NewAuthenticators(
	ctx context.Context,
	cfg AuthConfig,
	apiKeys repository.APIKeyRepository,
	users repository.AuthRepository,
) []auth.Authenticator {
	authenticators := []auth.Authenticator{authAdapter.NewAPIKeyAdapter(apiKeys)}
	if cfg.JWTSecret == "" {
		// An empty HMAC key would accept tokens anyone can sign.
		logger.Logger(ctx).Warn("JWT_SECRET not set: bearer tokens will be rejected")
	} else {
		authenticators = append(authenticators, authAdapter.NewJWTAdapter(cfg.JWTSecret, users))
	}
	return authenticators
}
