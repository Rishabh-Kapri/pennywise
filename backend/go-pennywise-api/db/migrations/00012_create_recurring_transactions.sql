-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS recurring_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    payee_id UUID REFERENCES payees(id) ON DELETE SET NULL,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    amount NUMERIC(12, 2) NOT NULL,
    note TEXT,

    -- DAILY | WEEKLY | MONTHLY | YEARLY, repeated every interval_count periods
    frequency TEXT NOT NULL,
    interval_count INT NOT NULL DEFAULT 1,

    -- dates are TEXT 'YYYY-MM-DD' to match transactions.date
    next_date TEXT NOT NULL,
    end_date TEXT,
    last_run_date TEXT,

    paused BOOLEAN NOT NULL DEFAULT FALSE,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_recurring_transactions_budget
    ON recurring_transactions (budget_id) WHERE deleted = FALSE;

-- drives the "what is due now" scan across budgets
CREATE INDEX IF NOT EXISTS idx_recurring_transactions_due
    ON recurring_transactions (next_date) WHERE deleted = FALSE AND paused = FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS recurring_transactions;
-- +goose StatementEnd
