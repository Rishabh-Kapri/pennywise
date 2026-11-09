// Package worker runs go-pennywise-api's jobs. Like internal/handler (HTTP) it
// is an inbound adapter: it turns a claimed job into a service call. These are
// the last steps of the message pipeline; the rest run in cipher.
package worker

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// Deps are what the handlers call.
type Deps struct {
	Queues   []string
	Broker   job.Broker
	Messages service.MessageJobService
}

// Setup builds the worker on the broker main already opened, registers its
// handlers and runs it until ctx is cancelled and in-flight jobs finish. With
// no broker (JOB_DRIVER unset) nothing starts and it returns nil.
func Setup(ctx context.Context, deps Deps) error {
	if deps.Broker == nil {
		logger.Logger(ctx).Info("job worker off (JOB_DRIVER not set)")
		return nil
	}

	w, err := jobrunner.NewWorker(
		deps.Broker,
		jobrunner.WithConcurrency(5),
		jobrunner.WithQueues(deps.Queues),
		jobrunner.WithPrefix("pennywise"),
	)
	if err != nil {
		return errs.Wrap(errs.CodeWorkerCreateError, "create job worker failed", err)
	}

	w.RegisterJobHandler(model.JobTypeTransactionCreate, transactionCreate(deps))
	w.RegisterJobHandler(model.JobTypePipelineReport, pipelineReport(deps))
	w.RegisterJobHandler(model.JobTypeTransactionLearn, transactionLearn(deps))

	if err := w.Run(ctx); err != nil {
		return errs.Wrap(errs.CodeWorkerRunError, "worker run failed", err)
	}
	return nil
}

// transactionCreate handles model.JobTypeTransactionCreate, the step after
// cipher has classified a message. It creates the transaction, then queues
// pipeline.report with the transaction_created outcome, or transaction_duplicate
// if the transaction was already there before this job started.
//
// Errors: CodeJobInvalidPayload (no budget, or the payload doesn't decode),
// whatever MessageJobService.CreateTransaction returns, and
// CodeJobEnqueueFailed. The runner retries them all up to MaxAttempts. That is
// safe after the transaction was created: the dedupe hash stops a second one,
// and the report keeps its idempotency key.
func transactionCreate(deps Deps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.TransactionCreatePayload](j)
		if err != nil {
			return nil, err
		}

		created, err := deps.Messages.CreateTransaction(ctx, p)
		if err != nil {
			return nil, err
		}

		outcome, learn := model.MessageOutcomeTransactionCreated, created.Learn
		if !created.Created && created.ExistingSince.Before(j.CreatedAt) {
			created.Duplicate = true
			outcome, learn = model.MessageOutcomeTransactionDuplicate, nil
			logger.Logger(ctx).Info("message matched an existing transaction", "transactionIds", created.TransactionIDs)
		}

		_, err = jobrunner.EnqueueNext(ctx, deps.Broker, j, model.JobTypePipelineReport, model.QueuePennywise,
			p.Message.IdempotencyKey(budgetID), model.PipelineReportPayload{
				Message:        p.Message,
				Outcome:        outcome,
				TransactionIDs: created.TransactionIDs,
				Learn:          learn,
			})
		if err != nil {
			return nil, err
		}
		return jobrunner.EncodeResult(created)
	}
}

// pipelineReport handles model.JobTypePipelineReport, the last job of every
// message whichever way it ended. It records the outcome in the pipeline run
// history, for the Activity page. After a created transaction it queues
// transaction.learn; otherwise nothing follows it.
//
// Errors: CodeJobInvalidPayload (no budget, or the payload doesn't decode) and
// whatever MessageJobService.ReportPipeline returns. A failure here loses only
// history: the transaction, if any, already exists.
func pipelineReport(deps Deps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.PipelineReportPayload](j)
		if err != nil {
			return nil, err
		}
		if err := deps.Messages.ReportPipeline(ctx, p); err != nil {
			return nil, err
		}
		if p.Learn != nil {
			_, err = jobrunner.EnqueueNext(ctx, deps.Broker, j, model.JobTypeTransactionLearn, model.QueuePennywise,
				p.Message.IdempotencyKey(budgetID), *p.Learn)
		}
		return nil, err
	}
}

// transactionLearn handles model.JobTypeTransactionLearn: it runs after the
// pipeline creates a transaction, and on its own when the user corrects or
// approves one. Nothing follows it.
//
// Errors: CodeJobInvalidPayload (no budget, or the payload doesn't decode) and
// CodeTransactionLearnFailed. The runner retries them; repeating is safe.
func transactionLearn(deps Deps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, _, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.TransactionLearnPayload](j)
		if err != nil {
			return nil, err
		}
		return nil, deps.Messages.LearnTransaction(ctx, p)
	}
}
