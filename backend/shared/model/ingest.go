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

// MailboxWatchScheduleName is the job_schedules name of the connection's daily
// watch renewal.
func MailboxWatchScheduleName(connectionID uuid.UUID) string {
	return "mailbox.watch:" + connectionID.String()
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

// Where a ReceivedMessage came from.
const (
	MessageSourceEmail = "email"
	MessageSourceSMS   = "sms"
	MessageSourceAPI   = "api"
)

// ReceivedMessage is the JSON payload of a JobTypeMessageReceived job: one
// message that might be a bank transaction, in the same shape whatever it
// came from.
type ReceivedMessage struct {
	Source     string    `json:"source"`   // MessageSourceEmail, MessageSourceSMS or MessageSourceAPI
	Provider   string    `json:"provider"` // gmail, android, api, ...
	ExternalID string    `json:"externalId"`
	Sender     string    `json:"sender"`
	Subject    string    `json:"subject,omitempty"`
	Body       string    `json:"body"`
	ReceivedAt time.Time `json:"receivedAt"`
	// RunID is set when a person sends a finished message through the pipeline
	// again. It makes that run's jobs new ones instead of duplicates.
	RunID string `json:"runId,omitempty"`
}
