-- +goose Up
-- One row per bank message (email, SMS, API) a budget received, holding where
-- the message stands. Before this, "waiting for review" could only be worked
-- out by walking a message's jobs and decoding its last pipeline.report.
--
-- The pipeline.report handler writes the row when a run ends; continue, run
-- again and dismiss move it. The moves allowed are model.InboundMessageStatus
-- .CanMoveTo:
--
--   (first report) ──► needs_review | done
--   processing     ──► needs_review | done      (a run reported)
--   needs_review   ──► processing               (continue)
--   done           ──► processing               (run again)
--   any but dismissed ──► dismissed             (dismiss)
--
-- A message still on its first run has no row yet; the jobs show it.
CREATE TYPE inbound_message_status AS ENUM ('processing', 'needs_review', 'done', 'dismissed');

CREATE TABLE inbound_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    -- The message's identity: model.ReceivedMessage without its RunID, so a
    -- continued or re-run message stays one row.
    source TEXT NOT NULL,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    status inbound_message_status NOT NULL,
    -- model.MessageOutcome of the last report, and why.
    outcome TEXT NOT NULL,
    reason TEXT,
    -- model.MessageReview: what the person is asked, and the job that carries
    -- on if they continue. Always set while needs_review; kept after a move so
    -- a failed continue can go back. The next report replaces it.
    review JSONB,
    transaction_ids UUID[] NOT NULL DEFAULT '{}',
    -- The pipeline.report job behind this row. No foreign key: the job store
    -- may be another database (JOB_STORE_URL).
    report_job_id UUID NOT NULL,
    reported_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (budget_id, source, provider, external_id),
    CHECK (status <> 'needs_review' OR review IS NOT NULL)
);
-- "What's waiting for me?" and the newest messages by state.
CREATE INDEX idx_inbound_messages_budget_status
    ON inbound_messages (budget_id, status, updated_at DESC);

-- Messages already parked or finished: the latest report each message still
-- shows (continue and run again hide the reports they replace).
INSERT INTO inbound_messages (
    budget_id, source, provider, external_id, status, outcome, reason, review,
    transaction_ids, report_job_id, reported_at
)
SELECT DISTINCT ON (budget_id, payload->'message'->>'source', COALESCE(payload->'message'->>'provider', ''),
                    payload->'message'->>'externalId')
    budget_id,
    payload->'message'->>'source',
    COALESCE(payload->'message'->>'provider', ''),
    payload->'message'->>'externalId',
    CASE WHEN payload->>'outcome' = 'needs_review' AND jsonb_typeof(payload->'review') = 'object'
         THEN 'needs_review' ELSE 'done' END::inbound_message_status,
    payload->>'outcome',
    NULLIF(payload->>'reason', ''),
    CASE WHEN payload->>'outcome' = 'needs_review' AND jsonb_typeof(payload->'review') = 'object'
         THEN payload->'review' END,
    CASE WHEN jsonb_typeof(payload->'transactionIds') = 'array'
         THEN ARRAY(SELECT jsonb_array_elements_text(payload->'transactionIds'))::uuid[]
         ELSE '{}' END,
    id,
    created_at
FROM jobs
WHERE type = 'pipeline.report'
    AND status = 'succeeded'
    AND deleted_at IS NULL
    AND budget_id IS NOT NULL
    AND payload->'message'->>'source' IS NOT NULL
    AND payload->'message'->>'externalId' IS NOT NULL
    AND payload->>'outcome' IS NOT NULL
ORDER BY budget_id, payload->'message'->>'source', COALESCE(payload->'message'->>'provider', ''),
         payload->'message'->>'externalId', created_at DESC;

-- +goose Down
DROP TABLE inbound_messages;
DROP TYPE inbound_message_status;
