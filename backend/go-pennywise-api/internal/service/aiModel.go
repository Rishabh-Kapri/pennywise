package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"

	"github.com/google/uuid"
)

var (
	ErrInvalidAIModel = errors.New("invalid ai model")
	// ErrAIModelTestFailed: the model didn't answer the test call, so it
	// wasn't saved. The wrapped message is the provider's error.
	ErrAIModelTestFailed = errors.New("the model didn't pass its test")
)

// MaxModelSlugLength bounds a typed-in model name.
const MaxModelSlugLength = 200

// AIModelService manages the models a user runs with their saved keys. A
// model is tested (through cipher, which holds the keys' decryption) before
// it's saved or changed.
type AIModelService interface {
	List(ctx context.Context, userID uuid.UUID) ([]model.UserLLMModel, error)
	Create(ctx context.Context, userID uuid.UUID, req model.CreateUserLLMModelRequest) (*model.UserLLMModel, error)
	Update(ctx context.Context, userID, id uuid.UUID, req model.UpdateUserLLMModelRequest) (*model.UserLLMModel, error)
	SetDefault(ctx context.Context, userID, id uuid.UUID) (*model.UserLLMModel, error)
	// SetEnabled switches one model on or off without deleting it.
	SetEnabled(ctx context.Context, userID, id uuid.UUID, enabled bool) (*model.UserLLMModel, error)
	// Reorder sets the order of the kind's models. ids must list all of them.
	Reorder(ctx context.Context, userID uuid.UUID, req model.ReorderUserLLMModelsRequest) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
	// Test re-tests a saved model; cipher records the result on it.
	Test(ctx context.Context, userID, id uuid.UUID) (*model.TestUserLLMModelResponse, error)
}

// ModelTester runs a test call. The real one posts to cipher.
type ModelTester func(ctx context.Context, req model.TestUserLLMModelRequest) (*model.TestUserLLMModelResponse, error)

// NewCipherModelTester tests models through cipher's /api/llm/models/test.
func NewCipherModelTester(cipher *transport.Client) ModelTester {
	return func(ctx context.Context, req model.TestUserLLMModelRequest) (*model.TestUserLLMModelResponse, error) {
		res, err := transport.Post[*model.TestUserLLMModelResponse](ctx, cipher, "/api/llm/models/test", nil, req)
		if errs.StatusOf(err) == http.StatusNotFound {
			return nil, repository.ErrLLMProviderNotFound
		}
		return res, err
	}
}

type aiModelService struct {
	repo       repository.UserLLMModelRepository
	test       ModelTester
	isDemoUser IsDemoUser
}

func NewAIModelService(repo repository.UserLLMModelRepository, test ModelTester, isDemoUser IsDemoUser) AIModelService {
	return &aiModelService{repo: repo, test: test, isDemoUser: isDemoUser}
}

func (s *aiModelService) List(ctx context.Context, userID uuid.UUID) ([]model.UserLLMModel, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *aiModelService) Create(
	ctx context.Context,
	userID uuid.UUID,
	req model.CreateUserLLMModelRequest,
) (*model.UserLLMModel, error) {
	if s.isDemoUser(ctx, userID) {
		return nil, ErrAIProviderReadOnly
	}
	if !req.Provider.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAIProvider, req.Provider)
	}
	if !req.Kind.Valid() {
		return nil, fmt.Errorf("%w: kind must be chat, decision or embedding", ErrInvalidAIModel)
	}
	if req.Kind == model.LLMModelEmbedding && !req.Provider.ServesEmbeddings() {
		return nil, fmt.Errorf("%w: embedding models need an OpenRouter or Ollama key", ErrInvalidAIModel)
	}
	slug, endpoint, err := cleanModel(req.Kind, req.Slug, req.Endpoint)
	if err != nil {
		return nil, err
	}
	if err := s.mustPass(ctx, model.TestUserLLMModelRequest{
		Provider: req.Provider, Kind: req.Kind, Slug: slug, Endpoint: endpoint,
	}); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, userID, repository.UserLLMModelCreate{
		Provider: req.Provider, Kind: req.Kind, Slug: slug, Endpoint: endpoint,
		MakeDefault: req.MakeDefault, VerifiedAt: true,
	})
}

func (s *aiModelService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	req model.UpdateUserLLMModelRequest,
) (*model.UserLLMModel, error) {
	if s.isDemoUser(ctx, userID) {
		return nil, ErrAIProviderReadOnly
	}
	existing, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	slug, endpoint, err := cleanModel(existing.Kind, req.Slug, req.Endpoint)
	if err != nil {
		return nil, err
	}
	// Test the new values before saving; a failure leaves the saved model as
	// it was.
	if err := s.mustPass(ctx, model.TestUserLLMModelRequest{
		Provider: existing.Provider, Kind: existing.Kind, Slug: slug, Endpoint: endpoint,
	}); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, userID, id, slug, endpoint)
}

func (s *aiModelService) SetDefault(ctx context.Context, userID, id uuid.UUID) (*model.UserLLMModel, error) {
	if s.isDemoUser(ctx, userID) {
		return nil, ErrAIProviderReadOnly
	}
	return s.repo.SetDefault(ctx, userID, id)
}

func (s *aiModelService) SetEnabled(
	ctx context.Context,
	userID, id uuid.UUID,
	enabled bool,
) (*model.UserLLMModel, error) {
	if s.isDemoUser(ctx, userID) {
		return nil, ErrAIProviderReadOnly
	}
	return s.repo.SetEnabled(ctx, userID, id, enabled)
}

func (s *aiModelService) Reorder(
	ctx context.Context,
	userID uuid.UUID,
	req model.ReorderUserLLMModelsRequest,
) error {
	if s.isDemoUser(ctx, userID) {
		return ErrAIProviderReadOnly
	}
	if !req.Kind.Valid() {
		return fmt.Errorf("%w: kind must be chat, decision or embedding", ErrInvalidAIModel)
	}
	seen := make(map[uuid.UUID]bool, len(req.IDs))
	for _, id := range req.IDs {
		if seen[id] {
			return fmt.Errorf("%w: a model is listed twice", ErrInvalidAIModel)
		}
		seen[id] = true
	}
	return s.repo.Reorder(ctx, userID, req.Kind, req.IDs)
}

func (s *aiModelService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if s.isDemoUser(ctx, userID) {
		return ErrAIProviderReadOnly
	}
	return s.repo.Delete(ctx, userID, id)
}

func (s *aiModelService) Test(ctx context.Context, userID, id uuid.UUID) (*model.TestUserLLMModelResponse, error) {
	m, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.test(ctx, model.TestUserLLMModelRequest{
		ModelID: &m.ID, Provider: m.Provider, Kind: m.Kind, Slug: m.Slug, Endpoint: m.Endpoint,
	})
}

func (s *aiModelService) mustPass(ctx context.Context, req model.TestUserLLMModelRequest) error {
	res, err := s.test(ctx, req)
	if err != nil {
		return err
	}
	if res == nil || !res.OK {
		msg := "no answer"
		if res != nil && res.Error != "" {
			msg = res.Error
		}
		return fmt.Errorf("%w: %s", ErrAIModelTestFailed, msg)
	}
	return nil
}

// cleanModel trims and checks a model's slug and endpoint for its kind.
func cleanModel(kind model.LLMModelKind, slug, endpoint string) (string, string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", "", fmt.Errorf("%w: model is required", ErrInvalidAIModel)
	}
	if len(slug) > MaxModelSlugLength {
		return "", "", fmt.Errorf("%w: model name is too long", ErrInvalidAIModel)
	}
	endpoint = strings.TrimSpace(endpoint)
	if kind != model.LLMModelDecision {
		if endpoint != "" {
			return "", "", fmt.Errorf("%w: only decision models take an endpoint", ErrInvalidAIModel)
		}
		return slug, "", nil
	}
	clean, err := normalizeDecisionEndpoint(endpoint)
	if err != nil {
		return "", "", err
	}
	return slug, clean, nil
}

// normalizeDecisionEndpoint accepts an https URL, or plain http only for a
// local or private address (a self-hosted decisions service). The key is sent
// to this URL, so it must not travel in clear text over the internet. Cipher
// still refuses private addresses unless the server allows them.
func normalizeDecisionEndpoint(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("%w: a decision model needs its endpoint", ErrInvalidAIModel)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("%w: the endpoint must be an https URL", ErrInvalidAIModel)
	}
	if u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("%w: the endpoint can't carry credentials or a fragment", ErrInvalidAIModel)
	}
	if u.Scheme == "http" && !isLocalHost(u.Hostname()) {
		return "", fmt.Errorf("%w: the endpoint must use https", ErrInvalidAIModel)
	}
	return u.String(), nil
}

func isLocalHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}
