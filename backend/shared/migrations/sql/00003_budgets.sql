-- +goose Up
CREATE TABLE budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth_users(id),
    name TEXT NOT NULL,
    is_selected BOOLEAN,
    -- inflowCategoryId / startingBalPayeeId / ccGroupId
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);
CREATE INDEX idx_budgets_user_id ON budgets (user_id);

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    transfer_payee_id UUID, -- FK added below, once payees exists
    type TEXT NOT NULL,
    suffix TEXT, -- last 4 digits of the account number
    -- Which bank the account is with, as a logo id the clients know ("hdfc",
    -- "icici", ...). NULL = unknown; clients then fall back to the account name.
    bank TEXT,
    closed BOOLEAN DEFAULT false,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_accounts_budget ON accounts (budget_id) WHERE deleted = FALSE;

CREATE TABLE payees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    transfer_account_id UUID REFERENCES accounts(id),
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payees_budget ON payees (budget_id) WHERE deleted = FALSE;

-- accounts <-> payees reference each other, so one side has to be added after
-- both tables exist. This is the only ALTER TABLE in the schema besides RLS.
ALTER TABLE accounts
    ADD CONSTRAINT fk_transfer_payee_id FOREIGN KEY (transfer_payee_id) REFERENCES payees(id);

CREATE TABLE category_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    hidden BOOLEAN DEFAULT false,
    is_system BOOLEAN DEFAULT false,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    category_group_id UUID REFERENCES category_groups(id) ON DELETE SET NULL,
    -- Free-form note.
    note TEXT,
    -- What belongs in the category ("supermarket runs and household staples,
    -- not restaurants"). Distinct from note: meant to stay stable and can be
    -- AI-suggested.
    description TEXT NOT NULL DEFAULT '',
    hidden BOOLEAN DEFAULT false,
    is_system BOOLEAN DEFAULT false,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_categories_budget ON categories (budget_id) WHERE deleted = FALSE;
-- The fuzzy lookup is always budget-scoped and matches on name.
CREATE INDEX idx_categories_name_trgm ON categories USING gin (name gin_trgm_ops);

CREATE TABLE monthly_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    month TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    budgeted NUMERIC(12, 2) NOT NULL,
    carryover_balance NUMERIC(12, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_monthly_budgets_lookup ON monthly_budgets (budget_id, category_id, month);

-- A category's target (YNAB's "goal"): what the category needs funded and on
-- what schedule. At most one per category. Only the definition is stored; how
-- much a month is underfunded is derived from that month's budgeted/activity/
-- balance. Which columns matter depends on type (see model.CategoryGoal), and
-- the API clears the ones a type does not use.
CREATE TABLE category_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    category_id UUID NOT NULL UNIQUE REFERENCES categories(id) ON DELETE CASCADE,
    -- NEED: needed for spending; TB/TBD: savings balance (by date);
    -- MF: monthly savings builder; DEBT: monthly debt payment
    type TEXT NOT NULL CHECK (type IN ('NEED', 'TB', 'TBD', 'MF', 'DEBT')),
    target NUMERIC(12, 2) NOT NULL CHECK (target > 0),
    -- NEED only: how often it repeats ('none' = once, by target_month)
    cadence TEXT CHECK (cadence IN ('none', 'weekly', 'monthly', 'yearly')),
    -- every N months / years
    cadence_frequency INTEGER CHECK (cadence_frequency BETWEEN 1 AND 120),
    -- weekday (0 = Sunday) for weekly, else day of month; NULL = last day
    day SMALLINT CHECK (day BETWEEN 0 AND 31),
    -- NEED: true refills up to target, false sets aside another target
    needs_whole_amount BOOLEAN NOT NULL DEFAULT FALSE,
    -- YYYY-MM: TBD deadline, or first due month of a once/yearly/every-N target
    target_month TEXT CHECK (target_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    -- YYYY-MM: first month the target applies to
    creation_month TEXT NOT NULL CHECK (creation_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    -- YYYY-MM: the one month the target is skipped
    snoozed_month TEXT CHECK (snoozed_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (type = 'NEED' OR cadence IS NULL),
    CHECK (type <> 'NEED' OR cadence IS NOT NULL),
    CHECK (type <> 'TBD' OR target_month IS NOT NULL)
);
CREATE INDEX idx_category_goals_budget ON category_goals (budget_id);

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    color TEXT NOT NULL DEFAULT '',
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uniq_tags_budget_name_active ON tags (budget_id, name) WHERE deleted = false;

CREATE TABLE loan_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL UNIQUE REFERENCES accounts(id),
    interest_rate NUMERIC(6, 3) NOT NULL,
    original_balance NUMERIC(12, 2) NOT NULL,
    monthly_payment NUMERIC(12, 2) NOT NULL,
    loan_start_date TEXT NOT NULL,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);

CREATE TABLE recurring_transactions (
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
CREATE INDEX idx_recurring_transactions_budget
    ON recurring_transactions (budget_id) WHERE deleted = FALSE;
-- drives the "what is due now" scan across budgets
CREATE INDEX idx_recurring_transactions_due
    ON recurring_transactions (next_date) WHERE deleted = FALSE AND paused = FALSE;

-- +goose Down
DROP TABLE recurring_transactions;
DROP TABLE loan_metadata;
DROP TABLE tags;
DROP TABLE category_goals;
DROP TABLE monthly_budgets;
DROP TABLE categories;
DROP TABLE category_groups;
ALTER TABLE accounts DROP CONSTRAINT fk_transfer_payee_id;
DROP TABLE payees;
DROP TABLE accounts;
DROP TABLE budgets;
