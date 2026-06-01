-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS working_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL UNIQUE REFERENCES budgets(id) ON DELETE CASCADE,
    document JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS working_memory;
-- +goose StatementEnd
