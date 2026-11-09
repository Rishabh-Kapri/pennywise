-- +goose Up
-- TypeSafe (Jev) keys: decision models only, called at POST /v1/systemone.
ALTER TYPE llm_provider ADD VALUE IF NOT EXISTS 'typesafe';

-- +goose Down
-- Postgres can't drop an enum value. An unused 'typesafe' label is harmless.
SELECT 1;
