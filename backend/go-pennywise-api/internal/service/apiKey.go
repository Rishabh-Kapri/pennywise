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
	"github.com/google/uuid"
)

// The key format lives in shared/model so the auth adapter checks exactly
// what Generate produces.
const (
	KeyLength   = model.APIKeyLength
	KeyPrefix   = model.APIKeyPrefix
	KeyVersion  = model.APIKeyVersion
	KeyIDPrefix = model.APIKeyIDPrefix
	KeyIDLength = model.APIKeyIDLength
)

type APIKeyService interface {
	Generate() (fullKey, keyID string, err error)
	ParseKey(fullKey string) (prefix, version, base64String string, err error)
	ValidateFormat(fullKey string) bool
	// repository methods
	Create(ctx context.Context, apiKey *model.APIKey) (string, error)
	GetAll(ctx context.Context) ([]model.APIKey, error)
	GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error)
	GetByHash(ctx context.Context, fullKey string) (*model.APIKey, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

type apiKeyService struct {
	prefix  string
	version string
	repo    repository.APIKeyRepository
}

func NewApiKeyService(r repository.APIKeyRepository) APIKeyService {
	return &apiKeyService{
		prefix:  KeyPrefix,
		version: KeyVersion,
		repo:    r,
	}
}

func (s *apiKeyService) setAPIKeyDefaults(apiKey *model.APIKey) {
	apiKey.RateLimit = 1000
	apiKey.IsActive = true
	apiKey.Scopes = []model.Scope{model.ScopeRead}
	// @TODO: make expiry configurable
	expiry := time.Now().Add(time.Hour * 24 * 30) // expires in 30 days
	apiKey.ExpiresAt = &expiry
}

// Generate creates a new cryptographically secure API key
func (s *apiKeyService) Generate() (fullKey string, keyID string, err error) {
	// Generate random bytes
	randomBytes := make([]byte, KeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", errs.Wrap(errs.CodeInternalError, "failed to generate random bytes", err)
	}

	// Encode to URL-safe base64 for easy transmission
	base64String := base64.RawURLEncoding.EncodeToString(randomBytes)

	fullKey = fmt.Sprintf("%s_%s_%s", s.prefix, s.version, base64String)

	// A short public id for lookup and display, independent of the secret.
	idBytes := make([]byte, KeyIDLength)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", errs.Wrap(errs.CodeInternalError, "failed to generate key id", err)
	}
	keyID = KeyIDPrefix + hex.EncodeToString(idBytes)

	return fullKey, keyID, nil
}

func (s *apiKeyService) ParseKey(fullKey string) (prefix, version, base64String string, err error) {
	prefix, version, base64String, ok := model.ParseAPIKey(fullKey)
	if !ok {
		return "", "", "", errs.New(errs.CodeInvalidArgument, "invalid api key format")
	}
	return prefix, version, base64String, nil
}

// ValidateFormat never logs fullKey: it is the secret.
func (s *apiKeyService) ValidateFormat(fullKey string) bool {
	return model.ValidAPIKeyFormat(fullKey)
}

func (s *apiKeyService) Create(ctx context.Context, apiKey *model.APIKey) (string, error) {
	logger.Logger(ctx).Info("creating api key", "headers", utils.SanitizeHeadersForLogging(utils.GetHeaders(ctx)))
	userID := utils.MustUserID(ctx)

	if apiKey.Name == "" {
		return "", errs.New(errs.CodeInvalidArgument, "name is required")
	}

	// Set default values
	s.setAPIKeyDefaults(apiKey)

	// Generate a new API key
	fullKey, keyID, err := s.Generate()
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

func (s *apiKeyService) GetByHash(ctx context.Context, fullKey string) (*model.APIKey, error) {
	keyHash := utils.Hash(fullKey)
	logger.Logger(ctx).Debug("looking up api key by hash", "keyHash", keyHash)
	return s.repo.GetByHash(ctx, keyHash)
}

func (s *apiKeyService) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdateLastUsed(ctx, id)
}
