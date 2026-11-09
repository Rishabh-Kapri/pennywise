-- +goose Up
CREATE TABLE auth_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_version INTEGER NOT NULL DEFAULT 1,
    refresh_token_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);

CREATE TYPE auth_provider_type AS ENUM ('google');
CREATE TYPE oauth_type AS ENUM ('web', 'android');

CREATE TABLE auth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_user_id UUID NOT NULL REFERENCES auth_users(id),
    provider_type auth_provider_type NOT NULL,
    provider_id TEXT NOT NULL, -- e.g. the google id
    oauth_client_type oauth_type NOT NULL DEFAULT 'web',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false,
    UNIQUE (provider_id, oauth_client_type),
    UNIQUE (provider_type, provider_id, oauth_client_type)
);
CREATE UNIQUE INDEX idx_auth_providers_user_provider
    ON auth_providers (auth_user_id, provider_id, oauth_client_type);

CREATE TABLE google_provider_users (
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
    PRIMARY KEY (id, oauth_client_type),
    FOREIGN KEY (id, oauth_client_type) REFERENCES auth_providers (provider_id, oauth_client_type),
    UNIQUE (email, oauth_client_type)
);
CREATE UNIQUE INDEX idx_google_provider_users_id
    ON google_provider_users (id, oauth_client_type);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id TEXT NOT NULL,
    hashed_key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL,
    allowed_ips TEXT[],
    allowed_referrers TEXT[],
    rate_limit INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    rotation_enabled BOOLEAN,
    rotated_from_id UUID REFERENCES api_keys(id),
    rotation_due_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (key_id)
);

-- Expo push tokens for the mobile app, keyed to the authenticated user.
-- One row per device token; re-registering the same token refreshes last_seen_at.
CREATE TABLE device_push_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    expo_push_token TEXT NOT NULL UNIQUE,
    platform TEXT NOT NULL DEFAULT 'android',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_device_push_tokens_user ON device_push_tokens (auth_user_id);

-- +goose Down
DROP TABLE device_push_tokens;
DROP TABLE api_keys;
DROP TABLE google_provider_users;
DROP TABLE auth_providers;
DROP TYPE oauth_type;
DROP TYPE auth_provider_type;
DROP TABLE auth_users;
