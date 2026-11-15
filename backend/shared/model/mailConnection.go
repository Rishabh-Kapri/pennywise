package model

import (
	"time"

	"github.com/google/uuid"
)

// MailConnection is one mailbox Pennywise may read (table mail_connections).
// It is read once and its messages go to every budget in BudgetIDs. Login identity is separate (google_provider_users).
type MailConnection struct {
	ID        uuid.UUID
	Provider  string // gmail
	Account   string // email address, lowercase
	BudgetIDs []uuid.UUID
	// Credentials are sealed by the provider adapter; opaque everywhere else.
	Credentials []byte
	// Cursor is the provider's position (Gmail history ID): fetch after it.
	// Empty until the first watch.
	Cursor      string
	WatchExpiry *time.Time // nil: not watched yet
	Status      MailConnectionStatus
	LastSyncAt  *time.Time
	LastError   string
	PausedAt    *time.Time // nil: syncing
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MailConnectionStatus string

const (
	// MailConnectionActive: syncing normally.
	MailConnectionActive MailConnectionStatus = "active"
	// MailConnectionNeedsReconnect: the provider refused the stored
	// credentials (MAIL_AUTH_FAILED); only the user reconnecting fixes it.
	MailConnectionNeedsReconnect MailConnectionStatus = "needs_reconnect"
	// MailConnectionError: the last sync failed for another reason; retried.
	MailConnectionError MailConnectionStatus = "error"
)
