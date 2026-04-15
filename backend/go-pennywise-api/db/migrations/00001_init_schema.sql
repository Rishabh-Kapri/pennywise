-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_id TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    picture TEXT,
    token_version INTEGER NOT NULL DEFAULT 1,
    refresh_token_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);

CREATE TABLE IF NOT EXISTS budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth_users(id),
    name TEXT NOT NULL,
    is_selected BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);

CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id),
    transfer_payee_id UUID,
    type TEXT NOT NULL,
    closed BOOLEAN DEFAULT false,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id),
    transfer_account_id UUID REFERENCES accounts(id),
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Adding this constraint directly. accounts had transfer_payee_id as a UUID.
ALTER TABLE accounts ADD CONSTRAINT fk_transfer_payee_id FOREIGN KEY (transfer_payee_id) REFERENCES payees(id);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id),
    email TEXT NOT NULL,
    history_id NUMERIC(10, 0) NOT NULL,
    gmail_refresh_token TEXT NOT NULL,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS category_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id),
    hidden BOOLEAN DEFAULT false,
    is_system BOOLEAN DEFAULT false,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id),
    category_group_id UUID NOT NULL REFERENCES category_groups(id),
    note TEXT,
    hidden BOOLEAN DEFAULT false,
    is_system BOOLEAN DEFAULT false,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS monthly_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    month TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    budgeted NUMERIC(12, 2) NOT NULL,
    carryover_balance NUMERIC(12, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id),
    date TEXT NOT NULL,
    payee_id UUID REFERENCES payees(id),
    category_id UUID REFERENCES categories(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    note TEXT,
    amount NUMERIC(12, 2) NOT NULL,
    source TEXT,
    transfer_account_id UUID REFERENCES accounts(id),
    transfer_transaction_id UUID REFERENCES transactions(id),
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS predictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    email_text TEXT,
    amount NUMERIC(12, 2),
    account TEXT,
    account_prediction NUMERIC(10, 2),
    payee TEXT,
    payee_prediction NUMERIC(10, 2),
    category TEXT,
    category_prediction NUMERIC(10, 2),
    has_user_corrected BOOLEAN,
    user_corrected_account TEXT,
    user_corrected_payee TEXT,
    user_corrected_category TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);

CREATE TABLE IF NOT EXISTS loan_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) UNIQUE,
    interest_rate NUMERIC(6, 3) NOT NULL,
    original_balance NUMERIC(12, 2) NOT NULL,
    monthly_payment NUMERIC(12, 2) NOT NULL,
    loan_start_date TEXT NOT NULL,
    category_id UUID REFERENCES categories(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);

CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id),
    color TEXT NOT NULL DEFAULT '',
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, budget_id)
);

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id TEXT NOT NULL,
    hashed_key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    user_id UUID NOT NULL REFERENCES auth_users(id),
    scopes TEXT[] NOT NULL,
    allowed_ips TEXT[],
    allowed_referrers TEXT[],
    rate_limit INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    rotation_enabled BOOLEAN,
    rotated_from_id UUID REFERENCES api_keys(id),
    rotation_due_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(key_id)
);

CREATE INDEX IF NOT EXISTS idx_auth_users_google_id ON auth_users(google_id);
CREATE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users(email);
CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_budget_date ON transactions(budget_id, date DESC, updated_at DESC) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_transactions_account ON transactions(account_id, budget_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id, budget_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_monthly_budgets_lookup ON monthly_budgets(budget_id, category_id, month);
CREATE INDEX IF NOT EXISTS idx_predictions_txn ON predictions(budget_id, transaction_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_payees_budget ON payees(budget_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_accounts_budget ON accounts(budget_id) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS idx_categories_budget ON categories(budget_id) WHERE deleted = FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS loan_metadata;
DROP TABLE IF EXISTS predictions;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS monthly_budgets;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS category_groups;
DROP TABLE IF EXISTS users;
-- Drop constraints to safely drop items
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS fk_transfer_payee_id;
DROP TABLE IF EXISTS payees;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS auth_users;
-- +goose StatementEnd
