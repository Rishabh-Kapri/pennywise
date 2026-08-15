-- +goose Up
-- +goose StatementBegin
-- The budget-wide document library (GET /api/documents) always scans by
-- budget with the soft-deleted rows excluded, then orders the join by the
-- transaction's date. The existing idx_transaction_documents_budget covers
-- only budget_id and includes deleted rows, so make the predicate partial and
-- carry created_at as the tiebreaker the library's ORDER BY uses.
CREATE INDEX IF NOT EXISTS idx_transaction_documents_budget_live
    ON transaction_documents(budget_id, created_at DESC) WHERE deleted = FALSE;

-- Superseded by the partial index above: every query that filtered on
-- budget_id alone also filters on deleted = FALSE.
DROP INDEX IF EXISTS idx_transaction_documents_budget;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_transaction_documents_budget
    ON transaction_documents(budget_id);

DROP INDEX IF EXISTS idx_transaction_documents_budget_live;
-- +goose StatementEnd
