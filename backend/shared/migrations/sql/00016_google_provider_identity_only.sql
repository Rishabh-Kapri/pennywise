-- +goose Up
-- google_provider_users is login identity only. Gmail access lives in
-- mail_connections, so these columns have no reader left.
-- gmail_ingestion_paused was never in a migration: only databases older than
-- the baseline have it.
ALTER TABLE google_provider_users
    DROP COLUMN refresh_token,
    DROP COLUMN gmail_history_id,
    DROP COLUMN last_gmail_sync,
    DROP COLUMN expiry_at,
    DROP COLUMN IF EXISTS gmail_ingestion_paused;

-- +goose Down
-- Brings the columns back empty: the dropped tokens can't be restored.
ALTER TABLE google_provider_users
    ADD COLUMN refresh_token TEXT NOT NULL DEFAULT '',
    ADD COLUMN gmail_history_id NUMERIC(10, 0),
    ADD COLUMN last_gmail_sync TIMESTAMPTZ,
    ADD COLUMN expiry_at BIGINT;
