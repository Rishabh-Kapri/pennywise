package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	authPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// lastUsedTimeout bounds the background last_used write.
const lastUsedTimeout = 5 * time.Second

type apiKey struct {
	repo repository.APIKeyRepository
}

// NewAPIKeyAdapter returns the Authenticator for Credentials.APIKey.
func NewAPIKeyAdapter(repo repository.APIKeyRepository) authPort.Authenticator {
	return &apiKey{repo: repo}
}

// Authenticate implements [authPort.Authenticator].
//
//   - No API key in creds: (nil, nil), not this authenticator's credential.
//   - Malformed, unknown, inactive, expired or revoked key: CodeAuthInvalidCredentials.
//   - The lookup itself failed: CodeAuthLookupFailed (a server problem, not a bad key).
//
// The key is a secret: never log it or put it in an error.
func (a *apiKey) Authenticate(ctx context.Context, creds authPort.Credentials) (*authPort.Principal, error) {
	if creds.APIKey == "" {
		return nil, nil
	}
	if !model.ValidAPIKeyFormat(creds.APIKey) {
		return nil, errs.New(errs.CodeAuthInvalidCredentials, "invalid api key")
	}

	key, err := a.repo.GetByHash(ctx, utils.Hash(creds.APIKey))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errs.New(errs.CodeAuthInvalidCredentials, "invalid api key")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "look up api key", err)
	}
	if !key.IsValid() {
		return nil, errs.NewWith(errs.CodeAuthInvalidCredentials, "invalid api key",
			errs.WithDetail("key_id", key.KeyID))
	}

	a.touchLastUsed(ctx, key.ID, key.KeyID)

	userID := key.UserID
	return &authPort.Principal{
		Kind:   authPort.APIKeyKind,
		UserID: &userID,
		Scopes: key.Scopes,
		APIKey: key,
	}, nil
}

// touchLastUsed records the use in the background so auth doesn't wait on a
// write. The goroutine gets ctx's values (for logs) but not its cancellation:
// the request finishing must not cancel the write.
func (a *apiKey) touchLastUsed(ctx context.Context, id uuid.UUID, keyID string) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lastUsedTimeout)
	go func() {
		defer cancel()
		if err := a.repo.UpdateLastUsed(writeCtx, id); err != nil {
			logger.Logger(writeCtx).Warn("api key last_used not updated", "key_id", keyID, "err", err)
		}
	}()
}
