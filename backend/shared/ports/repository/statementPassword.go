package repository

import (
	"context"

	"github.com/google/uuid"
)

// StatementPasswordRepository stores each account's sealed statement PDF
// password. It never sees the plain password: the API seals it, cipher opens it
// when a statement job runs. Both use LLM_CREDENTIALS_KEY.
type StatementPasswordRepository interface {
	// Set saves (or replaces) the account's sealed password.
	Set(ctx context.Context, budgetID, accountID uuid.UUID, sealed []byte) error
	// Delete removes it. Removing one that isn't there is not an error.
	Delete(ctx context.Context, budgetID, accountID uuid.UUID) error
	// ListByBudget returns the budget's sealed passwords by account.
	ListByBudget(ctx context.Context, budgetID uuid.UUID) (map[uuid.UUID][]byte, error)
}
