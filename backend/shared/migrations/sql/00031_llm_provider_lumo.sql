-- +goose Up
-- Lumo keys: a user's own lumo-tamer server (base URL, optional key), chat models only.
ALTER TYPE llm_provider ADD VALUE IF NOT EXISTS 'lumo';

-- +goose Down
-- Postgres can't drop an enum value. An unused 'lumo' label is harmless.
SELECT 1;
