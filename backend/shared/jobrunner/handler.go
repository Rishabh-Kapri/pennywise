package jobrunner

import (
	"context"
	"encoding/json"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// Helpers for job handlers.

// BudgetContext prepares ctx for a handler of a budget-scoped job: it restores
// the enqueuer's request metadata and sets the job's budget. A job with no
// budget returns CodeJobInvalidPayload.
func BudgetContext(ctx context.Context, j *job.Job) (context.Context, uuid.UUID, error) {
	ctx = j.Metadata.Context(ctx)
	if j.BudgetID == nil || *j.BudgetID == uuid.Nil {
		return ctx, uuid.Nil, errs.NewWith(errs.CodeJobInvalidPayload, j.Type+": job has no budget",
			errs.WithDetail("job_id", j.ID))
	}
	return utils.WithBudgetID(ctx, *j.BudgetID), *j.BudgetID, nil
}

// DecodePayload reads j's JSON payload into a T. A payload that doesn't decode
// returns CodeJobInvalidPayload.
func DecodePayload[T any](j *job.Job) (T, error) {
	var v T
	if err := json.Unmarshal(j.Payload, &v); err != nil {
		return v, errs.Wrap(errs.CodeJobInvalidPayload, j.Type+": decode payload", err,
			errs.WithDetail("job_id", j.ID))
	}
	return v, nil
}

// EncodeResult marshals a handler's result. A failure returns CodeInternalError.
func EncodeResult(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "encode job result", err)
	}
	return b, nil
}

// EnqueueNext queues the job that follows parent in a chain, keeping its budget
// and root. key should be the same on every attempt of parent, so a retried
// parent gets back the job it already queued. Failures return
// CodeJobEnqueueFailed; the caller returns that so the parent is retried.
func EnqueueNext(
	ctx context.Context,
	broker job.Broker,
	parent *job.Job,
	jobType, queue, key string,
	payload any,
) (uuid.UUID, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodeJobEnqueueFailed, jobType+": encode payload", err)
	}
	parentID := parent.ID
	rootID := parent.ID
	if parent.RootJobID != nil {
		rootID = *parent.RootJobID
	}
	res, err := broker.Enqueue(
		ctx, jobType, p,
		job.WithQueue(queue),
		job.WithBudgetID(parent.BudgetID),
		job.WithIdempotencyKey(key),
		job.WithRootJobID(&rootID),
		job.WithParentID(&parentID),
	)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodeJobEnqueueFailed, jobType+": enqueue", err)
	}
	return res.JobID, nil
}
