package repository

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

var ErrLLMProviderNotFound = errors.New("ai provider not configured")

// UserLLMProviderUpsert is a full row to write. A nil APIKeyEncrypted with
// KeepAPIKey keeps the stored key.
type UserLLMProviderUpsert struct {
	Provider        model.LLMProviderName
	APIKeyEncrypted []byte
	APIKeyHint      string
	KeepAPIKey      bool
	BaseURL         string
	Model           string
	Priority        int
	Enabled         bool
}

type UserLLMProviderRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.UserLLMProvider, error)
	// ListEnabledForBudget returns the budget owner's enabled providers,
	// including encrypted keys, in fallback order.
	ListEnabledForBudget(ctx context.Context, budgetID uuid.UUID) ([]model.UserLLMProvider, error)
	GetForBudget(ctx context.Context, budgetID uuid.UUID, provider model.LLMProviderName) (*model.UserLLMProvider, error)
	Upsert(ctx context.Context, userID uuid.UUID, in UserLLMProviderUpsert) (*model.UserLLMProvider, error)
	Delete(ctx context.Context, userID uuid.UUID, provider model.LLMProviderName) error
	RecordVerification(ctx context.Context, budgetID uuid.UUID, provider model.LLMProviderName, testErr string) error
}
