package auth

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	authPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
)

type jwtAuth struct {
	secret []byte
	users  repository.AuthRepository
}

// NewJWTAdapter returns the Authenticator for Credentials.BearerToken: the
// HS256 access tokens go-pennywise-api issues. secret must not be empty
// (an empty HMAC key would accept tokens anyone can sign).
func NewJWTAdapter(secret string, users repository.AuthRepository) authPort.Authenticator {
	return &jwtAuth{secret: []byte(secret), users: users}
}

// Authenticate implements [authPort.Authenticator].
//
// Besides the signature and expiry, the token's "version" claim must match
// the user's token_version: bumping it (logout from all devices) revokes
// every access token issued before. Refresh tokens carry no version, so they
// can't be used as access tokens.
func (a *jwtAuth) Authenticate(ctx context.Context, creds authPort.Credentials) (*authPort.Principal, error) {
	if creds.BearerToken == "" {
		return nil, nil
	}

	token, err := jwt.Parse(creds.BearerToken, func(*jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "invalid or expired token", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errs.New(errs.CodeAuthInvalidCredentials, "invalid token claims")
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "invalid token subject", err)
	}
	userID, err := uuid.Parse(sub)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "invalid user id in token", err)
	}
	// JSON numbers decode as float64.
	version, ok := claims["version"].(float64)
	if !ok {
		return nil, errs.New(errs.CodeAuthInvalidCredentials, "token has no version")
	}

	current, err := a.users.GetTokenVersion(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, errs.NewWith(errs.CodeAuthInvalidCredentials, "user not found",
			errs.WithDetail("user_id", userID))
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "look up token version", err,
			errs.WithDetail("user_id", userID))
	}
	if int(version) != current {
		return nil, errs.NewWith(errs.CodeAuthInvalidCredentials, "token revoked, please login again",
			errs.WithDetail("user_id", userID))
	}

	return &authPort.Principal{Kind: authPort.UserKind, UserID: &userID}, nil
}
