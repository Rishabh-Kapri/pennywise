package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

var (
	KeyLength  = 32
	KeyPrefix  = "pwk" // PennyWise Key
	KeyVersion = "v1"
)

type APIKeyService interface {
	Generate() (fullKey, keyID string, err error)
	ParseKey(fullKey string) (prefix, version, base64String string, err error)
	ValidateFormat(fullKey string) bool
	// repository methods
	Create(ctx context.Context, apiKey *model.APIKey) (string, error)
	GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error)
	GetByHash(ctx context.Context, fullKey string) (*model.APIKey, error)
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

	// Generate a short key ID for lookup
	fullKey = fmt.Sprintf("%s_%s_%s", s.prefix, s.version, base64String)

	return fullKey, keyID, nil
}

func (s *apiKeyService) ParseKey(fullKey string) (prefix, version, base64String string, err error) {
	parts := strings.Split(fullKey, "_")
	if len(parts) != 3 {
		return "", "", "", errs.New(errs.CodeInvalidArgument, "invalid api key format")
	}

	return parts[0], parts[1], parts[2], nil
}

func (s *apiKeyService) ValidateFormat(fullKey string) bool {
	prefix, version, randomPart, err := s.ParseKey(fullKey)
	if err != nil {
		return false
	}

	// Verify prefix matches
	if prefix != s.prefix {
		return false
	}

	// Verify version is recognized
	if version != s.version {
		return false
	}

	// Verify random part has expected length
	expectedLen := base64.RawURLEncoding.EncodedLen(KeyLength)
	if len(randomPart) != expectedLen {
		return false
	}

	return true
}

func (s *apiKeyService) Create(ctx context.Context, apiKey *model.APIKey) (string, error) {
	logger.Logger(ctx).Info("creating api key", "headers", utils.GetHeaders(ctx))
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
	err = s.repo.Create(ctx, nil, apiKey)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "failed to create api key", err)
	}
	return fullKey, nil
}

func (s *apiKeyService) GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error) {
	return s.repo.GetByKeyID(ctx, nil, keyID)
}

func (s *apiKeyService) GetByHash(ctx context.Context, fullKey string) (*model.APIKey, error) {
	keyHash := utils.Hash(fullKey)
	logger.Logger(ctx).Debug("GetByHash", "fullKey", fullKey, "keyHash", keyHash)
	return s.repo.GetByHash(ctx, nil, keyHash)
}
