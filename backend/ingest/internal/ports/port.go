package ports

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// Provider names. A mail provider's name is also Connection.Provider and the
// key it is built under (MAIL_PROVIDERS).
const (
	ProviderGmail   = "gmail"
	ProviderAPI     = "api"     // posted to POST /api/ingest/messages
	ProviderAndroid = "android" // SMS forwarded by the Android app
)

// InboundMessage is a message from any source, before it is queued. Which
// budgets it goes to is up to the caller of Accept.
type InboundMessage struct {
	Source     string // model.SourceEmail, SourceSMS or SourceAPI
	Provider   string
	ExternalID string // Gmail message ID; derived from the content when empty
	Sender     string
	Subject    string
	Body       string
	ReceivedAt time.Time
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
}
