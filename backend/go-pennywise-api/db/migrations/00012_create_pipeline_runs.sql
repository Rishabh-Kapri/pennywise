-- +goose Up
-- +goose StatementBegin
CREATE TYPE pipeline_run_status AS ENUM ('running', 'waiting_retry', 'completed', 'failed');
CREATE TYPE pipeline_event_status AS ENUM ('started', 'succeeded', 'skipped', 'failed', 'waiting_retry', 'retry_signaled');

-- One row per email-to-transaction workflow execution. Summary state for cheap
-- budget-scoped listing; the step-by-step timeline lives in pipeline_run_events.
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    workflow_id TEXT NOT NULL,
    workflow_run_id TEXT NOT NULL,
    -- "<workflow_id>-parsed" child that runs parse/predict/create; retry signals target this
    child_workflow_id TEXT,
    trigger TEXT NOT NULL DEFAULT 'gmail_push' CHECK (trigger IN ('gmail_push', 'manual')),
    email_account TEXT,
    status pipeline_run_status NOT NULL DEFAULT 'running',
    current_step TEXT NOT NULL DEFAULT 'fetch_user',
    error TEXT,
    emails_fetched INTEGER NOT NULL DEFAULT 0,
    emails_skipped INTEGER NOT NULL DEFAULT 0,
    transactions_created INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

-- Append-only timeline of step transitions; per-email entries carry message_id.
CREATE TABLE IF NOT EXISTS pipeline_run_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    step TEXT NOT NULL,
    status pipeline_event_status NOT NULL,
    message_id TEXT,
    -- Detail examples:
    -- parse:   {"merchant":"Amazon","account":"XX1234","amount":-499.0,"date":"2026-07-06","transactionType":"debit"}
    -- predict: {"payee":"Amazon","category":"Shopping","account":"HDFC","source":"VECTOR","confidence":"high","reasoning":"..."}
    -- create_transactions: {"transactionIds":["..."],"count":2}
    -- failed/waiting_retry: {"error":"...","retrySignal":"retry-predict"}
    detail JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_runs_workflow
    ON pipeline_runs(workflow_id, workflow_run_id);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_budget_started
    ON pipeline_runs(budget_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_pipeline_run_events_run_created
    ON pipeline_run_events(run_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pipeline_run_events;
DROP TABLE IF EXISTS pipeline_runs;
DROP TYPE IF EXISTS pipeline_event_status;
DROP TYPE IF EXISTS pipeline_run_status;
-- +goose StatementEnd
