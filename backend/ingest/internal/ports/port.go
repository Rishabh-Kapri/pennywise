package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type Source string

const (
	EmailSource Source = "email"
	SmsSource   Source = "sms"
	ApiSource   Source = "api"
)

type InboundMessage struct {
	Source     Source
	Provider   string
	BudgetID   []uuid.UUID
	ExternalID string // Gmail message ID, SMS hash
	Sender     string
	Subject    string
	Body       string
	ReceivedAt time.Time
	// MailAccount and Attachments are set for email: the mailbox it came
	// from and its files (names only), so a statement's PDF can be fetched.
	MailAccount string
	Attachments []model.MessageAttachment
}

// Query narrows a mailbox search.
type Query struct {
	Days       int   // only mail newer than this many days
	MaxResults int64 // at most this many messages
}

// ConnectedMailbox is a mailbox a user has just granted access to.
type ConnectedMailbox struct {
	Account     string // the address the user actually granted
	Credentials []byte // sealed by the adapter; store it as is
}

type MailProvider interface {
	// Connect turns the one-time code from the provider's consent screen into
	// credentials for the mailbox the user picked, which may not be the one
	// they log in with.
	Connect(ctx context.Context, code string) (ConnectedMailbox, error)
	// Watch starts (or renews) push notifications for the mailbox. It returns
	// the cursor to fetch from next and when the watch runs out; the caller
	// saves both on the connection.
	Watch(ctx context.Context, conn model.MailConnection) (cursor string, expiresAt time.Time, err error)
	// StopWatch ends push notifications, e.g. when the last budget is detached.
	StopWatch(ctx context.Context, conn model.MailConnection) error
	FetchSince(ctx context.Context, conn model.MailConnection) (messages []InboundMessage, cursor string, err error)
	// Search lists recent mail without bodies, for the user to choose from.
	Search(ctx context.Context, conn model.MailConnection, q Query) ([]InboundMessage, error)
	// Fetch loads messages in full. A message that's gone is left out.
	Fetch(ctx context.Context, conn model.MailConnection, ids []string) ([]InboundMessage, error)
	// Attachment downloads one file of a message. Look the message up again
	// first: a provider's attachment handle can change between fetches.
	Attachment(ctx context.Context, conn model.MailConnection, messageID, fileName string) ([]byte, error)
}
