-- +goose Up
-- Agent run metrics as columns, and each tool call of a run as a row.
-- Before this, a run's tokens and tool calls lived in agent_runs.metadata.
-- Old rows keep them there and leave these columns NULL (no backfill: the old
-- token counts don't say which provider counted them, and Anthropic counted
-- cached tokens apart from input).

-- Rollups of the run's llm_usage rows (purpose 'agent_turn'), written by
-- cipher in one statement when the run finishes. llm_usage is the source of
-- truth; these are copies kept for per-run queries and alerts.
ALTER TABLE agent_runs
    -- Why the run ended: end_turn, max_tokens, error, max_turns (turn budget
    -- spent, a final answer was forced) or max_tool_calls (tool budget spent).
    ADD COLUMN stop_reason TEXT,
    -- input_tokens counts every prompt token, cached or not; cache_read_tokens
    -- and cache_write_tokens are the part of it served from or written to the
    -- provider's prompt cache. NULL when no call reported usage.
    ADD COLUMN input_tokens INTEGER,
    ADD COLUMN output_tokens INTEGER,
    ADD COLUMN cache_read_tokens INTEGER,
    ADD COLUMN cache_write_tokens INTEGER,
    -- Cost the providers reported, in millionths of a US dollar. NULL when none did.
    ADD COLUMN cost_micros BIGINT,
    -- Hash of the system prompt and tool definitions the run used.
    ADD COLUMN prompt_version TEXT,
    -- Provider and model that actually answered the run's last turn.
    ADD COLUMN resolved_provider TEXT,
    ADD COLUMN resolved_model TEXT,
    -- Model calls the run made, and tools it ran.
    ADD COLUMN turn_count INTEGER,
    ADD COLUMN tool_call_count INTEGER,
    ADD CONSTRAINT agent_runs_metrics_non_negative CHECK (
        (input_tokens IS NULL OR input_tokens >= 0)
        AND (output_tokens IS NULL OR output_tokens >= 0)
        AND (cache_read_tokens IS NULL OR cache_read_tokens >= 0)
        AND (cache_write_tokens IS NULL OR cache_write_tokens >= 0)
        AND (cost_micros IS NULL OR cost_micros >= 0)
        AND (turn_count IS NULL OR turn_count >= 0)
        AND (tool_call_count IS NULL OR tool_call_count >= 0)
    );

COMMENT ON COLUMN agent_runs.metadata IS
'Run-level metadata example: {"traceId":"trace-123","enabledTools":["get_schema","execute_sql"],"maxTokens":1024}. Runs from before migration 29 also hold inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, totalTokens and toolCalls here.';

-- One row per tool call an agent run made. Holds the user's data (query
-- arguments, rows the tool returned): not granted to the agent role, and
-- deleted with the conversation (DeleteConversation), not soft-deleted.
-- cipher bounds input and output (32 KiB each); a larger one is stored as
-- {"truncated": true, "bytes": N, "preview": "..."} with truncated set.
CREATE TABLE tool_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL, -- order of the call within the run, from 1
    turn INTEGER NOT NULL,     -- the model turn that asked for it, from 1
    tool_call_id TEXT,         -- the provider's id, pairs the call with its result
    tool_name TEXT NOT NULL,
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB,
    is_error BOOLEAN NOT NULL DEFAULT false,
    input_bytes INTEGER NOT NULL DEFAULT 0,  -- size before truncation
    output_bytes INTEGER NOT NULL DEFAULT 0, -- size before truncation
    truncated BOOLEAN NOT NULL DEFAULT false,
    duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (sequence > 0),
    CHECK (turn > 0),
    CHECK (input_bytes >= 0 AND output_bytes >= 0),
    CHECK (duration_ms IS NULL OR duration_ms >= 0),
    UNIQUE (run_id, sequence)
);
CREATE INDEX idx_tool_invocations_budget_created ON tool_invocations (budget_id, created_at DESC);
CREATE INDEX idx_tool_invocations_tool_created ON tool_invocations (tool_name, created_at DESC);

-- +goose Down
DROP TABLE tool_invocations;

COMMENT ON COLUMN agent_runs.metadata IS
'Run-level metadata example: {"traceId":"trace-123","toolsEnabled":["get_schema","execute_sql"],"inputTokens":1200,"outputTokens":340,"latencyMs":5400}';

ALTER TABLE agent_runs
    DROP CONSTRAINT agent_runs_metrics_non_negative,
    DROP COLUMN tool_call_count,
    DROP COLUMN turn_count,
    DROP COLUMN resolved_model,
    DROP COLUMN resolved_provider,
    DROP COLUMN prompt_version,
    DROP COLUMN cost_micros,
    DROP COLUMN cache_write_tokens,
    DROP COLUMN cache_read_tokens,
    DROP COLUMN output_tokens,
    DROP COLUMN input_tokens,
    DROP COLUMN stop_reason;
