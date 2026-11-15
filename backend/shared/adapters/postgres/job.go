package postgres

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// The broker only moves jobs between states. It writes no history: the
// job.ObservedBroker decorator records each change through the JobLog port,
// so the history works the same whichever broker adapter is configured.
//
// locked_by, locked_at and heartbeat_at are never cleared: they describe the
// last attempt. Ownership checks always include status = 'running', so a stale
// lock on a queued or finished job means nothing.

const jobColumns = `id, budget_id, type, queue, idempotency_key, payload, status, priority,
	max_attempts, attempt, metadata, root_job_id, parent_id, schedule_id,
	scheduled_for, progress, result, run_at, started_at, completed_at, locked_at,
	locked_by, last_error, created_at, updated_at, deleted_at`

type jobBroker struct {
	BaseRepository
}

type jobStore struct {
	BaseRepository
}

type jobLog struct {
	BaseRepository
}

func NewPgJobBrokerAdapter(pool *pgxpool.Pool) job.Broker {
	return &jobBroker{BaseRepository: NewBaseRepository(pool)}
}

func NewPgJobStoreAdapter(pool *pgxpool.Pool) job.JobStore {
	return &jobStore{BaseRepository: NewBaseRepository(pool)}
}

func NewPgJobLogAdapter(pool *pgxpool.Pool) job.JobLog {
	return &jobLog{BaseRepository: NewBaseRepository(pool)}
}

// ---------------------------------------------------------------------------
// Broker
// ---------------------------------------------------------------------------

// Enqueue implements [job.Broker]. A second Enqueue with the same
// (queue, type, idempotency key), or for the same (schedule, tick), returns the first
// job's ID with Duplicate set.
func (b *jobBroker) Enqueue(
	ctx context.Context,
	jobType string,
	payload []byte,
	opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	o, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	if len(o.Payload) == 0 {
		o.Payload = []byte("{}")
	}
	if !json.Valid(o.Payload) {
		return job.EnqueueResult{}, errs.NewWith(
			errs.CodeJobInvalidOption, "payload is not valid JSON", errs.WithDetail("type", o.Type),
		)
	}

	// Request metadata (correlation ID, origin service) travels with the job;
	// the runner restores it with Metadata.Context.
	metadata, err := json.Marshal(job.MetadataFromContext(ctx))
	if err != nil {
		return job.EnqueueResult{}, errs.Wrap(errs.CodeJobEnqueueFailed, "encode job metadata", err)
	}

	// DO NOTHING without a conflict target covers both unique indexes
	// (idempotency key and schedule tick). On a conflict no row comes back.
	var res job.EnqueueResult
	err = b.Executor(ctx).QueryRow(
		ctx, `
		INSERT INTO jobs (
			budget_id, type, idempotency_key, payload, priority, max_attempts,
			metadata, root_job_id, parent_id, schedule_id, scheduled_for, run_at, queue
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		o.BudgetID, o.Type, o.IdempotencyKey, o.Payload, o.Priority, o.MaxAttempts,
		metadata, o.RootJobID, o.ParentID, o.ScheduleID, o.ScheduledFor, o.RunAt, o.Queue,
	).Scan(&res.JobID)
	if stderrors.Is(err, pgx.ErrNoRows) {
		res.Duplicate = true
		res.JobID, err = b.existingJobID(ctx, o)
		if err != nil {
			return job.EnqueueResult{}, err
		}
		return res, nil
	}
	if err != nil {
		return job.EnqueueResult{}, errs.Wrap(errs.CodeJobEnqueueFailed, "insert job", err,
			errs.WithDetail("type", o.Type))
	}
	return res, nil
}

// existingJobID finds the job an Enqueue collided with.
func (b *jobBroker) existingJobID(ctx context.Context, o job.EnqueueOptions) (uuid.UUID, error) {
	var (
		id  uuid.UUID
		err error
	)
	switch {
	case o.IdempotencyKey != "":
		err = b.Executor(ctx).QueryRow(
			ctx,
			`SELECT id FROM jobs WHERE queue = $1 AND type = $2 AND idempotency_key = $3`,
			o.Queue, o.Type, o.IdempotencyKey,
		).Scan(&id)
	case o.ScheduleID != nil && o.ScheduledFor != nil:
		err = b.Executor(ctx).QueryRow(
			ctx,
			`SELECT id FROM jobs WHERE schedule_id = $1 AND scheduled_for = $2`,
			o.ScheduleID, o.ScheduledFor,
		).Scan(&id)
	default:
		// Only those two unique indexes can make the insert a no-op.
		return uuid.Nil, errs.NewWith(errs.CodeJobEnqueueFailed,
			"insert conflicted without an idempotency key or schedule tick",
			errs.WithDetail("type", o.Type))
	}
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodeJobLookupFailed, "find duplicate job", err,
			errs.WithDetail("type", o.Type))
	}
	return id, nil
}

// ClaimJob implements [job.Broker]. It looks once and never waits: with
// nothing ready it returns an error with code CodeJobsUnavailable. Waiting and
// polling are the runner's job.
func (b *jobBroker) ClaimJob(ctx context.Context, queues []string, workerID string) (*job.Job, error) {
	// SKIP LOCKED: a row another worker is claiming right now is skipped, not
	// waited on, so two workers never get the same job.
	j, err := scanJob(b.Executor(ctx).QueryRow(
		ctx, `
		UPDATE jobs SET
			status = 'running',
			attempt = attempt + 1,
			locked_by = $1,
			locked_at = now(),
			heartbeat_at = now(),
			started_at = COALESCE(started_at, now()),
			updated_at = now()
		WHERE id = (
			SELECT id FROM jobs
			WHERE queue = ANY($2) AND status = 'queued' AND deleted_at IS NULL AND run_at <= now()
			ORDER BY priority DESC, run_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING `+jobColumns,
		workerID, queues,
	))
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.New(errs.CodeJobsUnavailable, "no job ready to run")
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeJobClaimFailed, "claim job", err, errs.WithDetail("worker_id", workerID))
	}
	return j, nil
}

// Heartbeat implements [job.Broker].
func (b *jobBroker) Heartbeat(ctx context.Context, claim job.Claim) error {
	return b.updateClaimed(ctx, claim, "heartbeat", `heartbeat_at = now()`)
}

// UpdateProgress implements [job.Broker].
func (b *jobBroker) UpdateProgress(ctx context.Context, claim job.Claim, progress json.RawMessage) error {
	return b.updateClaimed(ctx, claim, "update progress",
		`progress = $4, heartbeat_at = now()`, jsonOrNull(progress))
}

// MarkCompleted implements [job.Broker].
// The attempt's duration is recorded in the job history by the
// ObservedBroker; the jobs row doesn't store it.
func (b *jobBroker) MarkCompleted(ctx context.Context, claim job.Claim, result json.RawMessage, _ time.Duration) error {
	return b.updateClaimed(ctx, claim, "mark completed",
		`status = 'succeeded', result = $4, completed_at = now()`, jsonOrNull(result))
}

// MarkPending implements [job.Broker]: the attempt failed and the job runs
// again at runAt. Whether attempts are left is the caller's decision; once
// they are used up, call MarkDead instead.
func (b *jobBroker) MarkPending(
	ctx context.Context,
	_ *job.Job,
	claim job.Claim,
	lastError string,
	runAt time.Time,
) error {
	return b.updateClaimed(ctx, claim, "mark pending",
		`status = 'queued', last_error = $4, run_at = $5`, lastError, runAt)
}

// MarkDead implements [job.Broker].
// reason goes to the job history (ObservedBroker); the row keeps lastError.
func (b *jobBroker) MarkDead(ctx context.Context, claim job.Claim, lastError string, _ string) error {
	return b.updateClaimed(ctx, claim, "mark dead",
		`status = 'dead', last_error = $4, completed_at = now()`, lastError)
}

// updateClaimed applies set to the job only while claim still owns it. set
// may use $4 onward for args; $1..$3 are the claim.
//
// Zero rows updated means the claim is gone: the reaper requeued the job
// after missed heartbeats, someone cancelled it, or a newer attempt holds it.
func (b *jobBroker) updateClaimed(
	ctx context.Context,
	claim job.Claim,
	action string,
	set string,
	args ...any,
) error {
	tag, err := b.Executor(ctx).Exec(
		ctx, `
		UPDATE jobs SET `+set+`, updated_at = now()
		WHERE id = $1 AND locked_by = $2 AND attempt = $3 AND status = 'running'`,
		append([]any{claim.JobID, claim.WorkerID, claim.Attempt}, args...)...,
	)
	if err != nil {
		return errs.Wrap(errs.CodeJobUpdateFailed, action, err, claimDetails(claim))
	}
	if tag.RowsAffected() == 0 {
		return errs.NewWith(errs.CodeJobClaimLost, action+": job is no longer owned by this worker",
			claimDetails(claim))
	}
	return nil
}

// RequeueStaleRunning implements [job.Broker]: running jobs whose heartbeat
// is older than staleThreshold go back to queued, or to dead when no attempts
// are left. Uses the database clock, so app servers with skewed clocks agree.
func (b *jobBroker) RequeueStaleRunning(
	ctx context.Context,
	staleThreshold time.Duration,
	reaperID string,
) (int64, error) {
	tag, err := b.Executor(ctx).Exec(
		ctx, `
		UPDATE jobs SET
			status = CASE WHEN attempt >= max_attempts
				THEN 'dead'::job_status ELSE 'queued'::job_status END,
			completed_at = CASE WHEN attempt >= max_attempts THEN now() END,
			last_error = 'no heartbeat from ' || COALESCE(locked_by, 'unknown worker'),
			run_at = now(),
			updated_at = now()
		WHERE id IN (
			SELECT id FROM jobs
			WHERE status = 'running'
				AND COALESCE(heartbeat_at, locked_at) < now() - make_interval(secs => $1)
			FOR UPDATE SKIP LOCKED
		)`,
		staleThreshold.Seconds(),
	)
	if err != nil {
		return 0, errs.Wrap(errs.CodeJobUpdateFailed, "requeue stale jobs", err,
			errs.WithDetail("reaper_id", reaperID))
	}
	return tag.RowsAffected(), nil
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

// GetJobByID implements [job.JobStore].
func (s *jobStore) GetJobByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	j, err := scanJob(s.Executor(ctx).QueryRow(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE id = $1 AND deleted_at IS NULL`, id))
	if stderrors.Is(err, pgx.ErrNoRows) {
		return nil, errs.Wrap(errs.CodeJobNotFound, "job not found", err, errs.WithDetail("job_id", id))
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "get job", err, errs.WithDetail("job_id", id))
	}
	return j, nil
}

// ListByRoot implements [job.JobStore]: the root job and every job it
// spawned, oldest first.
func (s *jobStore) ListByRoot(ctx context.Context, rootID uuid.UUID) ([]job.Job, error) {
	rows, err := s.Executor(ctx).Query(ctx, `
		SELECT `+jobColumns+` FROM jobs
		WHERE (id = $1 OR root_job_id = $1) AND deleted_at IS NULL
		ORDER BY created_at`, rootID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list jobs by root", err,
			errs.WithDetail("root_job_id", rootID))
	}
	defer rows.Close()

	var jobs []job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, errs.Wrap(errs.CodeJobLookupFailed, "scan job", err)
		}
		jobs = append(jobs, *j)
	}
	if err := rows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list jobs by root", err,
			errs.WithDetail("root_job_id", rootID))
	}
	return jobs, nil
}

// inScope is the condition for a job the scope may see directly: the budget's
// own, or the system job of one of its mailboxes.
func inScope(alias string, scope job.JobScope) sq.Sqlizer {
	return sq.Expr(
		alias+`.budget_id = ? OR (`+alias+`.budget_id IS NULL AND `+alias+`.type = ? AND lower(`+alias+`.payload->>'account') = ANY(?))`,
		scope.BudgetID, model.JobTypeMailboxChanged, scope.Accounts,
	)
}

// ListTrees implements [job.JobStore].
func (s *jobStore) ListTrees(ctx context.Context, f job.JobTreeFilter) ([]job.JobTree, int, error) {
	if f.Scope.Accounts == nil {
		f.Scope.Accounts = []string{}
	}
	direct, directArgs, err := inScope("jobs", f.Scope).ToSql()
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "build job scope", err)
	}
	where := sq.And{
		sq.Expr("jobs.deleted_at IS NULL AND jobs.root_job_id IS NULL"),
		// A system job is also visible through a job of this budget under it,
		// which keeps old jobs reachable after a mailbox is detached.
		sq.Expr(`(`+direct+` OR (jobs.budget_id IS NULL AND EXISTS (
			SELECT 1 FROM jobs c
			WHERE c.root_job_id = jobs.id AND c.budget_id = ? AND c.deleted_at IS NULL)))`,
			append(directArgs, f.Scope.BudgetID)...),
	}

	match := sq.And{
		sq.Expr("(m.id = jobs.id OR m.root_job_id = jobs.id)"),
		sq.Expr("m.deleted_at IS NULL"),
		sq.Expr("(m.budget_id = ? OR m.budget_id IS NULL)", f.Scope.BudgetID),
	}
	filtering := false
	add := func(cond sq.Sqlizer) {
		match, filtering = append(match, cond), true
	}
	if f.Status != "" {
		add(sq.Expr("m.status = ?::job_status", string(f.Status)))
	}
	if f.Retrying {
		add(sq.Expr("m.status = 'queued' AND m.attempt > 0"))
	}
	if f.Type != "" {
		add(sq.Eq{"m.type": f.Type})
	}
	if f.Queue != "" {
		add(sq.Eq{"m.queue": f.Queue})
	}
	if f.JobID != nil {
		add(sq.Eq{"m.id": *f.JobID})
	}
	if f.ErrorCode != "" {
		add(sq.Expr(
			"split_part(m.last_error, ': ', 1) = ? AND (m.status = 'dead' OR (m.status = 'queued' AND m.attempt > 0))",
			f.ErrorCode,
		))
	}
	if filtering {
		cond, args, err := match.ToSql()
		if err != nil {
			return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "build job filter", err)
		}
		where = append(where, sq.Expr("EXISTS (SELECT 1 FROM jobs m WHERE "+cond+")", args...))
	}
	if f.Since != nil {
		where = append(where, sq.Expr("jobs.created_at >= ?", *f.Since))
	}
	if len(f.HiddenRootTypes) > 0 {
		where = append(where, sq.NotEq{"jobs.type": f.HiddenRootTypes})
	}
	if f.HideEmptySyncs {
		// A sync that broke or is still running stays. Children are matched the
		// way the tree's members are, so a sync that only found another budget's
		// email counts as empty here.
		where = append(where, sq.Expr(`NOT (
			jobs.type IN (?, ?) AND jobs.status IN ('succeeded', 'cancelled') AND NOT EXISTS (
				SELECT 1 FROM jobs c
				WHERE c.root_job_id = jobs.id AND c.deleted_at IS NULL
					AND (c.budget_id = ? OR c.budget_id IS NULL)))`,
			model.JobTypeMailboxChanged, model.JobTypeMailboxBackfill, f.Scope.BudgetID))
	}

	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Select(jobColumns, "count(*) OVER () AS total").
		From("jobs").
		Where(where).
		OrderBy("created_at DESC")
	if f.Limit > 0 {
		query = query.Limit(uint64(f.Limit))
	}
	if f.Offset > 0 {
		query = query.Offset(uint64(f.Offset))
	}
	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "build job tree query", err)
	}
	rows, err := s.Executor(ctx).Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "list job trees", err)
	}
	defer rows.Close()

	var (
		trees []job.JobTree
		ids   []uuid.UUID
		total int
	)
	for rows.Next() {
		root, err := scanJob(rows, &total)
		if err != nil {
			return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "scan job tree", err)
		}
		trees = append(trees, job.JobTree{Root: *root})
		ids = append(ids, root.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "list job trees", err)
	}
	if len(trees) == 0 {
		return nil, total, nil
	}

	members, err := s.Executor(ctx).Query(ctx, `
		SELECT `+jobColumns+` FROM jobs
		WHERE (id = ANY($1) OR root_job_id = ANY($1))
			AND (budget_id = $2 OR budget_id IS NULL)
			AND deleted_at IS NULL
		ORDER BY created_at`, ids, f.Scope.BudgetID)
	if err != nil {
		return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "list job tree members", err)
	}
	defer members.Close()

	index := make(map[uuid.UUID]int, len(trees))
	for i := range trees {
		index[trees[i].Root.ID] = i
	}
	for members.Next() {
		j, err := scanJob(members)
		if err != nil {
			return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "scan job tree member", err)
		}
		rootID := j.ID
		if j.RootJobID != nil {
			rootID = *j.RootJobID
		}
		if i, ok := index[rootID]; ok {
			trees[i].Jobs = append(trees[i].Jobs, *j)
		}
	}
	if err := members.Err(); err != nil {
		return nil, 0, errs.Wrap(errs.CodeJobLookupFailed, "list job tree members", err)
	}
	return trees, total, nil
}

// Overview implements [job.JobStore].
func (s *jobStore) Overview(
	ctx context.Context,
	scope job.JobScope,
	since time.Time,
) (job.JobCounts, []job.JobFailure, error) {
	if scope.Accounts == nil {
		scope.Accounts = []string{}
	}
	var counts job.JobCounts
	err := s.Executor(ctx).QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'running'),
			count(*) FILTER (WHERE status = 'queued' AND attempt = 0),
			count(*) FILTER (WHERE status = 'queued' AND attempt > 0),
			count(*) FILTER (WHERE status = 'dead'),
			count(*) FILTER (WHERE status = 'succeeded' AND completed_at >= $3)
		FROM jobs j
		WHERE deleted_at IS NULL AND (
			j.budget_id = $1 OR (j.budget_id IS NULL AND j.type = $4 AND lower(j.payload->>'account') = ANY($2)))`,
		scope.BudgetID, scope.Accounts, since, model.JobTypeMailboxChanged,
	).Scan(&counts.Running, &counts.Queued, &counts.Retrying, &counts.Failed, &counts.Done)
	if err != nil {
		return counts, nil, errs.Wrap(errs.CodeJobLookupFailed, "count jobs", err)
	}

	rows, err := s.Executor(ctx).Query(ctx, `
		SELECT
			split_part(last_error, ': ', 1) AS code,
			(array_agg(last_error ORDER BY updated_at DESC))[1],
			count(*),
			max(updated_at)
		FROM jobs j
		WHERE deleted_at IS NULL
			AND last_error IS NOT NULL
			AND (status = 'dead' OR (status = 'queued' AND attempt > 0))
			AND (j.budget_id = $1 OR (j.budget_id IS NULL AND j.type = $3 AND lower(j.payload->>'account') = ANY($2)))
		GROUP BY code
		ORDER BY count(*) DESC, max(updated_at) DESC
		LIMIT 10`,
		scope.BudgetID, scope.Accounts, model.JobTypeMailboxChanged,
	)
	if err != nil {
		return counts, nil, errs.Wrap(errs.CodeJobLookupFailed, "group job failures", err)
	}
	defer rows.Close()

	var failures []job.JobFailure
	for rows.Next() {
		var f job.JobFailure
		if err := rows.Scan(&f.Code, &f.Message, &f.Count, &f.LastSeen); err != nil {
			return counts, nil, errs.Wrap(errs.CodeJobLookupFailed, "scan job failure", err)
		}
		failures = append(failures, f)
	}
	if err := rows.Err(); err != nil {
		return counts, nil, errs.Wrap(errs.CodeJobLookupFailed, "group job failures", err)
	}
	return counts, failures, nil
}

// ListEvents implements [job.JobStore].
func (s *jobStore) ListEvents(ctx context.Context, jobID uuid.UUID) ([]job.JobEvent, error) {
	rows, err := s.Executor(ctx).Query(ctx, `
		SELECT id, job_id, type::text, status::text, attempt, actor,
			COALESCE(error, ''), COALESCE(reason, ''), duration_ms, next_run_at, created_at
		FROM job_events
		WHERE job_id = $1
		ORDER BY created_at, id`, jobID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list job events", err, errs.WithDetail("job_id", jobID))
	}
	defer rows.Close()

	var events []job.JobEvent
	for rows.Next() {
		var (
			e                 job.JobEvent
			eventType, status string
		)
		if err := rows.Scan(&e.ID, &e.JobID, &eventType, &status, &e.Attempt, &e.Actor,
			&e.Error, &e.Reason, &e.DurationMs, &e.NextRunAt, &e.CreatedAt); err != nil {
			return nil, errs.Wrap(errs.CodeJobLookupFailed, "scan job event", err)
		}
		e.Type, e.Status = job.JobEventType(eventType), job.JobStatus(status)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeJobLookupFailed, "list job events", err, errs.WithDetail("job_id", jobID))
	}
	return events, nil
}

// CancelJob implements [job.JobStore]. Only a queued job can be cancelled:
// a running handler can't be stopped from the database.
func (s *jobStore) CancelJob(ctx context.Context, jobID uuid.UUID, actor string) (*job.Job, error) {
	return s.transition(ctx, jobID, actor, transition{
		action: "cancel job",
		set:    `status = 'cancelled', completed_at = now()`,
		from:   job.QueuedJobStatus,
		event:  job.EventCancelled,
	})
}

// RetryDead implements [job.JobStore]: a dead job is queued again with its
// attempts reset, so it gets the full MaxAttempts.
func (s *jobStore) RetryDead(ctx context.Context, jobID uuid.UUID, actor string) (*job.Job, error) {
	return s.transition(ctx, jobID, actor, transition{
		action: "retry dead job",
		set:    `status = 'queued', attempt = 0, run_at = now(), completed_at = NULL`,
		from:   job.DeadJobStatus,
		event:  job.EventManualRetry,
	})
}

// transition is a manual status change on a job no worker holds.
type transition struct {
	action string
	set    string
	from   job.JobStatus
	event  job.JobEventType
}

// transition applies t and records it in job_events in the same transaction:
// the store and its history share one database, so they can commit together.
func (s *jobStore) transition(ctx context.Context, jobID uuid.UUID, actor string, t transition) (*job.Job, error) {
	var updated *job.Job
	err := s.InTx(ctx, func(ctx context.Context) error {
		j, err := scanJob(s.Executor(ctx).QueryRow(
			ctx, `
			UPDATE jobs SET `+t.set+`, updated_at = now()
			WHERE id = $1 AND status = $2::job_status AND deleted_at IS NULL
			RETURNING `+jobColumns,
			jobID, string(t.from),
		))
		if stderrors.Is(err, pgx.ErrNoRows) {
			return s.whyNotTransitioned(ctx, jobID, t)
		}
		if err != nil {
			return errs.Wrap(errs.CodeJobUpdateFailed, t.action, err, errs.WithDetail("job_id", jobID))
		}
		updated = j
		return appendJobEvent(ctx, s.Executor(ctx), job.JobEvent{
			JobID:   j.ID,
			Type:    t.event,
			Status:  j.Status,
			Attempt: j.Attempt,
			Actor:   actor,
		})
	})
	if err != nil {
		return nil, wrapUncoded(err, errs.CodeJobUpdateFailed, t.action)
	}
	return updated, nil
}

// RetryDeadByCode implements [job.JobStore].
func (s *jobStore) RetryDeadByCode(
	ctx context.Context,
	scope job.JobScope,
	code, actor string,
	limit int,
) (int, error) {
	if scope.Accounts == nil {
		scope.Accounts = []string{}
	}
	retried := 0
	err := s.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.Executor(ctx).Query(ctx, `
			WITH target AS (
				SELECT j.id FROM jobs j
				WHERE j.deleted_at IS NULL
					AND j.status = 'dead'
					AND split_part(j.last_error, ': ', 1) = $1
					AND (j.budget_id = $2 OR (j.budget_id IS NULL AND j.type = $3 AND lower(j.payload->>'account') = ANY($4)))
				ORDER BY j.updated_at
				LIMIT $5
				FOR UPDATE SKIP LOCKED
			)
			UPDATE jobs SET status = 'queued', attempt = 0, run_at = now(), completed_at = NULL, updated_at = now()
			FROM target WHERE jobs.id = target.id
			RETURNING jobs.id`,
			code, scope.BudgetID, model.JobTypeMailboxChanged, scope.Accounts, limit,
		)
		if err != nil {
			return errs.Wrap(errs.CodeJobUpdateFailed, "retry dead jobs", err)
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return errs.Wrap(errs.CodeJobUpdateFailed, "retry dead jobs", err)
		}
		for _, id := range ids {
			if err := appendJobEvent(ctx, s.Executor(ctx), job.JobEvent{
				JobID:  id,
				Type:   job.EventManualRetry,
				Status: job.QueuedJobStatus,
				Actor:  actor,
			}); err != nil {
				return err
			}
		}
		retried = len(ids)
		return nil
	})
	if err != nil {
		return 0, wrapUncoded(err, errs.CodeJobUpdateFailed, "retry dead jobs")
	}
	return retried, nil
}

// Dismiss implements [job.JobStore].
func (s *jobStore) Dismiss(ctx context.Context, jobIDs []uuid.UUID) error {
	_, err := s.Executor(ctx).Exec(ctx, `
		UPDATE jobs SET deleted_at = now(), updated_at = now()
		WHERE id = ANY($1) AND deleted_at IS NULL AND status NOT IN ('queued', 'running')`, jobIDs)
	if err != nil {
		return errs.Wrap(errs.CodeJobUpdateFailed, "dismiss jobs", err)
	}
	return nil
}

// whyNotTransitioned tells "no such job" apart from "wrong status".
func (s *jobStore) whyNotTransitioned(ctx context.Context, jobID uuid.UUID, t transition) error {
	current, err := s.GetJobByID(ctx, jobID)
	if err != nil {
		return err
	}
	return errs.NewWith(errs.CodeJobInvalidState, t.action+": job is not "+string(t.from),
		errs.WithDetail("job_id", jobID),
		errs.WithDetail("status", string(current.Status)))
}

// ---------------------------------------------------------------------------
// Log
// ---------------------------------------------------------------------------

// Append implements [job.JobLog]. ID and CreatedAt are set by the database.
func (l *jobLog) Append(ctx context.Context, e job.JobEvent) error {
	return appendJobEvent(ctx, l.Executor(ctx), e)
}

func appendJobEvent(ctx context.Context, db DBTX, e job.JobEvent) error {
	_, err := db.Exec(
		ctx, `
		INSERT INTO job_events (job_id, type, status, attempt, actor, error, reason, duration_ms, next_run_at)
		VALUES ($1, $2::job_event_type, $3::job_status, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9)`,
		e.JobID, string(e.Type), string(e.Status), e.Attempt, e.Actor, e.Error, e.Reason, e.DurationMs, e.NextRunAt,
	)
	if err != nil {
		return errs.Wrap(errs.CodeJobEventAppendFailed, "append job event", err,
			errs.WithDetail("job_id", e.JobID), errs.WithDetail("event", string(e.Type)))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// scanJob reads one row of jobColumns, plus any extra trailing columns.
func scanJob(row pgx.Row, extra ...any) (*job.Job, error) {
	var (
		j       job.Job
		key     *string
		status  string
		lastErr *string
		meta    []byte
	)
	dest := []any{
		&j.ID, &j.BudgetID, &j.Type, &j.Queue, &key, &j.Payload, &status, &j.Priority,
		&j.MaxAttempts, &j.Attempt, &meta, &j.RootJobID, &j.ParentID, &j.ScheduleID,
		&j.ScheduledFor, &j.Progress, &j.Result, &j.RunAt, &j.StartedAt, &j.CompletedAt, &j.LockedAt,
		&j.LockedBy, &lastErr, &j.CreatedAt, &j.UpdatedAt, &j.DeletedAt,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	j.Status = job.JobStatus(status)
	if len(meta) > 0 {
		if err := json.Unmarshal(meta, &j.Metadata); err != nil {
			return nil, err
		}
	}
	if key != nil {
		j.IdempotencyKey = *key
	}
	if lastErr != nil {
		j.LastError = *lastErr
	}
	return &j, nil
}

// wrapUncoded gives a job code to errors that didn't come from this adapter:
// InTx returns BeginTx and Commit failures (including ctx cancellation) as is.
// Errors that already carry a code pass through unchanged, and the cause stays
// reachable, so errors.Is(err, context.Canceled) still works.
func wrapUncoded(err error, code errs.Code, action string) error {
	var coded *errs.Error
	if err == nil || stderrors.As(err, &coded) {
		return err
	}
	return errs.Wrap(code, action, err)
}

// jsonOrNull stores an empty result or progress as NULL, not as invalid JSON.
func jsonOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

func claimDetails(claim job.Claim) errs.Option {
	return errs.WithDetails(map[string]any{
		"job_id":    claim.JobID,
		"worker_id": claim.WorkerID,
		"attempt":   claim.Attempt,
	})
}
