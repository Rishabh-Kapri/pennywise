-- +goose Up
ALTER TABLE google_provider_users
    ADD COLUMN gmail_ingestion_paused BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE google_provider_users DROP COLUMN gmail_ingestion_paused;
