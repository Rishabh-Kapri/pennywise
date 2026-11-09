// Package auth is the authentication port: credentials go in, a Principal
// (who is calling) comes out. It knows nothing about HTTP or databases; the
// gin middleware (shared/middleware) extracts the credentials and the
// adapters (shared/adapters/auth) check them against repository ports.
package auth

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type Kind string

const (
	UserKind    Kind = "user"
	APIKeyKind  Kind = "api_key"
	ServiceKind Kind = "service"
)

type Principal struct {
	Kind    Kind
	UserID  *uuid.UUID
	Service string
	Scopes  []model.Scope
	// APIKey is the key record, set only for APIKeyKind. Route scope checks
	// and per-key rate limiting read it.
	APIKey *model.APIKey
}

type Credentials struct {
	APIKey       string
	BearerToken  string
	ServiceToken string
}

type Authenticator interface {
	// Authenticate checks the one credential this authenticator handles.
	//
	//   - (nil, nil): creds don't hold that credential; try the next authenticator.
	//   - (principal, nil): accepted.
	//   - (nil, err): the credential is there but was rejected, or couldn't be
	//     checked. Stop: errs.CodeAuthInvalidCredentials means a bad credential,
	//     errs.CodeAuthLookupFailed means the check itself failed.
	Authenticate(ctx context.Context, creds Credentials) (*Principal, error)
}

type principalKey struct{}

// WithPrincipal returns a ctx carrying p.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext returns the authenticated caller, or nil.
func PrincipalFromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}
