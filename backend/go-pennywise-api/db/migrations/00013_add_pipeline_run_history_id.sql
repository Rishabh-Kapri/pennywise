-- +goose Up
-- +goose StatementBegin
-- Gmail history id that triggered the run. Recorded at run creation so the
-- Activity page can show it before any email is parsed.
ALTER TABLE pipeline_runs ADD COLUMN IF NOT EXISTS gmail_history_id NUMERIC(20, 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeline_runs DROP COLUMN IF EXISTS gmail_history_id;
-- +goose StatementEnd
