-- +goose Up
-- +goose StatementBegin
-- Receipts/documents attached to transactions. The binary lives on disk
-- (UPLOADS_DIR); storage_path is relative to that root.
CREATE TABLE IF NOT EXISTS transaction_documents (
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

CREATE INDEX IF NOT EXISTS idx_transaction_documents_txn
    ON transaction_documents(transaction_id) WHERE deleted = FALSE;

CREATE INDEX IF NOT EXISTS idx_transaction_documents_budget
    ON transaction_documents(budget_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS transaction_documents;
-- +goose StatementEnd
