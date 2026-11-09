package auth

import (
	"context"
	"crypto/subtle"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	authPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
)

type serviceToken struct {
	tokens map[string]string // service name → token
}

// NewServiceTokenAdapter returns the Authenticator for
// Credentials.ServiceToken. tokens maps each service's name to the token it
// sends; the matching name becomes Principal.Service, so a caller's identity
// comes from its token, never from a header it chose. Empty tokens are
// ignored.
func NewServiceTokenAdapter(tokens map[string]string) authPort.Authenticator {
	clean := make(map[string]string, len(tokens))
	for name, token := range tokens {
		if token != "" {
			clean[name] = token
		}
	}
	return &serviceToken{tokens: clean}
}

// Authenticate implements [authPort.Authenticator]. A service principal
// never carries a user: services act on budgets, not as users.
func (s *serviceToken) Authenticate(_ context.Context, creds authPort.Credentials) (*authPort.Principal, error) {
	if creds.ServiceToken == "" {
		return nil, nil
	}
	// Compare against every token in constant time, so timing reveals
	// neither which service matched nor how much of a token was right.
	// The name can legitimately be "" (factory's shared-token fallback), so
	// track the match separately.
	matched, found := "", false
	for name, token := range s.tokens {
		if subtle.ConstantTimeCompare([]byte(creds.ServiceToken), []byte(token)) == 1 {
			matched, found = name, true
		}
	}
	if !found {
		return nil, errs.New(errs.CodeAuthInvalidCredentials, "invalid service token")
	}
	return &authPort.Principal{Kind: authPort.ServiceKind, Service: matched}, nil
}
