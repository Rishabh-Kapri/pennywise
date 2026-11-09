-- +goose Up
-- Job queue (shared/ports/job): Postgres orchestrates retries instead of
-- Temporal. jobs doubles as the outbox for broker adapters.
CREATE TYPE job_status AS ENUM ('queued', 'running', 'cancelled', 'succeeded', 'dead');
CREATE TYPE job_event_type AS ENUM (
    'enqueued', 'claimed', 'retry_scheduled', 'reaped',
    'completed', 'dead', 'cancelled', 'manual_retry'
);

-- Recurring job definitions. A scheduler enqueues a job per tick and moves
-- next_run_at forward.
CREATE TABLE job_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    budget_id UUID REFERENCES budgets(id) ON DELETE CASCADE,  -- NULL = system schedule
    type TEXT NOT NULL,
    queue TEXT NOT NULL,                                      -- where each tick's job goes
    payload JSONB NOT NULL DEFAULT '{}',
    cron TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled BOOLEAN NOT NULL DEFAULT true,
    next_run_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_job_schedules_due ON job_schedules (next_run_at)
    WHERE enabled AND deleted_at IS NULL;

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID REFERENCES budgets(id) ON DELETE CASCADE,  -- NULL = system job
    type TEXT NOT NULL,
    -- The producer always names it (job.WithQueue); a worker claims from the
    -- queues it is given.
    queue TEXT NOT NULL,
    idempotency_key TEXT,                                     -- NULL = no dedupe
    payload JSONB NOT NULL DEFAULT '{}',
    status job_status NOT NULL DEFAULT 'queued',
    priority INTEGER NOT NULL DEFAULT 0,                      -- higher runs first
    max_attempts INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts >= 1),
    attempt INTEGER NOT NULL DEFAULT 0,
    -- job.Metadata: correlation ID and origin service of the enqueuing
    -- request. Never the user or a trust flag.
    metadata JSONB NOT NULL DEFAULT '{}',

    root_job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    parent_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    schedule_id UUID REFERENCES job_schedules(id) ON DELETE SET NULL,
    scheduled_for TIMESTAMPTZ,                                -- the cron tick this job is for

    progress JSONB,
    result JSONB,

    run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    -- The last attempt's claim; never cleared. Ownership checks also require
    -- status = 'running'.
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    heartbeat_at TIMESTAMPTZ,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
-- Idempotency: one job type can go to several queues, one row per queue, so
-- each queue dedupes its own copy.
CREATE UNIQUE INDEX idx_jobs_idempotency ON jobs (queue, type, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
-- A schedule fires at most once per tick, even with two schedulers.
CREATE UNIQUE INDEX idx_jobs_schedule_tick ON jobs (schedule_id, scheduled_for)
    WHERE schedule_id IS NOT NULL;
-- ClaimJob: due queued jobs on a queue, highest priority, then oldest.
CREATE INDEX idx_jobs_claim ON jobs (queue, priority DESC, run_at)
    WHERE status = 'queued' AND deleted_at IS NULL;
-- Reaper: running jobs whose heartbeat went stale.
CREATE INDEX idx_jobs_running_heartbeat ON jobs (heartbeat_at) WHERE status = 'running';
CREATE INDEX idx_jobs_root ON jobs (root_job_id) WHERE root_job_id IS NOT NULL;
CREATE INDEX idx_jobs_budget_created ON jobs (budget_id, created_at DESC);

-- Append-only history of every state change (job.ObservedBroker, JobStore).
CREATE TABLE job_events (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    type job_event_type NOT NULL,
    status job_status NOT NULL,
    attempt INTEGER NOT NULL,
    actor TEXT NOT NULL,                                      -- cipher-worker-1, reaper, admin
    error TEXT,
    reason TEXT,
    duration_ms BIGINT,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_job_events_job_created ON job_events (job_id, created_at);

-- +goose Down
DROP TABLE job_events;
DROP TABLE jobs;
DROP TABLE job_schedules;
DROP TYPE job_event_type;
DROP TYPE job_status;
