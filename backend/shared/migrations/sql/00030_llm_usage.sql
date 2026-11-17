-- +goose Up
-- One row per call to a model provider: chat (agent turns, titles, memory,
-- pipeline steps), embeddings and decision endpoints. cipher writes it from
-- the clients its resolver builds, so every caller is covered, including
-- calls that failed. Never holds prompt or response text.
CREATE TABLE llm_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- NULL only for a call made outside a budget's context.
    budget_id UUID REFERENCES budgets(id) ON DELETE CASCADE,
    -- The owner of the key the call ran on.
    user_id UUID REFERENCES auth_users(id) ON DELETE CASCADE,
    -- Set for calls made for an agent run (its turns, title, embeddings).
    agent_run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL,
    -- What the call was for: agent_turn, title, memory_observe,
    -- pipeline_check, pipeline_extract, pipeline_verify, pipeline_predict,
    -- statement_extract, statement_fallback, embedding, decision, model_test,
    -- the old email flow's steps (parse_extract, predict_summarize, ...), other.
    purpose TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL, -- the model that answered, else the one asked for
    prompt_version TEXT,
    -- input_tokens counts every prompt token, cached or not; the cache columns
    -- are the part of it read from or written to the prompt cache. NULL when
    -- the provider reported no usage.
    input_tokens INTEGER,
    output_tokens INTEGER,
    cache_read_tokens INTEGER,
    cache_write_tokens INTEGER,
    -- Millionths of a US dollar, as the provider reported it; 0 for a local
    -- model; NULL when the provider reports no cost.
    cost_micros BIGINT,
    latency_ms INTEGER NOT NULL,
    stop_reason TEXT,
    status TEXT NOT NULL,
    error TEXT, -- the provider's error, cut to 500 characters
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('ok', 'error', 'cancelled')),
    CHECK (latency_ms >= 0),
    CHECK (
        (input_tokens IS NULL OR input_tokens >= 0)
        AND (output_tokens IS NULL OR output_tokens >= 0)
        AND (cache_read_tokens IS NULL OR cache_read_tokens >= 0)
        AND (cache_write_tokens IS NULL OR cache_write_tokens >= 0)
        AND (cost_micros IS NULL OR cost_micros >= 0)
    )
);
-- Per budget per day, and per run.
CREATE INDEX idx_llm_usage_budget_created ON llm_usage (budget_id, created_at DESC);
CREATE INDEX idx_llm_usage_agent_run ON llm_usage (agent_run_id) WHERE agent_run_id IS NOT NULL;

-- +goose Down
DROP TABLE llm_usage;
