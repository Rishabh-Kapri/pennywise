// Package service holds ingest's use cases. Handlers and adapters turn
// whatever arrived into a ports.InboundMessage; Accept queues it.
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/sanitize"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// Queued is the job created (or found) for one budget.
type Queued struct {
	BudgetID  uuid.UUID `json:"budgetId"`
	JobID     uuid.UUID `json:"jobId"`
	Duplicate bool      `json:"duplicate"` // already queued earlier
}

type IngestService interface {
	// Accept queues msg once for each budget in msg.BudgetID. It is safe to
	// call again with the same message: the job idempotency key
	// (budget, source, provider, external ID) returns the first job instead.
	// Email bodies are sanitized here, so every path into the pipeline gets
	// the same text. opts apply to each queued job.
	Accept(ctx context.Context, msg ports.InboundMessage, opts ...job.EnqueueOption) ([]Queued, error)
}

type ingestService struct {
	broker job.Broker // nil when no job queue is configured
}

func NewIngestService(broker job.Broker) IngestService {
	return &ingestService{broker: broker}
}

func (s *ingestService) Accept(
	ctx context.Context,
	msg ports.InboundMessage,
	opts ...job.EnqueueOption,
) ([]Queued, error) {
	if s.broker == nil {
		return nil, errs.New(errs.CodeIngestQueueUnavailable, "no job queue configured")
	}
	if msg.Source == ports.EmailSource {
		msg.Body = sanitize.Text(msg.Body)
	}
	if err := validate(msg); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(model.ReceivedMessage{
		Source:     string(msg.Source),
		Provider:   msg.Provider,
		ExternalID: msg.ExternalID,
		Sender:     msg.Sender,
		Subject:    msg.Subject,
		Body:       msg.Body,
		ReceivedAt: msg.ReceivedAt,
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "encode message", err)
	}

	queued := make([]Queued, 0, len(msg.BudgetID))
	for _, budgetID := range msg.BudgetID {
		res, err := s.broker.Enqueue(
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
			return nil, err
		}
		queued = append(queued, Queued{BudgetID: budgetID, JobID: res.JobID, Duplicate: res.Duplicate})
	}
	return queued, nil
}

// idempotencyKey is unique per budget and message. The jobs table enforces
// (queue, type, key) across all budgets, so the budget must be part of it.
func idempotencyKey(budgetID uuid.UUID, msg ports.InboundMessage) string {
	return strings.Join([]string{budgetID.String(), string(msg.Source), msg.Provider, msg.ExternalID}, ":")
}

func validate(msg ports.InboundMessage) error {
	var missing []string
	if msg.Source == "" {
		missing = append(missing, "source")
	}
	if msg.Provider == "" {
		missing = append(missing, "provider")
	}
	if msg.ExternalID == "" {
		missing = append(missing, "external id")
	}
	if strings.TrimSpace(msg.Body) == "" {
		missing = append(missing, "body")
	}
	if len(msg.BudgetID) == 0 {
		missing = append(missing, "budget")
	}
	if len(missing) > 0 {
		return errs.NewWith(errs.CodeIngestInvalidMessage, "message is missing "+strings.Join(missing, ", "))
	}
	return nil
}
