package service

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/google/uuid"
)

// JobUpdateForwarder sends every service's job changes to the websocket clients
// of the budgets they belong to.
type JobUpdateForwarder struct {
	Source      job.UpdateSource
	Websocket   WebsocketService
	Store       job.JobStore
	Connections repository.MailConnectionRepository
}

// Run forwards updates until ctx is cancelled.
func (f *JobUpdateForwarder) Run(ctx context.Context) {
	if err := f.Source.Listen(ctx, func(update job.Update) { f.forward(ctx, update) }); err != nil {
		logger.Logger(ctx).Error("job updates stopped", "err", err)
	}
}

func (f *JobUpdateForwarder) forward(ctx context.Context, update job.Update) {
	for _, budgetID := range f.budgetsOf(ctx, update) {
		if err := f.Websocket.SendNotification(ctx, budgetID, model.EventJobUpdate, update); err != nil {
			logger.Logger(ctx).Warn("job update not sent", "job_id", update.JobID, "err", err)
		}
	}
}

// budgetsOf finds who to tell. A system job has no budget of its own, but a
// mailbox job belongs to the budgets that mailbox feeds.
func (f *JobUpdateForwarder) budgetsOf(ctx context.Context, update job.Update) []uuid.UUID {
	if update.BudgetID != nil {
		return []uuid.UUID{*update.BudgetID}
	}
	if update.Type != model.JobTypeMailboxChanged || f.Store == nil || f.Connections == nil {
		return nil
	}
	j, err := f.Store.GetJobByID(ctx, update.JobID)
	if err != nil {
		return nil
	}
	var mailbox model.MailboxChanged
	if err := json.Unmarshal(j.Payload, &mailbox); err != nil {
		return nil
	}
	connection, err := f.Connections.GetByAccount(ctx, mailbox.Provider, mailbox.Account)
	if err != nil || connection == nil {
		return nil
	}
	return connection.BudgetIDs
}
