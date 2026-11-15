// Package memory holds in-memory adapters: no database, everything lives in
// a Go map inside the process. They are meant for tests and for learning how
// a port behaves; all data is lost when the process exits.
package memory

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
)

// jobBroker is the in-memory job.Broker.
//
// Think of it as the Postgres `jobs` table, but as a Go map:
//
//	jobs      ≈ the rows of the table, by ID
//	keys      ≈ the UNIQUE (queue, type, idempotency_key) index
//	mu        ≈ Postgres row locks: only one goroutine changes the map at a time
type jobBroker struct {
	mu         sync.Mutex
	jobs       map[uuid.UUID]*job.Job
	keys       map[idempotencyKey]uuid.UUID
	ticks      map[scheduleTick]uuid.UUID // ≈ the UNIQUE (schedule_id, scheduled_for) index
	heartbeats map[uuid.UUID]time.Time    // ≈ the heartbeat_at column
}

// scheduleTick identifies one firing of a schedule.
type scheduleTick struct {
	schedule uuid.UUID
	at       int64 // UnixNano: equal instants compare equal whatever their location
}

// idempotencyKey is the map key for duplicate checks. The key is unique per
// job type, so "email.extract"+"m1" and "email.predict"+"m1" are different.
type idempotencyKey struct {
	queue   string
	jobType string
	key     string
}

func NewMemoryJobBrokerAdapter() job.Broker {
	return &jobBroker{
		jobs:       map[uuid.UUID]*job.Job{},
		keys:       map[idempotencyKey]uuid.UUID{},
		ticks:      map[scheduleTick]uuid.UUID{},
		heartbeats: map[uuid.UUID]time.Time{},
	}
}

// Enqueue implements [job.Broker].
//
// Postgres equivalent:
//
//	INSERT INTO jobs (...) VALUES (...)
//	ON CONFLICT (queue, type, idempotency_key) DO NOTHING
func (b *jobBroker) Enqueue(
	ctx context.Context,
	jobType string,
	payload []byte,
	opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	// 1. Turn the options into one struct with defaults filled in
	//    (MaxAttempts 3, RunAt now) and check it is valid.
	o, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 2. Duplicate check. If this (queue, type, key) was enqueued before, don't make
	//    a second job: hand back the first one's ID. An empty key means
	//    "no dedupe", so every call creates a new job.
	if o.IdempotencyKey != "" {
		k := idempotencyKey{queue: o.Queue, jobType: o.Type, key: o.IdempotencyKey}
		if existingID, ok := b.keys[k]; ok {
			return job.EnqueueResult{JobID: existingID, Duplicate: true}, nil
		}
	}

	if o.ScheduleID != nil && o.ScheduledFor != nil {
		if existingID, ok := b.ticks[scheduleTick{*o.ScheduleID, o.ScheduledFor.UnixNano()}]; ok {
			return job.EnqueueResult{JobID: existingID, Duplicate: true}, nil
		}
	}

	// 3. "Insert the row".
	now := time.Now()
	j := &job.Job{
		ID:             uuid.New(),
		BudgetID:       o.BudgetID,
		Type:           o.Type,
		Queue:          o.Queue,
		IdempotencyKey: o.IdempotencyKey,
		Payload:        o.Payload,
		Status:         job.QueuedJobStatus,
		Priority:       o.Priority,
		MaxAttempts:    o.MaxAttempts,
		Attempt:        0, // nobody has tried it yet
		Metadata:       job.MetadataFromContext(ctx),
		RootJobID:      o.RootJobID,
		ParentID:       o.ParentID,
		ScheduleID:     o.ScheduleID,
		ScheduledFor:   o.ScheduledFor,
		RunAt:          o.RunAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	b.jobs[j.ID] = j
	if o.IdempotencyKey != "" {
		b.keys[idempotencyKey{queue: o.Queue, jobType: o.Type, key: o.IdempotencyKey}] = j.ID
	}
	if o.ScheduleID != nil && o.ScheduledFor != nil {
		b.ticks[scheduleTick{*o.ScheduleID, o.ScheduledFor.UnixNano()}] = j.ID
	}

	return job.EnqueueResult{JobID: j.ID, Duplicate: false}, nil
}

// ClaimJob implements [job.Broker].
//
// It looks once and never waits: with nothing ready it returns an error with
// code CodeJobsUnavailable. Waiting and polling are the runner's job.
//
// Postgres equivalent:
//
//	UPDATE jobs SET status = 'running', attempts = attempts + 1,
//	       locked_by = $worker, locked_at = now(), started_at = now()
//	WHERE id = (
//	    SELECT id FROM jobs
//	    WHERE queue = ANY($queues) AND status = 'queued' AND run_at <= now()
//	    ORDER BY priority DESC, run_at ASC
//	    LIMIT 1
//	    FOR UPDATE SKIP LOCKED
//	)
//	RETURNING *
func (b *jobBroker) ClaimJob(ctx context.Context, queues []string, workerID string) (*job.Job, error) {
	// The lock is what stops two slots from claiming the same job: while one
	// slot is inside here, every other slot waits at this line. In Postgres,
	// FOR UPDATE SKIP LOCKED does the same job without making them wait.
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	// 1. Find the best job this worker is allowed to run.
	var best *job.Job
	for _, j := range b.jobs {
		if !slices.Contains(queues, j.Queue) { // another worker's queue
			continue
		}
		if j.Status != job.QueuedJobStatus { // already running, done or dead
			continue
		}
		if j.RunAt.After(now) { // a delayed job or a retry that isn't due yet
			continue
		}
		if best == nil || runsBefore(j, best) {
			best = j
		}
	}
	if best == nil {
		return nil, errs.New(errs.CodeJobsUnavailable, "no job ready to run")
	}

	// 2. Take it: mark it running, owned by this worker, one more attempt used.
	best.Status = job.RunningJobStatus
	best.Attempt++
	lockedBy := workerID
	best.LockedBy = &lockedBy
	best.LockedAt = &now
	if best.StartedAt == nil {
		best.StartedAt = &now
	}
	best.UpdatedAt = now
	b.heartbeats[best.ID] = now

	// 3. Return a copy, not the pointer stored in the map. If we returned
	//    `best`, the handler could change the job (say, set Status) without
	//    going through the broker, which a real database would never allow.
	claimed := *best
	return &claimed, nil
}

// claimedJob returns the stored job while claim still owns it, or a
// CodeJobClaimLost error if it doesn't. The caller must hold b.mu.
func (b *jobBroker) claimedJob(claim job.Claim, action string) (*job.Job, error) {
	j, ok := b.jobs[claim.JobID]
	if !ok || j.Status != job.RunningJobStatus || j.Attempt != claim.Attempt ||
		j.LockedBy == nil || *j.LockedBy != claim.WorkerID {
		return nil, errs.NewWith(errs.CodeJobClaimLost, action+": job is no longer owned by this worker",
			errs.WithDetails(map[string]any{
				"job_id":    claim.JobID,
				"worker_id": claim.WorkerID,
				"attempt":   claim.Attempt,
			}))
	}
	return j, nil
}

// runsBefore reports whether a should be claimed before b:
// higher priority first, then whichever was due earliest.
func runsBefore(a, b *job.Job) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	return a.RunAt.Before(b.RunAt)
}

// Heartbeat implements [job.Broker].
func (b *jobBroker) Heartbeat(ctx context.Context, claim job.Claim) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	j, err := b.claimedJob(claim, "heartbeat")
	if err != nil {
		return err
	}
	b.touch(j)
	return nil
}

// UpdateProgress implements [job.Broker].
func (b *jobBroker) UpdateProgress(ctx context.Context, claim job.Claim, progress json.RawMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	j, err := b.claimedJob(claim, "update progress")
	if err != nil {
		return err
	}
	j.Progress = slices.Clone(progress)
	b.touch(j)
	return nil
}

// touch records a sign of life from the job's worker. The caller must hold b.mu.
func (b *jobBroker) touch(j *job.Job) {
	now := time.Now()
	b.heartbeats[j.ID] = now
	j.UpdatedAt = now
}

// MarkCompleted implements [job.Broker]. The attempt's duration is recorded in
// the job history by the ObservedBroker; the job itself doesn't store it.
//
// Postgres equivalent:
//
//	UPDATE jobs SET status = 'succeeded', result = $4, completed_at = now()
//	WHERE id = $1 AND locked_by = $2 AND attempt = $3 AND status = 'running'
func (b *jobBroker) MarkCompleted(ctx context.Context, claim job.Claim, result json.RawMessage, _ time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	j, err := b.claimedJob(claim, "mark completed")
	if err != nil {
		return err
	}

	now := time.Now()
	j.Status = job.SucceededJobStatus
	j.Result = slices.Clone(result) // keep our own copy, like ClaimJob's return
	j.CompletedAt = &now
	j.UpdatedAt = now
	return nil
}

// MarkPending implements [job.Broker]: the attempt failed and the job runs
// again at runAt.
func (b *jobBroker) MarkPending(ctx context.Context, _ *job.Job, claim job.Claim, lastError string, runAt time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	j, err := b.claimedJob(claim, "mark pending")
	if err != nil {
		return err
	}
	j.Status = job.QueuedJobStatus
	j.LastError = lastError
	j.RunAt = runAt
	j.UpdatedAt = time.Now()
	return nil
}

// MarkDead implements [job.Broker]. The reason goes to the job history
// (ObservedBroker); the job keeps lastError.
func (b *jobBroker) MarkDead(ctx context.Context, claim job.Claim, lastError string, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	j, err := b.claimedJob(claim, "mark dead")
	if err != nil {
		return err
	}
	now := time.Now()
	j.Status = job.DeadJobStatus
	j.LastError = lastError
	j.CompletedAt = &now
	j.UpdatedAt = now
	return nil
}

// RequeueStaleRunning implements [job.Broker]: running jobs with no sign of
// life for staleThreshold go back to queued, or to dead when no attempts are
// left.
func (b *jobBroker) RequeueStaleRunning(
	ctx context.Context,
	staleThreshold time.Duration,
	reaperID string,
) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	var requeued int64
	for _, j := range b.jobs {
		if j.Status != job.RunningJobStatus {
			continue
		}
		lastSeen := b.heartbeats[j.ID]
		if lastSeen.IsZero() && j.LockedAt != nil {
			lastSeen = *j.LockedAt
		}
		if now.Sub(lastSeen) <= staleThreshold {
			continue
		}
		owner := "unknown worker"
		if j.LockedBy != nil {
			owner = *j.LockedBy
		}
		j.LastError = "no heartbeat from " + owner
		j.RunAt = now
		j.UpdatedAt = now
		if j.Attempt >= j.MaxAttempts {
			j.Status = job.DeadJobStatus
			j.CompletedAt = &now
		} else {
			j.Status = job.QueuedJobStatus
		}
		requeued++
	}
	return requeued, nil
}
