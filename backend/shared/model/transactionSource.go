package model

import (
	"time"

	"github.com/google/uuid"
)

// TransactionSource is one message or statement line that reported a
// transaction, as GET /transactions/:id/sources returns it. Fields the source
// didn't state are nil. OccurredAt is set only when the source gave a time of
// day; OccurredOn is the date it gave.
type TransactionSource struct {
	ID          uuid.UUID  `json:"id"`
	Kind        string     `json:"kind"`
	Origin      string     `json:"origin"`
	RawText     string     `json:"rawText"`
	Channel     *string    `json:"channel,omitempty"`
	Merchant    *string    `json:"merchant,omitempty"`
	VPA         *string    `json:"vpa,omitempty"`
	Ref         *string    `json:"ref,omitempty"`
	AccountHint *string    `json:"accountHint,omitempty"`
	Direction   *string    `json:"direction,omitempty"`
	Amount      *float64   `json:"amount,omitempty"`
	Currency    *string    `json:"currency,omitempty"`
	OccurredOn  *string    `json:"occurredOn,omitempty"` // YYYY-MM-DD
	OccurredAt  *time.Time `json:"occurredAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// StatementLineSource is a statement row recorded as the source of the
// transaction it matched.
type StatementLineSource struct {
	TransactionID uuid.UUID
	RawText       string
	Channel       string
	Merchant      string
	VPA           string
	Ref           string
	AccountHint   string
	Direction     string // DEBIT or CREDIT
	Amount        float64
	Currency      string
	OccurredOn    string // YYYY-MM-DD
	RewardsPoints *int
}
