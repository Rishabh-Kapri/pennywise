-- +goose Up
-- +goose StatementBegin
-- Per-run LLM accounting: how many model round-trips the run made, what they
-- cost in tokens, and the per-model breakdown ({"gemma4:12b": {...}}).
ALTER TABLE pipeline_runs
    ADD COLUMN IF NOT EXISTS llm_calls INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS llm_usage JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeline_runs
    DROP COLUMN IF EXISTS llm_calls,
    DROP COLUMN IF EXISTS input_tokens,
    DROP COLUMN IF EXISTS output_tokens,
    DROP COLUMN IF EXISTS llm_usage;
-- +goose StatementEnd
