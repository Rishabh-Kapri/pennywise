package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type APIKeyService interface {
	Create(ctx context.Context, apiKey *model.APIKey) (string, error)
	GetAll(ctx context.Context) ([]model.APIKey, error)
	GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error)
}

type apiKeyService struct {
	repo repository.APIKeyRepository
}

func NewApiKeyService(r repository.APIKeyRepository) APIKeyService {
	return &apiKeyService{repo: r}
}

func (s *apiKeyService) setAPIKeyDefaults(apiKey *model.APIKey) {
	apiKey.RateLimit = 1000
	apiKey.IsActive = true
	apiKey.Scopes = []model.Scope{model.ScopeRead}
	// @TODO: make expiry configurable
	expiry := time.Now().Add(time.Hour * 24 * 30) // expires in 30 days
	apiKey.ExpiresAt = &expiry
}

// generateAPIKey makes a new secret key in the format shared/model defines, so
// the auth adapter accepts exactly what it produces, and a public key id from
// separate random bytes.
func generateAPIKey() (fullKey string, keyID string, err error) {
	randomBytes := make([]byte, model.APIKeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", errs.Wrap(errs.CodeInternalError, "failed to generate random bytes", err)
	}
	fullKey = fmt.Sprintf("%s_%s_%s", model.APIKeyPrefix, model.APIKeyVersion,
		base64.RawURLEncoding.EncodeToString(randomBytes))

	idBytes := make([]byte, model.APIKeyIDLength)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", errs.Wrap(errs.CodeInternalError, "failed to generate key id", err)
	}
	keyID = model.APIKeyIDPrefix + hex.EncodeToString(idBytes)

	return fullKey, keyID, nil
}

func (s *apiKeyService) Create(ctx context.Context, apiKey *model.APIKey) (string, error) {
	logger.Logger(ctx).Info("creating api key", "headers", utils.SanitizeHeadersForLogging(utils.BuildInternalHeaders(ctx)))
	userID := utils.MustUserID(ctx)

	if apiKey.Name == "" {
		return "", errs.New(errs.CodeInvalidArgument, "name is required")
	}

	// Set default values
	s.setAPIKeyDefaults(apiKey)

	// Generate a new API key
	fullKey, keyID, err := generateAPIKey()
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "failed to generate api key", err)
	}
	// Hash the key
	apiKey.HashedKey = utils.Hash(fullKey)

	apiKey.KeyID = keyID
	apiKey.UserID = userID

	// Create the API key
	err = s.repo.Create(ctx, apiKey)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "failed to create api key", err)
	}
	return fullKey, nil
}

func (s *apiKeyService) GetAll(ctx context.Context) ([]model.APIKey, error) {
	userID := utils.MustUserID(ctx)
	return s.repo.GetAll(ctx, userID)
}

func (s *apiKeyService) GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error) {
	return s.repo.GetByKeyID(ctx, keyID)
}
