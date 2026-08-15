-- +goose Up
-- +goose StatementBegin
-- 00001 declares transactions.status as NOT NULL DEFAULT 'MANUAL', but it does
-- so inside CREATE TABLE IF NOT EXISTS -- databases whose transactions table
-- predates that migration kept a nullable, default-less column, and every row
-- inserted without an explicit status (the 00003 JSON import) landed with
-- status NULL. Those rows list fine (GetAllNormalized coerces NULL to MANUAL)
-- but fail every single-row lookup, so editing one returns
-- TRANSACTION_LOOKUP_FAILED.
--
-- Backfill the NULLs with the value the column default would have given them,
-- then make the constraint match what 00001 intended. All three statements are
-- no-ops on a database that was created from 00001.
UPDATE transactions SET status = 'MANUAL' WHERE status IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE transactions ALTER COLUMN status SET DEFAULT 'MANUAL';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE transactions ALTER COLUMN status SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The backfilled values are indistinguishable from genuine MANUAL rows, so the
-- down migration only releases the constraint.
ALTER TABLE transactions ALTER COLUMN status DROP NOT NULL;
-- +goose StatementEnd
