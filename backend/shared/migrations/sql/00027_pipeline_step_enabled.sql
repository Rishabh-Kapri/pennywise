-- +goose Up
-- A budget can turn off a step its pipeline file marks optional (message
-- verify, lookup and predict; statement fallback). NULL = on, the default;
-- only FALSE is ever stored.
ALTER TABLE budget_pipeline_steps ADD COLUMN enabled BOOLEAN;

-- +goose Down
ALTER TABLE budget_pipeline_steps DROP COLUMN enabled;
