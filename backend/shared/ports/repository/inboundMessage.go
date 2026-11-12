package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// InboundMessageRepository holds where each bank message stands
// (inbound_messages). Every status change goes through RecordReport or
// Transition, which only make the moves model.InboundMessageStatus.CanMoveTo
// allows, and only from the status the caller saw.
type InboundMessageRepository interface {
	// RecordReport saves how a run ended. A message without a row gets one; a
	// row moves only from processing (a run it was waiting on). It reports
	// whether it wrote: false means the row wasn't waiting on a run, or this
	// report is already saved. Status comes from report.Outcome.StatusAfter,
	// and the report's review replaces the row's.
	RecordReport(ctx context.Context, report model.InboundMessageReport) (bool, error)
	// Get returns a message, or ErrNotFound.
	Get(ctx context.Context, key model.InboundMessageKey) (*model.InboundMessage, error)
	// Transition moves a message from `from` to `to` and reports whether it
	// moved: false means it's no longer in `from`. Callers check CanMoveTo
	// first. The review stays, so processing can go back to needs_review when
	// a continue fails to queue its job; any other move into needs_review is a
	// report's (RecordReport).
	Transition(ctx context.Context, key model.InboundMessageKey, from, to model.InboundMessageStatus) (bool, error)
	// List returns the budget's messages in a status, newest change first.
	List(ctx context.Context, budgetID uuid.UUID, status model.InboundMessageStatus, limit int) ([]model.InboundMessage, error)
	// CountByStatus counts the budget's messages in each status. Statuses with
	// none are missing from the map.
	CountByStatus(ctx context.Context, budgetID uuid.UUID) (map[model.InboundMessageStatus]int, error)
}
