-- +goose Up
CREATE TYPE transaction_status AS ENUM ('UNAPPROVED', 'APPROVED', 'REJECTED', 'MANUAL');

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    date TEXT NOT NULL,
    payee_id UUID REFERENCES payees(id) ON DELETE SET NULL,
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    amount NUMERIC(12, 2) NOT NULL,
    note TEXT,

    dedupe_hash VARCHAR(64),
    status transaction_status NOT NULL DEFAULT 'MANUAL',
    raw_bank_text TEXT,
    summary TEXT,

    transfer_account_id UUID REFERENCES accounts(id),
    transfer_transaction_id UUID REFERENCES transactions(id),
    tag_ids UUID[] DEFAULT '{}',

    location_lat DOUBLE PRECISION,
    location_lng DOUBLE PRECISION,
    location_name TEXT,
    location_source VARCHAR(10),

    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Partial: manual transactions have no dedupe_hash and must not collide.
CREATE UNIQUE INDEX idx_transactions_dedupe
    ON transactions (budget_id, dedupe_hash)
    WHERE dedupe_hash IS NOT NULL AND deleted = FALSE;
CREATE INDEX idx_transactions_budget_date
    ON transactions (budget_id, date DESC, updated_at DESC) WHERE deleted = FALSE;
CREATE INDEX idx_transactions_account
    ON transactions (account_id, budget_id) WHERE deleted = FALSE;
CREATE INDEX idx_transactions_category
    ON transactions (category_id, budget_id) WHERE deleted = FALSE;
-- Serves the per-category activity scan in category_balances_by_month.
CREATE INDEX idx_transactions_budget_category_date
    ON transactions (budget_id, category_id, date) WHERE deleted = FALSE;

-- Receipts/documents attached to transactions. The body lives in a Store
-- (document_blobs by default, or UPLOADS_DIR); storage_path is the store key.
CREATE TABLE transaction_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_transaction_documents_txn
    ON transaction_documents (transaction_id) WHERE deleted = FALSE;
-- The budget-wide document library (GET /api/documents) scans by budget with
-- soft-deleted rows excluded; created_at is the tiebreaker its ORDER BY uses.
CREATE INDEX idx_transaction_documents_budget_live
    ON transaction_documents (budget_id, created_at DESC) WHERE deleted = FALSE;

-- Receipt/document bodies, stored in Postgres rather than on a container volume
-- or in object storage.
--
-- `key` is the same relative path the other Store implementations hand out
-- (<budgetId>/<transactionId>/<Payee>_<YYYYMMDD>-<n><ext>), so
-- transaction_documents.storage_path stays meaningful whichever backend wrote
-- the row and a backend switch needs no data rewrite.
CREATE TABLE document_blobs (
    key        TEXT PRIMARY KEY,
    -- Anything over ~2kB is TOASTed out of the heap row anyway; EXTERNAL keeps
    -- that but skips compression. Receipts are PDFs and JPEGs, already
    -- compressed, so the default would burn CPU for no saving. Needs PG 16+.
    body       BYTEA STORAGE EXTERNAL NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- An uploaded bank/card statement and the rows extracted from it, reconciled
-- against the ledger on read. The PDF is not stored; its extracted text is, so
-- a failed extraction can be retried without the file or its password.
CREATE TABLE statement_imports (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id          UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    file_name          TEXT NOT NULL,
    file_hash          TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'processing',
    error              TEXT,
    page_count         INTEGER NOT NULL DEFAULT 0,
    pages_processed    INTEGER NOT NULL DEFAULT 0,
    default_account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
    sections           JSONB NOT NULL DEFAULT '[]'::jsonb,
    dropped_rows       INTEGER NOT NULL DEFAULT 0,
    source_text        TEXT NOT NULL DEFAULT '',
    deleted            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_statement_imports_budget
    ON statement_imports (budget_id, created_at DESC) WHERE deleted = FALSE;
CREATE INDEX idx_statement_imports_hash
    ON statement_imports (budget_id, file_hash) WHERE deleted = FALSE;

-- +goose Down
DROP TABLE statement_imports;
DROP TABLE document_blobs;
DROP TABLE transaction_documents;
DROP TABLE transactions;
DROP TYPE transaction_status;
