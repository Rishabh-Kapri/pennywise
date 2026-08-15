-- +goose Up
-- +goose StatementBegin
-- Receipt/document bodies, stored in Postgres rather than on a container volume
-- or in object storage.
--
-- `key` is the same relative path the other Store implementations hand out
-- (<budgetId>/<transactionId>/<Payee>_<YYYYMMDD>-<n><ext>), so
-- transaction_documents.storage_path stays meaningful whichever backend wrote
-- the row and a backend switch needs no data rewrite.
CREATE TABLE IF NOT EXISTS document_blobs (
    key        TEXT PRIMARY KEY,
    body       BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Anything over ~2kB is TOASTed out of the heap row anyway; EXTERNAL keeps that
-- but skips the compression attempt. Receipts are PDFs and JPEGs, which are
-- already compressed, so the default would burn CPU on both write and read for
-- no meaningful saving.
ALTER TABLE document_blobs ALTER COLUMN body SET STORAGE EXTERNAL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS document_blobs;
-- +goose StatementEnd
