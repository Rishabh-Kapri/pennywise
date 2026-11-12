// Package service holds ingest's use cases. Handlers and adapters turn
// whatever arrived into a ports.InboundMessage; Accept queues it.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/sanitize"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// Enqueuer is the part of the job queue the services use.
type Enqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...job.EnqueueOption) (job.EnqueueResult, error)
}

// NewEnqueuer returns broker, or with no job queue (nil) one that fails
// every call with CodeIngestQueueUnavailable.
func NewEnqueuer(broker job.Broker) Enqueuer {
	if broker == nil {
		return noQueue{}
	}
	return broker
}

type noQueue struct{}

func (noQueue) Enqueue(context.Context, string, []byte, ...job.EnqueueOption) (job.EnqueueResult, error) {
	return job.EnqueueResult{}, errs.New(errs.CodeIngestQueueUnavailable, "no job queue configured")
}

// Queued is the job created (or found) for one budget.
type Queued struct {
	BudgetID  uuid.UUID `json:"budgetId"`
	JobID     uuid.UUID `json:"jobId"`
	Duplicate bool      `json:"duplicate"` // already queued earlier
}

// Accepted is what Accept queued for one message.
type Accepted struct {
	// ExternalID is the message's, or the one derived from its content.
	ExternalID string   `json:"externalId"`
	Jobs       []Queued `json:"jobs"`
}

type IngestService interface {
	// Accept queues msg once for each of budgets. It is safe to call again
	// with the same message: the job idempotency key (budget, source,
	// provider, external ID) returns the first job instead. A message without
	// an ExternalID gets one derived from its sender, subject, body and
	// ReceivedAt. Email bodies are sanitized here, so every path into the
	// pipeline gets the same text. opts apply to each queued job.
	Accept(ctx context.Context, budgets []uuid.UUID, msg ports.InboundMessage, opts ...job.EnqueueOption) (Accepted, error)
}

type ingestService struct {
	queue Enqueuer
}

func NewIngestService(queue Enqueuer) IngestService {
	return &ingestService{queue: queue}
}

func (s *ingestService) Accept(
	ctx context.Context,
	budgets []uuid.UUID,
	msg ports.InboundMessage,
	opts ...job.EnqueueOption,
) (Accepted, error) {
	if msg.ExternalID == "" && !msg.ReceivedAt.IsZero() {
		msg.ExternalID = contentID(msg)
	}
	if msg.Source == model.SourceEmail {
		msg.Body = sanitize.Text(msg.Body)
	}
	if err := validate(budgets, msg); err != nil {
		return Accepted{}, err
	}

	payload, err := json.Marshal(model.ReceivedMessage{
		Source:     msg.Source,
		Provider:   msg.Provider,
		ExternalID: msg.ExternalID,
		Sender:     msg.Sender,
		Subject:    msg.Subject,
		Body:       msg.Body,
		ReceivedAt: msg.ReceivedAt,
	})
	if err != nil {
		return Accepted{}, errs.Wrap(errs.CodeInternalError, "encode message", err)
	}

	accepted := Accepted{ExternalID: msg.ExternalID, Jobs: make([]Queued, 0, len(budgets))}
	for _, budgetID := range budgets {
		res, err := s.queue.Enqueue(
			ctx, model.JobTypeMessageReceived, payload,
			append([]job.EnqueueOption{
				job.WithQueue(model.QueueCipher),
				job.WithBudgetID(&budgetID),
				job.WithIdempotencyKey(idempotencyKey(budgetID, msg)),
			}, opts...)...,
		)
		if err != nil {
			// Budgets queued before this one stay queued; a retry of the
			// whole message dedupes them and queues the rest.
			return Accepted{}, err
		}
		accepted.Jobs = append(accepted.Jobs, Queued{BudgetID: budgetID, JobID: res.JobID, Duplicate: res.Duplicate})
	}
	return accepted, nil
}

// idempotencyKey is unique per budget and message. The jobs table enforces
// (queue, type, key) across all budgets, so the budget must be part of it.
func idempotencyKey(budgetID uuid.UUID, msg ports.InboundMessage) string {
	return strings.Join([]string{budgetID.String(), msg.Source, msg.Provider, msg.ExternalID}, ":")
}

// contentID derives a stable ID: the same message sent twice gets the same
// ID, while two identical alerts at different times don't collide. How the
// message arrived stays out of it, so an SMS caught live and imported from the
// inbox later is one message. Changing it re-queues messages already queued.
func contentID(msg ports.InboundMessage) string {
	sum := sha256.Sum256([]byte(msg.Sender + "\x00" + msg.Subject + "\x00" + msg.Body + "\x00" +
		msg.ReceivedAt.UTC().Format(time.RFC3339Nano)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validate(budgets []uuid.UUID, msg ports.InboundMessage) error {
	var missing []string
	if msg.Source == "" {
		missing = append(missing, "source")
	}
	if msg.Provider == "" {
		missing = append(missing, "provider")
	}
	if msg.ExternalID == "" {
		missing = append(missing, "external id or received time")
	}
	if strings.TrimSpace(msg.Body) == "" {
		missing = append(missing, "body")
	}
	if len(budgets) == 0 {
		missing = append(missing, "budget")
	}
	if len(missing) > 0 {
		return errs.NewWith(errs.CodeIngestInvalidMessage, "message is missing "+strings.Join(missing, ", "))
	}
	return nil
}
