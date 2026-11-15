package job

import (
	"context"
	"encoding/json"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/google/uuid"
)

type JobStatus string

const (
	QueuedJobStatus    JobStatus = "queued"
	RunningJobStatus   JobStatus = "running"
	CancelledJobStatus JobStatus = "cancelled"
	SucceededJobStatus JobStatus = "succeeded"
	DeadJobStatus      JobStatus = "dead"
)

type Job struct {
	ID             uuid.UUID
	BudgetID       *uuid.UUID // nil for system jobs
	Type           string     // what type of job this is, eg, email.process, learn.txn etc
	Queue          string     // the queue a worker claims it from (model.Queue*)
	IdempotencyKey string     // unique per (Type, IdempotencyKey)
	Payload        []byte     // the input
	Status         JobStatus
	Priority       int // higher runs first
	MaxAttempts    int
	Attempt        int
	// Metadata is the request metadata captured from the enqueuing ctx
	// (MetadataFromContext); the runner restores it with Metadata.Context so
	// a job and every job it spawns log under the same correlation ID.
	Metadata Metadata

	RootJobID    *uuid.UUID // the job that started it all, nil for root
	ParentID     *uuid.UUID // the previous step
	ScheduleID   *uuid.UUID // for a cron job, its schedule config.
	ScheduledFor *time.Time

	Progress json.RawMessage // updated while running
	Result   json.RawMessage // the final output fo the step

	RunAt       time.Time // when this job is scheduled at
	StartedAt   *time.Time
	CompletedAt *time.Time
	LockedAt    *time.Time // time at which the worked picked the job
	LockedBy    *string    // which worker picked the job
	LastError   string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type JobFilter struct {
	Status   JobStatus
	Type     string
	Retrying *bool
	Limit    int
	Offset   int
}

type EnqueueOptions struct {
	BudgetID *uuid.UUID
	Type     string
	// Queue the job goes to. Required: there is no default routing, the
	// producer always names it (WithQueue).
	Queue          string
	IdempotencyKey string
	Payload        []byte
	Priority       int
	MaxAttempts    int
	RunAt          time.Time
	RootJobID      *uuid.UUID
	ParentID       *uuid.UUID
	ScheduleID     *uuid.UUID
	ScheduledFor   *time.Time // when this job should be run at, only for scheduled jobs
}

type EnqueueOption func(*EnqueueOptions)

func WithBudgetID(id *uuid.UUID) EnqueueOption {
	return func(opts *EnqueueOptions) {
		opts.BudgetID = id
	}
}

// WithIdempotencyKey makes a second Enqueue with the same (queue, type, key)
// return the first job instead of creating another.
func WithIdempotencyKey(key string) EnqueueOption {
	return func(opts *EnqueueOptions) {
		opts.IdempotencyKey = key
	}
}

// WithQueue sets the queue the job goes to. Every Enqueue needs it. To send
// one job type to several queues, enqueue once per queue.
func WithQueue(q string) EnqueueOption {
	return func(opts *EnqueueOptions) {
		opts.Queue = q
	}
}

func WithRootJobID(id *uuid.UUID) EnqueueOption {
	return func(opts *EnqueueOptions) {
		opts.RootJobID = id
	}
}

func WithParentID(id *uuid.UUID) EnqueueOption {
	return func(opts *EnqueueOptions) {
		opts.ParentID = id
	}
}

// WithSchedule marks a job as one schedule's tick. A tick is enqueued at most
// once, even with two schedulers.
func WithSchedule(id uuid.UUID, at time.Time) EnqueueOption {
	return func(opts *EnqueueOptions) {
		opts.ScheduleID = &id
		opts.ScheduledFor = &at
	}
}

type EnqueueResult struct {
	JobID     uuid.UUID
	Duplicate bool // true = Idempotency key matched an existing job
}

// the data telling which worker owns the job
type Claim struct {
	JobID    uuid.UUID
	WorkerID string
	Attempt  int
	// Who to tell about changes (see Update). Ownership checks don't use them.
	BudgetID *uuid.UUID
	Type     string
}

// Progress is what a running job is doing now, stored in jobs.progress.
type Progress struct {
	Message string `json:"message,omitempty"`
	Current int    `json:"current,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// Update says a job changed. BudgetID is nil for a system job.
type Update struct {
	JobID    uuid.UUID       `json:"jobId"`
	BudgetID *uuid.UUID      `json:"budgetId,omitempty"`
	Type     string          `json:"type"`
	Status   JobStatus       `json:"status"`
	Progress json.RawMessage `json:"progress,omitempty"`
}

// Notifier announces job changes across services. Delivery is best effort, so
// watchers treat an update as a hint to look again, not as the job's state.
type Notifier interface {
	Notify(ctx context.Context, update Update) error
}

// UpdateSource delivers every service's job changes. Listen blocks until ctx
// is cancelled.
type UpdateSource interface {
	Listen(ctx context.Context, handle func(Update)) error
}

type Broker interface {
	// Create a job, return jobID.
	// Returns existing jobID on duplicate job
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...EnqueueOption) (EnqueueResult, error)
	// Claim a job: oldest first, highest priority
	// ClaimJob claims the next due job from any of queues. Jobs on other
	// queues are never returned, so a worker only sees work it has handlers for.
	ClaimJob(ctx context.Context, queues []string, workerID string) (*Job, error)
	Heartbeat(ctx context.Context, claim Claim) error
	UpdateProgress(ctx context.Context, claim Claim, progress json.RawMessage) error
	// Mark a job complete
	MarkCompleted(ctx context.Context, claim Claim, result json.RawMessage, duration time.Duration) error
	MarkPending(ctx context.Context, job *Job, claim Claim, lastError string, runAt time.Time) error
	MarkDead(ctx context.Context, claim Claim, lastError string, reason string) error
	// Reaper: running jobs with no heartbeat for stateThreshold go back to queued
	// or dead if attempts are used up.
	RequeueStaleRunning(ctx context.Context, staleThreshold time.Duration, reaperID string) (int64, error)
}

func ResolveEnqueueOptions(jobType string, payload []byte, opts ...EnqueueOption) (EnqueueOptions, error) {
	o := EnqueueOptions{Type: jobType, Payload: payload, MaxAttempts: 3, RunAt: time.Now()}
	for _, opt := range opts {
		opt(&o)
	}
	if o.Type == "" {
		return o, errs.New(errs.CodeJobInvalidOption, "empty type")
	}
	if o.Queue == "" {
		return o, errs.New(errs.CodeJobInvalidOption, "job type %q has no queue (job.WithQueue)", o.Type)
	}
	if o.MaxAttempts < 1 {
		o.MaxAttempts = 1
	}
	return o, nil
}

// handles the job definitions (only postgres for now)
// JobScope is what one budget may see: its own jobs, the system jobs of the
// mailboxes feeding it, and system jobs that led to its jobs.
type JobScope struct {
	BudgetID uuid.UUID
	Accounts []string // lowercase addresses of the budget's mail connections
}

// JobTreeFilter picks job trees. A tree matches when any job in it matches, so
// filtering on "dead" still returns the whole tree around the dead job.
type JobTreeFilter struct {
	Scope     JobScope
	Status    JobStatus
	Retrying  bool // queued again after a failed attempt
	Type      string
	Queue     string
	ErrorCode string // a job failing with this code (JobFailure.Code)
	// HiddenRootTypes leaves out trees whose root is one of these types. A job
	// of the type inside another tree stays.
	HiddenRootTypes []string
	// HideEmptySyncs leaves out finished mailbox jobs with nothing under them
	// for this budget, and from the total too.
	HideEmptySyncs bool
	JobID          *uuid.UUID
	Since          *time.Time
	Limit          int
	Offset         int
}

// JobTree is a job with no root and everything it led to, oldest first. Jobs
// includes the root and leaves out other budgets' jobs under a shared root.
type JobTree struct {
	Root Job
	Jobs []Job
}

// JobCounts counts a scope's jobs by state. Done covers the overview's window;
// the others are what's happening now.
type JobCounts struct {
	Running  int
	Queued   int
	Retrying int
	Failed   int
	Done     int
}

// JobFailure is one cause of current failures: dead or retrying jobs grouped by
// the error code at the start of their last error.
type JobFailure struct {
	Code     string
	Message  string // the most recent error with this code
	Count    int
	LastSeen time.Time
}

type JobStore interface {
	GetJobByID(ctx context.Context, id uuid.UUID) (*Job, error)
	ListByRoot(ctx context.Context, rootID uuid.UUID) (jobs []Job, err error)
	// ListTrees lists the scope's job trees, newest first. total counts every
	// matching tree, not just this page.
	ListTrees(ctx context.Context, filter JobTreeFilter) (trees []JobTree, total int, err error)
	// Overview sums up the scope's jobs. Done counts jobs finished since since.
	Overview(ctx context.Context, scope JobScope, since time.Time) (JobCounts, []JobFailure, error)
	// ListEvents returns a job's history, oldest first.
	ListEvents(ctx context.Context, jobID uuid.UUID) ([]JobEvent, error)
	CancelJob(ctx context.Context, jobID uuid.UUID, actor string) (*Job, error)
	RetryDead(ctx context.Context, jobID uuid.UUID, actor string) (*Job, error)
	// RetryDeadByCode queues the scope's dead jobs whose last error starts with
	// code again, at most limit of them, and returns how many it queued.
	RetryDeadByCode(ctx context.Context, scope JobScope, code, actor string, limit int) (int, error)
	// Dismiss soft-deletes the jobs that have finished. Their idempotency keys
	// stay, so a dismissed message isn't imported again.
	Dismiss(ctx context.Context, jobIDs []uuid.UUID) error
}

// ScheduleStore keeps recurring job definitions (job_schedules).
type ScheduleStore interface {
	// EnsureSchedule creates the schedule named s.Name, or turns it back on
	// with s.NextRunAt. One that's already on keeps its next run.
	EnsureSchedule(ctx context.Context, s JobSchedule) error
	// DisableSchedule stops the named schedule. A name that doesn't exist is
	// not an error.
	DisableSchedule(ctx context.Context, name string) error
	// ListDueSchedules returns enabled schedules whose next run has passed.
	ListDueSchedules(ctx context.Context, now time.Time, limit int) ([]JobSchedule, error)
	// AdvanceSchedule moves a schedule from its run at from to the one at to.
	// If another scheduler already moved it, nothing changes.
	AdvanceSchedule(ctx context.Context, id uuid.UUID, from, to time.Time) error
	// ListSchedules returns the schedules scope can see, each with its latest
	// run. Enabled ones come first, soonest next run first.
	ListSchedules(ctx context.Context, scope ScheduleScope) ([]ScheduleOverview, error)
}

type JobLog interface {
	Append(ctx context.Context, e JobEvent) error
}

type JobEventType string

const (
	EventEnqueued       JobEventType = "enqueued"
	EventClaimed        JobEventType = "claimed"
	EventRetryScheduled JobEventType = "retry_scheduled"
	EventReaped         JobEventType = "reaped"
	EventCompleted      JobEventType = "completed"
	EventDead           JobEventType = "dead"
	EventCancelled      JobEventType = "cancelled"
	EventManualRetry    JobEventType = "manual_retry"
)

type JobEvent struct {
	ID         int64
	JobID      uuid.UUID
	Type       JobEventType
	Status     JobStatus
	Attempt    int
	Actor      string // cipher-worker-1, reaper, admin
	Error      string
	Reason     string
	DurationMs *int64
	NextRunAt  *time.Time
	CreatedAt  time.Time
	DeletedAt  *time.Time
}

type JobSchedule struct {
	ID        uuid.UUID
	Name      string
	BudgetID  *uuid.UUID
	Type      string
	Queue     string
	Payload   []byte
	Cron      string
	Timezone  string
	Enabled   bool
	NextRunAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// ScheduleScope is what one budget can see: its own schedules, and the system
// schedules named in SystemNames (those have no budget).
type ScheduleScope struct {
	BudgetID    uuid.UUID
	SystemNames []string
}

// ScheduleRun is the job a schedule queued for one tick.
type ScheduleRun struct {
	JobID        uuid.UUID
	Status       JobStatus
	ScheduledFor time.Time
	CompletedAt  *time.Time
	LastError    string
}

// ScheduleOverview is a schedule and its latest run, nil if it hasn't run.
type ScheduleOverview struct {
	JobSchedule
	LastRun *ScheduleRun
}
