package model

import (
	"strings"
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
	// RunID is set when a person sends a finished message through the pipeline
	// again. It makes that run's jobs new ones instead of duplicates.
	RunID string `json:"runId,omitempty"`
	// MailAccount is the mailbox an email came from, so its attachments can
	// be fetched later. Empty for SMS and API messages.
	MailAccount string `json:"mailAccount,omitempty"`
	// Attachments lists an email's files without their bytes.
	Attachments []MessageAttachment `json:"attachments,omitempty"`
}

// MessageAttachment is one file attached to an email. ID is the provider's
// handle for downloading it.
type MessageAttachment struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// IsPDF reports whether the attachment is a PDF, by type or file name.
func (a MessageAttachment) IsPDF() bool {
	return strings.EqualFold(a.MimeType, "application/pdf") || strings.HasSuffix(strings.ToLower(a.FileName), ".pdf")
}
