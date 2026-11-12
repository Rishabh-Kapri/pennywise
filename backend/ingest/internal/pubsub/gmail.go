// Package pubsub receives Gmail push notifications from Google Cloud Pub/Sub.
// It is ingest's inbound side for email, like internal/handler is for HTTP:
// it turns a push into a mailbox.changed job and acks. Fetching the mail is
// the job's handler's work, not this package's.
//
// Moved from go-gmail/pkg/pubsub. Its in-memory dedupe maps and the
// deterministic Temporal workflow ID are replaced by the job idempotency key
// (gmail:account:historyId), which survives restarts.
package pubsub

import (
	"context"
	"encoding/json"
	"strconv"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const originService = "ingest-gmail-push"

// gmailPush is the data Gmail publishes for each mailbox change.
type gmailPush struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

type Config struct {
	ProjectID       string
	Subscription    string // SUB_NAME
	CredentialsJSON string // GOOGLE_APPLICATION_CREDENTIALS_JSON; empty uses the default credentials
}

type GmailSubscriber struct {
	cfg     Config
	mailbox service.MailboxService
}

func NewGmailSubscriber(cfg Config, mailbox service.MailboxService) *GmailSubscriber {
	return &GmailSubscriber{cfg: cfg, mailbox: mailbox}
}

// Run receives pushes until ctx is cancelled, then returns once in-flight
// messages are handled. It fails fast if the subscription doesn't exist.
func (s *GmailSubscriber) Run(ctx context.Context) error {
	var opts []option.ClientOption
	if s.cfg.CredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(s.cfg.CredentialsJSON)))
	}
	client, err := pubsub.NewClient(ctx, s.cfg.ProjectID, opts...)
	if err != nil {
		return errs.Wrap(errs.CodeMailNotConfigured, "create pubsub client", err)
	}
	defer client.Close()

	sub := client.Subscription(s.cfg.Subscription)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return errs.Wrap(errs.CodeMailRequestFailed, "check pubsub subscription", err)
	}
	if !ok {
		return errs.New(errs.CodeMailNotConfigured, "pubsub subscription %q does not exist", s.cfg.Subscription)
	}

	logger.Logger(ctx).Info("receiving gmail pushes", "subscription", s.cfg.Subscription)
	// Receive runs the callback concurrently; that's safe because handling
	// only enqueues, and duplicates collapse on the idempotency key.
	return sub.Receive(ctx, func(msgCtx context.Context, msg *pubsub.Message) {
		if s.handle(messageContext(msgCtx), msg.Data) {
			msg.Ack()
		} else {
			msg.Nack()
		}
	})
}

// messageContext gives each push its own correlation ID. msgCtx derives from
// Run's ctx, so it already carries the service identity and outbound token.
func messageContext(msgCtx context.Context) context.Context {
	ctx := utils.WithOriginService(msgCtx, originService)
	return utils.WithCorrelationID(ctx, utils.NewCorrelationID())
}

// handle processes one push and reports whether to ack it. Malformed pushes
// are acked: redelivering them can never succeed. Only a failure to enqueue
// is nacked, so Pub/Sub delivers the push again later.
func (s *GmailSubscriber) handle(ctx context.Context, data []byte) (ack bool) {
	log := logger.Logger(ctx)

	var push gmailPush
	if err := json.Unmarshal(data, &push); err != nil || push.EmailAddress == "" || push.HistoryID == 0 {
		log.Warn("dropping malformed gmail push", "err", err)
		return true
	}

	res, err := s.mailbox.Changed(ctx, model.MailboxChanged{
		Provider: ports.ProviderGmail,
		Account:  push.EmailAddress,
		Cursor:   strconv.FormatUint(push.HistoryID, 10),
	})
	if err != nil {
		log.Error("queue gmail push failed, will be redelivered",
			"account", push.EmailAddress, "history_id", push.HistoryID, "err", err)
		return false
	}
	log.Info("gmail push queued", "account", push.EmailAddress, "history_id", push.HistoryID,
		"job_id", res.JobID, "duplicate", res.Duplicate)
	return true
}
