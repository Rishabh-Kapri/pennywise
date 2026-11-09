package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

var (
	ErrInvalidAIProvider     = errors.New("invalid ai provider")
	ErrAIProviderKeyRequired = errors.New("an API key is required for this provider")
	ErrAIProviderReadOnly    = errors.New("ai providers can't be changed on the demo account")
	ErrAIProvidersDisabled   = errors.New("AI providers are disabled: LLM_CREDENTIALS_KEY is not set on the server")
)

// IsDemoUser reports whether userID is the shared demo account.
type IsDemoUser func(ctx context.Context, userID uuid.UUID) bool

type AIProviderService interface {
	List(ctx context.Context, userID uuid.UUID) ([]model.UserLLMProvider, error)
	Upsert(
		ctx context.Context,
		userID uuid.UUID,
		provider model.LLMProviderName,
		req model.UpsertUserLLMProviderRequest,
	) (*model.UserLLMProvider, error)
	// Delete removes the key and every model that used it.
	Delete(ctx context.Context, userID uuid.UUID, provider model.LLMProviderName) error
}

// aiProviderService manages a user's keys, one per provider. Models used with
// the keys are AIModelService's.
type aiProviderService struct {
	repo       repository.UserLLMProviderRepository
	box        *utils.SecretBox // nil when LLM_CREDENTIALS_KEY is unset
	isDemoUser IsDemoUser
}

func NewAIProviderService(
	repo repository.UserLLMProviderRepository,
	box *utils.SecretBox,
	isDemoUser IsDemoUser,
) AIProviderService {
	return &aiProviderService{repo: repo, box: box, isDemoUser: isDemoUser}
}

func (s *aiProviderService) List(ctx context.Context, userID uuid.UUID) ([]model.UserLLMProvider, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *aiProviderService) Upsert(
	ctx context.Context,
	userID uuid.UUID,
	provider model.LLMProviderName,
	req model.UpsertUserLLMProviderRequest,
) (*model.UserLLMProvider, error) {
	if s.isDemoUser(ctx, userID) {
		return nil, ErrAIProviderReadOnly
	}
	if s.box == nil {
		return nil, ErrAIProvidersDisabled
	}
	in, err := s.buildUpsert(ctx, userID, provider, req)
	if err != nil {
		return nil, err
	}
	return s.repo.Upsert(ctx, userID, *in)
}

func (s *aiProviderService) buildUpsert(
	ctx context.Context,
	userID uuid.UUID,
	provider model.LLMProviderName,
	req model.UpsertUserLLMProviderRequest,
) (*repository.UserLLMProviderUpsert, error) {
	if !provider.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAIProvider, provider)
	}
	in := repository.UserLLMProviderUpsert{
		Provider: provider,
		Enabled:  req.Enabled == nil || *req.Enabled,
	}

	existing, err := s.findExisting(ctx, userID, provider)
	if err != nil {
		return nil, err
	}

	if provider == model.LLMProviderOllama {
		baseURL, err := normalizeProviderURL(req.BaseURL)
		if err != nil {
			return nil, err
		}
		in.BaseURL = baseURL
	} else if req.BaseURL != "" {
		return nil, fmt.Errorf("%w: only ollama takes a base URL", ErrInvalidAIProvider)
	}

	switch {
	case req.APIKey == nil:
		in.KeepAPIKey = true
		if existing == nil || !existing.HasAPIKey {
			if provider != model.LLMProviderOllama {
				return nil, ErrAIProviderKeyRequired
			}
		}
	case strings.TrimSpace(*req.APIKey) == "":
		if provider != model.LLMProviderOllama {
			return nil, ErrAIProviderKeyRequired
		}
	default:
		key := strings.TrimSpace(*req.APIKey)
		sealed, err := s.box.Seal(key)
		if err != nil {
			return nil, err
		}
		in.APIKeyEncrypted = sealed
		in.APIKeyHint = keyHint(key)
	}
	return &in, nil
}

func (s *aiProviderService) findExisting(
	ctx context.Context,
	userID uuid.UUID,
	provider model.LLMProviderName,
) (*model.UserLLMProvider, error) {
	providers, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range providers {
		if providers[i].Provider == provider {
			return &providers[i], nil
		}
	}
	return nil, nil
}

func (s *aiProviderService) Delete(ctx context.Context, userID uuid.UUID, provider model.LLMProviderName) error {
	if s.isDemoUser(ctx, userID) {
		return ErrAIProviderReadOnly
	}
	return s.repo.Delete(ctx, userID, provider)
}

// normalizeProviderURL accepts an http(s) origin with an optional path.
// Cipher additionally refuses to dial private addresses (transport.NewUserURLClient).
func normalizeProviderURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", fmt.Errorf("%w: a base URL is required for ollama", ErrInvalidAIProvider)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%w: base URL must be an http(s) URL", ErrInvalidAIProvider)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: base URL can't carry credentials, a query or a fragment", ErrInvalidAIProvider)
	}
	return raw, nil
}

func keyHint(key string) string {
	if len(key) <= 8 {
		return ""
	}
	return key[len(key)-4:]
}

// NewDemoUserCheck reports the seeded demo account when demo mode is on. Its
// credentials would be shared by every demo visitor, so it can't save any.
func NewDemoUserCheck(googleProvider repository.GoogleProviderRepository, demoMode bool) IsDemoUser {
	return func(ctx context.Context, userID uuid.UUID) bool {
		if !demoMode {
			return false
		}
		demo, err := googleProvider.GetUserByGoogleIDAndClientType(ctx, demoGoogleID, model.GoogleOAuthClientTypeWeb)
		if err != nil || demo == nil || demo.AuthUser == nil {
			return false
		}
		return demo.AuthUser.ID == userID
	}
}
