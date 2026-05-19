-- +goose Up
-- +goose StatementBegin
CREATE TYPE auth_provider_type AS ENUM ('google');
CREATE TYPE oauth_type AS ENUM('web', 'android');
CREATE TABLE IF NOT EXISTS auth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_user_id UUID NOT NULL REFERENCES auth_users(id), -- references the internal auth_users table
    provider_type auth_provider_type NOT NULL,
    provider_id TEXT NOT NULL, -- eg, google_id
    oauth_client_type oauth_type NOT NULL DEFAULT 'web',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false,
    UNIQUE(provider_id, oauth_client_type),
    UNIQUE(provider_type, provider_id, oauth_client_type)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_providers_user_provider
    ON auth_providers(auth_user_id, provider_id, oauth_client_type);

CREATE TABLE IF NOT EXISTS google_provider_users (
    id TEXT NOT NULL,
    oauth_client_type oauth_type NOT NULL DEFAULT 'web',
    name TEXT NOT NULL,
    picture TEXT,
    email TEXT NOT NULL,
    gmail_history_id NUMERIC(10, 0),
    refresh_token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_gmail_sync TIMESTAMPTZ NOT NULL DEFAULT now(),
    expiry_at BIGINT,
    deleted BOOLEAN DEFAULT false,
    PRIMARY KEY(id, oauth_client_type),
    FOREIGN KEY (id, oauth_client_type) REFERENCES auth_providers(provider_id, oauth_client_type),
    UNIQUE(email, oauth_client_type)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_google_provider_users_id
    ON google_provider_users(id, oauth_client_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS google_provider_users;
DROP TABLE IF EXISTS auth_providers;
DROP TYPE IF EXISTS google_oauth_type;
DROP TYPE IF EXISTS auth_provider_type;
-- +goose StatementEnd
