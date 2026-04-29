-- +goose Up
-- +goose StatementBegin
CREATE TYPE auth_provider_type AS ENUM ('google');
CREATE TABLE IF NOT EXISTS auth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_user_id UUID NOT NULL REFERENCES auth_users(id), -- references the internal auth_users table
    provider_type auth_provider_type NOT NULL,
    provider_id TEXT NOT NULL UNIQUE, -- eg, google_id
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_providers_user_provider
    ON auth_providers(auth_user_id, provider_id);

CREATE TABLE IF NOT EXISTS google_provider_users (
    id TEXT PRIMARY KEY REFERENCES auth_providers(provider_id),
    name TEXT NOT NULL,
    picture TEXT,
    email TEXT NOT NULL,
    gmail_history_id NUMERIC(10, 0),
    refresh_token_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_gmail_sync TIMESTAMPTZ NOT NULL DEFAULT now(),
    expiry_at BIGINT,
    deleted BOOLEAN DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_google_provider_users_id
    ON google_provider_users(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS google_provider_users;
DROP TABLE IF EXISTS auth_providers;
DROP TYPE IF EXISTS auth_provider_type;
-- +goose StatementEnd
