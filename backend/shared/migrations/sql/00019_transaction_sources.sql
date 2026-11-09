-- +goose Up
-- Evidence behind transactions, and the lookup table learned from it.
--
-- transaction_sources: every bank email, statement line or SMS that reported a
-- transaction, kept exactly as the bank sent it (raw_text) plus the few fields
-- lookup and reconciliation need. The transactions row stays the user's edited
-- result; a transaction can have several sources (the alert email and, later,
-- its statement line).
--
-- counterparty_aliases: "CHANNEL | MERCHANT" -> payee (+ usual category).
-- Exact lookup_key hits are the rule path; the optional embedding is the fuzzy
-- fallback. Replaces payee_rules and transaction_embeddings once the pipeline
-- reads from it.

CREATE TABLE transaction_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    -- NULL: a statement line no transaction matched yet (review pile).
    transaction_id UUID REFERENCES transactions(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('EMAIL', 'STATEMENT_LINE', 'SMS')),
    -- As the bank sent it: email body, statement narration/description cell.
    raw_text TEXT NOT NULL,

    -- Lookup key parts. NULL when the parser couldn't tell (credit alerts
    -- without a counterparty, non-bank receipts).
    channel TEXT CHECK (channel IN ('UPI', 'CARD', 'NEFT', 'IMPS', 'NACH')),
    merchant TEXT, -- upper case, single spaces, no trailing punctuation
    lookup_key TEXT GENERATED ALWAYS AS (channel || ' | ' || merchant) STORED,

    -- Matching fields: whatever this source stated, NULL if it didn't.
    vpa TEXT,
    ref TEXT,          -- UPI RRN, IMPS ref, NEFT UTR: unique per payment
    account_hint TEXT, -- trailing digits of the account or card
    direction TEXT CHECK (direction IN ('DEBIT', 'CREDIT')),
    amount NUMERIC(12, 2), -- unsigned, in currency
    currency CHAR(3),
    occurred_on DATE,
    occurred_at TIMESTAMPTZ, -- only when the source gave a time of day
    rewards_points INTEGER,
    statement_import_id UUID REFERENCES statement_imports(id) ON DELETE SET NULL,
    -- Who wrote the row. The backfill re-parses its own rows on every run, so
    -- it has to leave the ones the live pipeline wrote alone.
    origin TEXT NOT NULL DEFAULT 'BACKFILL' CHECK (origin IN ('BACKFILL', 'PIPELINE')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One email and one statement line per transaction; re-imports and
    -- re-runs of the backfill conflict here instead of duplicating.
    UNIQUE (transaction_id, kind)
);
CREATE INDEX idx_transaction_sources_lookup
    ON transaction_sources (budget_id, lookup_key) WHERE lookup_key IS NOT NULL;
CREATE INDEX idx_transaction_sources_ref
    ON transaction_sources (budget_id, ref) WHERE ref IS NOT NULL;
CREATE INDEX idx_transaction_sources_date
    ON transaction_sources (budget_id, occurred_on);
CREATE INDEX idx_transaction_sources_unmatched
    ON transaction_sources (budget_id, created_at DESC) WHERE transaction_id IS NULL;

CREATE TABLE counterparty_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    channel TEXT NOT NULL CHECK (channel IN ('UPI', 'CARD', 'NEFT', 'IMPS', 'NACH')),
    merchant TEXT NOT NULL,
    lookup_key TEXT GENERATED ALWAYS AS (channel || ' | ' || merchant) STORED,

    payee_id UUID NOT NULL REFERENCES payees(id) ON DELETE CASCADE,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,

    -- Embedding of lookup_key (bge-m3, local). NULL until embedded; exact
    -- lookup works without it.
    embedding vector(1024),

    source learning_source NOT NULL DEFAULT 'AUTO_LEARNED',
    hit_count INTEGER NOT NULL DEFAULT 1,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (budget_id, channel, merchant)
);
CREATE INDEX idx_counterparty_aliases_lookup ON counterparty_aliases (budget_id, lookup_key);
CREATE INDEX idx_counterparty_aliases_vector ON counterparty_aliases
    USING hnsw (embedding vector_cosine_ops);

-- +goose Down
DROP TABLE counterparty_aliases;
DROP TABLE transaction_sources;
