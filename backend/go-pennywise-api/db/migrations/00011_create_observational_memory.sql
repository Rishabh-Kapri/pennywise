-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS observational_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sequence_start INT NOT NULL,
    sequence_end INT NOT NULL,
    observations JSONB NOT NULL DEFAULT '[]',
    current_task TEXT,
    suggested_response TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS observational_memory;
-- +goose StatementEnd
