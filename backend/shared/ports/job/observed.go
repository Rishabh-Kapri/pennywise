package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// appendTimeout bounds one history write. It runs on a ctx detached from the
// caller's cancellation (see record), so it needs its own limit.
const appendTimeout = 5 * time.Second

// notifyTimeout is short because the worker waits for it, and a watcher missing
// an update only means a slightly stale screen.
const notifyTimeout = 500 * time.Millisecond

// ObservedBroker wraps any Broker and records each state change it makes as
// a JobEvent in a JobLog. It is itself a Broker, so callers can't tell it from
// the adapter it wraps. factory.ConnectJob wraps the broker in it when a job
// store is configured; broker adapters write no history themselves.
//
// The event is written after the broker call succeeds, as a separate write,
// so the history is best-effort: if the append fails, the state change stands
// and the failure is logged. Calls that change nothing (Heartbeat,
// UpdateProgress, a duplicate Enqueue, a failed call) record no event.
type ObservedBroker struct {
	broker Broker
	log    JobLog
	notify Notifier // nil: changes aren't announced
}

func NewObservedBroker(broker Broker, log JobLog) *ObservedBroker {
	return &ObservedBroker{broker: broker, log: log}
}

// WithNotifier announces each state change and progress update through n.
func (o *ObservedBroker) WithNotifier(n Notifier) *ObservedBroker {
	o.notify = n
	return o
}

func (o *ObservedBroker) Enqueue(
	ctx context.Context,
	jobType string,
	payload []byte,
	opts ...EnqueueOption,
) (EnqueueResult, error) {
	res, err := o.broker.Enqueue(ctx, jobType, payload, opts...)
	if err == nil && !res.Duplicate {
		o.record(ctx, JobEvent{
			JobID:   res.JobID,
			Type:    EventEnqueued,
			Status:  QueuedJobStatus,
			Attempt: 0,
			Actor:   enqueueActor(ctx),
		})
		if resolved, optsErr := ResolveEnqueueOptions(jobType, payload, opts...); optsErr == nil {
			o.announce(ctx, Update{JobID: res.JobID, BudgetID: resolved.BudgetID, Type: jobType, Status: QueuedJobStatus})
		}
	}
	return res, err
}

func (o *ObservedBroker) ClaimJob(ctx context.Context, queues []string, workerID string) (*Job, error) {
	j, err := o.broker.ClaimJob(ctx, queues, workerID)
	if err == nil && j != nil {
		o.record(ctx, JobEvent{
			JobID:   j.ID,
			Type:    EventClaimed,
			Status:  RunningJobStatus,
			Attempt: j.Attempt,
			Actor:   workerID,
		})
		o.announce(ctx, Update{JobID: j.ID, BudgetID: j.BudgetID, Type: j.Type, Status: RunningJobStatus})
	}
	return j, err
}

func (o *ObservedBroker) Heartbeat(ctx context.Context, claim Claim) error {
	return o.broker.Heartbeat(ctx, claim)
}

func (o *ObservedBroker) UpdateProgress(ctx context.Context, claim Claim, progress json.RawMessage) error {
	err := o.broker.UpdateProgress(ctx, claim, progress)
	if err == nil {
		o.announce(ctx, Update{
			JobID: claim.JobID, BudgetID: claim.BudgetID, Type: claim.Type, Status: RunningJobStatus, Progress: progress,
		})
	}
	return err
}

func (o *ObservedBroker) MarkCompleted(
	ctx context.Context,
	claim Claim,
	result json.RawMessage,
	duration time.Duration,
) error {
	err := o.broker.MarkCompleted(ctx, claim, result, duration)
	reason := fmt.Sprintf("completed by worker %s after %s", claim.WorkerID, duration)
	if err == nil {
		o.record(
			ctx, claimEvent(
				claim,
				EventCompleted,
				SucceededJobStatus,
				reason,
				new(duration.Milliseconds()),
			),
		)
		o.announceClaim(ctx, claim, SucceededJobStatus)
	}
	return err
}

func (o *ObservedBroker) MarkPending(
	ctx context.Context,
	job *Job,
	claim Claim,
	lastError string,
	runAt time.Time,
) error {
	err := o.broker.MarkPending(ctx, job, claim, lastError, runAt)
	reason := fmt.Sprintf(
		"attempt %d/%d failed, retrying at %s : %s",
		job.Attempt,
		job.MaxAttempts,
		runAt.UTC().Format(time.RFC3339),
		lastError,
	)
	if err == nil {
		e := claimEvent(claim, EventRetryScheduled, QueuedJobStatus, reason, nil)
		e.Error = lastError
		e.NextRunAt = &runAt
		o.record(ctx, e)
		o.announceClaim(ctx, claim, QueuedJobStatus)
		return nil
	}
	if errs.HasCode(err, errs.CodeJobClaimLost) {
		return nil
	}
	return err
}

func (o *ObservedBroker) MarkDead(ctx context.Context, claim Claim, lastError string, reason string) error {
	err := o.broker.MarkDead(ctx, claim, lastError, reason)
	if err == nil {
		e := claimEvent(claim, EventDead, DeadJobStatus, reason, nil)
		e.Error = lastError
		o.record(ctx, e)
		o.announceClaim(ctx, claim, DeadJobStatus)
	}
	return err
}

// RequeueStaleRunning records nothing: the port returns only a count, not
// which jobs were reaped, so there is no job to attach an event to.
func (o *ObservedBroker) RequeueStaleRunning(
	ctx context.Context,
	staleThreshold time.Duration,
	reaperID string,
) (int64, error) {
	return o.broker.RequeueStaleRunning(ctx, staleThreshold, reaperID)
}

// record appends e, logging (not returning) a failure. It keeps ctx's values
// (correlation ID) but not its cancellation: the state change already
// happened, so its history should be written even during shutdown.
func (o *ObservedBroker) record(ctx context.Context, e JobEvent) {
	appendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appendTimeout)
	defer cancel()
	if err := o.log.Append(appendCtx, e); err != nil {
		logger.Logger(ctx).Error("job history not recorded",
			"job_id", e.JobID, "event", e.Type, "attempt", e.Attempt, "err", err)
	}
}

func (o *ObservedBroker) announceClaim(ctx context.Context, c Claim, status JobStatus) {
	o.announce(ctx, Update{JobID: c.JobID, BudgetID: c.BudgetID, Type: c.Type, Status: status})
}

// announce tells watchers about a change, logging (not returning) a failure:
// the change already happened.
func (o *ObservedBroker) announce(ctx context.Context, u Update) {
	if o.notify == nil {
		return
	}
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notifyTimeout)
	defer cancel()
	if err := o.notify.Notify(notifyCtx, u); err != nil {
		logger.Logger(ctx).Warn("job update not announced", "job_id", u.JobID, "err", err)
	}
}

func claimEvent(c Claim, t JobEventType, s JobStatus, r string, durationMs *int64) JobEvent {
	return JobEvent{
		JobID: c.JobID, Type: t, Status: s, Attempt: c.Attempt, Actor: c.WorkerID,
		Reason: r, DurationMs: durationMs,
	}
}

// enqueueActor names who enqueued a job: the service in ctx, if any.
func enqueueActor(ctx context.Context) string {
	if name := utils.LocalServiceFromContext(ctx); name != "" {
		return name
	}
	return "unknown"
}
