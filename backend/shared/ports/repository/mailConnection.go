package repository

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// MailConnectionRepository stores connected mailboxes. Accounts are compared
// case-insensitively (stored lowercase). Lookups that find nothing return an
// error with code CONNECTION_NOT_FOUND that also matches ErrNotFound.
type MailConnectionRepository interface {
	// Upsert connects a mailbox to a budget. Connecting a mailbox that is
	// already connected replaces its credentials, adds the budget (once),
	// marks it active and unpauses it, so "reconnect" and "add to another
	// budget" are the same call.
	Upsert(ctx context.Context, provider, account string, credentials []byte, budgetID uuid.UUID) (*model.MailConnection, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.MailConnection, error)
	// GetByAccount finds the connection a provider notification is about.
	GetByAccount(ctx context.Context, provider, account string) (*model.MailConnection, error)
	// ListByBudget lists the mailboxes feeding a budget.
	ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.MailConnection, error)
	// ListWatchesExpiring lists enabled connections whose watch is missing or
	// ends before the given time, for renewal. Connections waiting for the
	// user to reconnect and paused ones are left out.
	ListWatchesExpiring(ctx context.Context, before time.Time) ([]model.MailConnection, error)
	// DetachBudget removes a budget. Removing the last one deletes the
	// connection and its credentials; an empty BudgetIDs in the result says
	// so.
	DetachBudget(ctx context.Context, id, budgetID uuid.UUID) (*model.MailConnection, error)
	// SetPaused pauses or resumes a connection, leaving its status alone.
	SetPaused(ctx context.Context, id uuid.UUID, paused bool) (*model.MailConnection, error)
	// SaveWatch records a new watch: its cursor and expiry.
	SaveWatch(ctx context.Context, id uuid.UUID, cursor string, expiresAt time.Time) error
	// SaveSync records a successful fetch: the new cursor, sync time now,
	// status active and no error.
	SaveSync(ctx context.Context, id uuid.UUID, cursor string) error
	// SaveFailure records a failed sync without moving the cursor.
	SaveFailure(ctx context.Context, id uuid.UUID, status model.MailConnectionStatus, lastError string) error
}
