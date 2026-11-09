package model

import (
	"time"

	"github.com/google/uuid"
)

// Job types and queues are in jobs.go.

// MailboxChanged is the JSON payload of a JobTypeMailboxChanged job.
// MailboxWatch is the payload of JobTypeMailboxStartWatch: renew this
// connection's provider watch.
type MailboxWatch struct {
	ConnectionID uuid.UUID `json:"connectionId"`
}

type MailboxChanged struct {
	Provider string `json:"provider"` // gmail
	Account  string `json:"account"`  // the email address
	// Cursor is the provider's position in the notification (Gmail history
	// ID). Informational: fetching starts from the connection's own cursor,
	// so out-of-order pushes don't matter.
	Cursor string `json:"cursor"`
}

// MailboxBackfill is the JSON payload of a JobTypeMailboxBackfill job.
type MailboxBackfill struct {
	ConnectionID uuid.UUID `json:"connectionId"`
	Account      string    `json:"account"` // the email address
	ExternalIDs  []string  `json:"externalIds"`
}

// ReceivedMessage is the JSON payload of a JobTypeMessageReceived job: one
// message that might be a bank transaction, in the same shape whatever it
// came from.
type ReceivedMessage struct {
	Source     string    `json:"source"`   // email | sms | api
	Provider   string    `json:"provider"` // gmail, android, api, ...
	ExternalID string    `json:"externalId"`
	Sender     string    `json:"sender"`
	Subject    string    `json:"subject,omitempty"`
	Body       string    `json:"body"`
	ReceivedAt time.Time `json:"receivedAt"`
}
