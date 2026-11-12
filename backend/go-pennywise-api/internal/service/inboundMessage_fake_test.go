package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
)

// fakeMessages is an in-memory InboundMessageRepository that keeps the same
// rules as the Postgres one: a report moves only a processing row, and a
// transition only from the status the caller saw.
type fakeMessages struct {
	rows        map[model.InboundMessageKey]*model.InboundMessage
	transitions int
}

func newFakeMessages() *fakeMessages {
	return &fakeMessages{rows: map[model.InboundMessageKey]*model.InboundMessage{}}
}

func (f *fakeMessages) RecordReport(_ context.Context, r model.InboundMessageReport) (bool, error) {
	row, ok := f.rows[r.Key]
	if ok && (row.Status != model.InboundMessageProcessing || row.ReportJobID == r.ReportJobID) {
		return false, nil
	}
	f.rows[r.Key] = &model.InboundMessage{
		ID: uuid.New(), BudgetID: r.Key.BudgetID, Source: r.Key.Source, Provider: r.Key.Provider,
		ExternalID: r.Key.ExternalID, Status: r.Outcome.StatusAfter(), Outcome: r.Outcome, Reason: r.Reason,
		Review: r.Review, TransactionIDs: r.TransactionIDs, ReportJobID: r.ReportJobID, ReportedAt: r.ReportedAt,
	}
	return true, nil
}

func (f *fakeMessages) Get(_ context.Context, key model.InboundMessageKey) (*model.InboundMessage, error) {
	row, ok := f.rows[key]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copied := *row
	return &copied, nil
}

func (f *fakeMessages) Transition(_ context.Context, key model.InboundMessageKey, from, to model.InboundMessageStatus) (bool, error) {
	row, ok := f.rows[key]
	if !ok || row.Status != from {
		return false, nil
	}
	row.Status = to
	f.transitions++
	return true, nil
}

func (f *fakeMessages) List(_ context.Context, budgetID uuid.UUID, status model.InboundMessageStatus, limit int) ([]model.InboundMessage, error) {
	out := []model.InboundMessage{}
	for _, row := range f.rows {
		if row.BudgetID == budgetID && row.Status == status && len(out) < limit {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (f *fakeMessages) CountByStatus(_ context.Context, budgetID uuid.UUID) (map[model.InboundMessageStatus]int, error) {
	counts := map[model.InboundMessageStatus]int{}
	for _, row := range f.rows {
		if row.BudgetID == budgetID {
			counts[row.Status]++
		}
	}
	return counts, nil
}

func (f *fakeMessages) status(budgetID uuid.UUID) model.InboundMessageStatus {
	return f.rows[model.ReceivedMessage{Source: "email", Provider: "gmail", ExternalID: "m1"}.Key(budgetID)].Status
}
