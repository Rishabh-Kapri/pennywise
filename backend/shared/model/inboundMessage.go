package model

import (
	"time"

	"github.com/google/uuid"
)

// InboundMessageStatus is where a bank message stands (inbound_messages.status).
type InboundMessageStatus string

const (
	// InboundMessageProcessing: a person sent the message on (continue, run
	// again) and the pipeline hasn't reported back yet.
	InboundMessageProcessing InboundMessageStatus = "processing"
	// InboundMessageNeedsReview: a step wasn't sure and parked it for a person.
	InboundMessageNeedsReview InboundMessageStatus = "needs_review"
	// InboundMessageDone: its run ended (a transaction, a duplicate, or not one).
	InboundMessageDone InboundMessageStatus = "done"
	// InboundMessageDismissed: a person put it away. Final.
	InboundMessageDismissed InboundMessageStatus = "dismissed"
)

// inboundMessageMoves is the message state machine: the statuses each status
// may move to. A message's first report creates its row as needs_review or
// done, so there is no "new" status.
//
//	processing ──report──► needs_review ──continue──► processing
//	processing ──report──► done ──run again──► processing
//	anything but dismissed ──dismiss──► dismissed
var inboundMessageMoves = map[InboundMessageStatus][]InboundMessageStatus{
	InboundMessageProcessing:  {InboundMessageNeedsReview, InboundMessageDone, InboundMessageDismissed},
	InboundMessageNeedsReview: {InboundMessageProcessing, InboundMessageDismissed},
	InboundMessageDone:        {InboundMessageProcessing, InboundMessageDismissed},
}

// CanMoveTo reports whether a message in status s may move to next. Staying in
// the same status is not a move.
func (s InboundMessageStatus) CanMoveTo(next InboundMessageStatus) bool {
	for _, allowed := range inboundMessageMoves[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// Valid reports whether s is a known status.
func (s InboundMessageStatus) Valid() bool {
	switch s {
	case InboundMessageProcessing, InboundMessageNeedsReview, InboundMessageDone, InboundMessageDismissed:
		return true
	}
	return false
}

// StatusAfter is the status a run that ended with this outcome leaves the
// message in.
func (o MessageOutcome) StatusAfter() InboundMessageStatus {
	if o == MessageOutcomeNeedsReview {
		return InboundMessageNeedsReview
	}
	return InboundMessageDone
}

// InboundMessageKey identifies one bank message in a budget: a
// ReceivedMessage without its RunID, so continuing or re-running it keeps it
// the same message.
type InboundMessageKey struct {
	BudgetID   uuid.UUID
	Source     string
	Provider   string
	ExternalID string
}

// Key is the message's identity in the budget.
func (m ReceivedMessage) Key(budgetID uuid.UUID) InboundMessageKey {
	return InboundMessageKey{BudgetID: budgetID, Source: m.Source, Provider: m.Provider, ExternalID: m.ExternalID}
}

// InboundMessage is a row of inbound_messages.
type InboundMessage struct {
	ID             uuid.UUID            `json:"id"`
	BudgetID       uuid.UUID            `json:"budgetId"`
	Source         string               `json:"source"`
	Provider       string               `json:"provider"`
	ExternalID     string               `json:"externalId"`
	Status         InboundMessageStatus `json:"status"`
	Outcome        MessageOutcome       `json:"outcome"`
	Reason         string               `json:"reason,omitempty"`
	Review         *MessageReview       `json:"review,omitempty"`
	TransactionIDs []uuid.UUID          `json:"transactionIds"`
	ReportJobID    uuid.UUID            `json:"reportJobId"`
	ReportedAt     time.Time            `json:"reportedAt"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

// Key is the row's message identity.
func (m InboundMessage) Key() InboundMessageKey {
	return InboundMessageKey{BudgetID: m.BudgetID, Source: m.Source, Provider: m.Provider, ExternalID: m.ExternalID}
}

// InboundMessageReport is how a run ended, as the pipeline.report job tells it.
type InboundMessageReport struct {
	Key            InboundMessageKey
	Outcome        MessageOutcome
	Reason         string
	Review         *MessageReview
	TransactionIDs []uuid.UUID
	ReportJobID    uuid.UUID
	ReportedAt     time.Time
}
