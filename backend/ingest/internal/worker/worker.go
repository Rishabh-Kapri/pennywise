// Package worker runs ingest's jobs. Like internal/handler (HTTP) and
// internal/pubsub (Gmail pushes) it is an inbound adapter: it turns a claimed
// job into calls on the ports and the outcome back into the job's result.
package worker

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// Deps are what the handlers call.
type Deps struct {
	Queues  []string
	Broker  job.Broker
	Mailbox service.MailboxService
}

// Setup builds ingest's worker on the broker main already opened, registers
// its handlers and starts it. The returned channel closes once the worker has
// stopped: after ctx is cancelled and in-flight jobs finish. With no broker
// (JOB_DRIVER unset) nothing starts and the channel is already closed.
func Setup(ctx context.Context, deps Deps) error {
	if deps.Broker == nil {
		logger.Logger(ctx).Info("job worker off (JOB_DRIVER not set)")
		return nil
	}

	w, err := jobrunner.NewWorker(
		deps.Broker,
		jobrunner.WithConcurrency(1),
		jobrunner.WithQueues(deps.Queues),
		jobrunner.WithPrefix("ingest"),
	)
	if err != nil {
		return errs.Wrap(errs.CodeWorkerCreateError, "create job worker failed", err)
	}
	w.RegisterJobHandler(model.JobTypeMailboxChanged, mailboxChanged(deps))
	w.RegisterJobHandler(model.JobTypeMailboxBackfill, mailboxBackfill(deps))
	w.RegisterJobHandler(model.JobTypeMailboxStartWatch, mailboxRenewWatch(deps))

	err = w.Run(ctx)
	logger.Logger(ctx).Info("run returned")
	if err != nil {
		return errs.Wrap(errs.CodeWorkerRunError, "worker run failed", err)
	}
	return nil
}

// childOf makes queued messages children of j, so they show under it on the
// Jobs screen.
func childOf(j *job.Job) []job.EnqueueOption {
	parentID := j.ID
	rootID := j.ID
	if j.RootJobID != nil {
		rootID = *j.RootJobID
	}
	return []job.EnqueueOption{job.WithRootJobID(&rootID), job.WithParentID(&parentID)}
}

// mailboxChanged handles model.JobTypeMailboxChanged: it syncs the mailbox, and
// each new message becomes a message.received child. A retry is safe: the
// cursor only moves once everything is queued, and idempotency keys drop what
// already was.
func mailboxChanged(deps Deps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		n, err := jobrunner.DecodePayload[model.MailboxChanged](j)
		if err != nil {
			return nil, err
		}
		result, err := deps.Mailbox.Sync(ctx, n, childOf(j)...)
		if err != nil {
			return nil, err
		}
		return jobrunner.EncodeResult(result)
	}
}

// mailboxRenewWatch handles model.JobTypeMailboxStartWatch, queued by the
// connection's schedule.
func mailboxRenewWatch(deps Deps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		p, err := jobrunner.DecodePayload[model.MailboxWatch](j)
		if err != nil {
			return nil, err
		}
		result, err := deps.Mailbox.RenewWatch(ctx, p.ConnectionID)
		if err != nil {
			return nil, err
		}
		return jobrunner.EncodeResult(result)
	}
}

// mailboxBackfill handles model.JobTypeMailboxBackfill: it fetches the emails
// the user chose, and each becomes a message.received child.
func mailboxBackfill(deps Deps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.MailboxBackfill](j)
		if err != nil {
			return nil, err
		}
		result, err := deps.Mailbox.Backfill(ctx, budgetID, p.ConnectionID, p.ExternalIDs, childOf(j)...)
		if err != nil {
			return nil, err
		}
		return jobrunner.EncodeResult(result)
	}
}
