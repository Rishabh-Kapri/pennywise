-- +goose Up
-- transactions.version: goes up by one on every write to the row (edits,
-- status moves, payee merges, soft delete). Clients send back the version they
-- read, as If-Match, so a save based on an old copy is refused instead of
-- silently overwriting a newer change. The adapters bump it in their SQL.
ALTER TABLE transactions ADD COLUMN version BIGINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE transactions DROP COLUMN version;
