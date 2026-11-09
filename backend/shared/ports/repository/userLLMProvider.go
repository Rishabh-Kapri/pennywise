package repository

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

var (
	ErrLLMProviderNotFound = errors.New("ai provider not configured")
	ErrLLMModelNotFound    = errors.New("ai model not found")
	ErrLLMModelExists      = errors.New("this model is already saved for that key")
)

// UserLLMProviderUpsert is a full key row to write. A nil APIKeyEncrypted with
// KeepAPIKey keeps the stored key.
type UserLLMProviderUpsert struct {
	Provider        model.LLMProviderName
	APIKeyEncrypted []byte
	APIKeyHint      string
	KeepAPIKey      bool
	BaseURL         string
	Enabled         bool
}

// UserLLMProviderRepository stores a user's keys, one per provider.
type UserLLMProviderRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserLLMProvider, error)
	// GetForBudget returns the budget owner's key for provider, including the
	// encrypted key, for cipher.
	GetForBudget(ctx context.Context, budgetID uuid.UUID, provider model.LLMProviderName) (*model.UserLLMProvider, error)
	Upsert(ctx context.Context, userID uuid.UUID, in UserLLMProviderUpsert) (*model.UserLLMProvider, error)
	// Delete removes the key and every model that used it.
	Delete(ctx context.Context, userID uuid.UUID, provider model.LLMProviderName) error
}

// UserLLMModelCreate is a model to add for one of the user's keys.
type UserLLMModelCreate struct {
	Provider    model.LLMProviderName
	Kind        model.LLMModelKind
	Slug        string
	Endpoint    string // decision models only
	MakeDefault bool
	VerifiedAt  bool // the model passed a test just before saving
}

// UserLLMModelRepository stores the models a user runs with their keys. Each
// kind has at most one default per user; the first model of a kind becomes it,
// and deleting the default promotes the oldest remaining one.
type UserLLMModelRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserLLMModel, error)
	// ListUsableForBudget returns the budget owner's enabled models whose key
	// is enabled too, with the key's encrypted secret and base URL, defaults
	// first and then by priority. kinds narrows the result; none means every
	// kind.
	ListUsableForBudget(
		ctx context.Context,
		budgetID uuid.UUID,
		kinds ...model.LLMModelKind,
	) ([]model.UserLLMModel, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*model.UserLLMModel, error)
	// Create fails with ErrLLMProviderNotFound when the user has no key for
	// in.Provider, and ErrLLMModelExists for a duplicate.
	Create(ctx context.Context, userID uuid.UUID, in UserLLMModelCreate) (*model.UserLLMModel, error)
	// Update changes the slug and endpoint and marks the model verified now.
	Update(ctx context.Context, userID, id uuid.UUID, slug, endpoint string) (*model.UserLLMModel, error)
	SetDefault(ctx context.Context, userID, id uuid.UUID) (*model.UserLLMModel, error)
	// SetEnabled switches a model on or off. A disabled default stays the
	// default; the next model by priority stands in until it's enabled again.
	SetEnabled(ctx context.Context, userID, id uuid.UUID, enabled bool) (*model.UserLLMModel, error)
	// Reorder sets the priority of the kind's models from their position in
	// ids, which must list all of them. Otherwise it changes nothing and fails
	// with ErrLLMModelNotFound.
	Reorder(ctx context.Context, userID uuid.UUID, kind model.LLMModelKind, ids []uuid.UUID) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
	// RecordVerification stores a test result on a model of the budget's owner.
	RecordVerification(ctx context.Context, budgetID, id uuid.UUID, testErr string) error
}
