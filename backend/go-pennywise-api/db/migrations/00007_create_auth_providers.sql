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

-- The CREATE TABLE IF NOT EXISTS statements above are skipped when the tables
-- already exist, which can leave behind pre-oauth_client_type constraints
-- (e.g. UNIQUE(provider_id) alone). That breaks Android login: the same Google
-- account must hold one row per oauth_client_type. Reconcile any such tables
-- with the shape declared above; every step is guarded, so this is a no-op on
-- tables the statements above just created.
DO $$
DECLARE
    con record;
    pk_cols int;
BEGIN
    -- Old single-column FKs from google_provider_users depend on the old
    -- unique(provider_id); drop them before dropping that constraint.
    FOR con IN
        SELECT conname, array_length(conkey, 1) AS ncols
        FROM pg_constraint
        WHERE conrelid = 'google_provider_users'::regclass
          AND contype = 'f'
          AND confrelid = 'auth_providers'::regclass
    LOOP
        IF con.ncols = 1 THEN
            EXECUTE format('ALTER TABLE google_provider_users DROP CONSTRAINT %I', con.conname);
        END IF;
    END LOOP;

    -- Drop auth_providers unique constraints that don't include oauth_client_type.
    FOR con IN
        SELECT c.conname
        FROM pg_constraint c
        WHERE c.conrelid = 'auth_providers'::regclass
          AND c.contype = 'u'
          AND NOT EXISTS (
              SELECT 1
              FROM pg_attribute a
              WHERE a.attrelid = c.conrelid
                AND a.attnum = ANY (c.conkey)
                AND a.attname = 'oauth_client_type'
          )
    LOOP
        EXECUTE format('ALTER TABLE auth_providers DROP CONSTRAINT %I', con.conname);
    END LOOP;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'auth_providers'::regclass
          AND conname = 'auth_providers_provider_id_oauth_client_type_key'
    ) THEN
        ALTER TABLE auth_providers
            ADD CONSTRAINT auth_providers_provider_id_oauth_client_type_key
            UNIQUE (provider_id, oauth_client_type);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'auth_providers'::regclass
          AND conname = 'auth_providers_provider_type_provider_id_oauth_client_type_key'
    ) THEN
        ALTER TABLE auth_providers
            ADD CONSTRAINT auth_providers_provider_type_provider_id_oauth_client_type_key
            UNIQUE (provider_type, provider_id, oauth_client_type);
    END IF;

    -- google_provider_users: composite primary key, email uniqueness per client type.
    SELECT array_length(conkey, 1) INTO pk_cols
    FROM pg_constraint
    WHERE conrelid = 'google_provider_users'::regclass AND contype = 'p';

    IF pk_cols = 1 THEN
        ALTER TABLE google_provider_users DROP CONSTRAINT google_provider_users_pkey;
        ALTER TABLE google_provider_users ADD PRIMARY KEY (id, oauth_client_type);
    END IF;

    FOR con IN
        SELECT c.conname
        FROM pg_constraint c
        WHERE c.conrelid = 'google_provider_users'::regclass
          AND c.contype = 'u'
          AND NOT EXISTS (
              SELECT 1
              FROM pg_attribute a
              WHERE a.attrelid = c.conrelid
                AND a.attnum = ANY (c.conkey)
                AND a.attname = 'oauth_client_type'
          )
    LOOP
        EXECUTE format('ALTER TABLE google_provider_users DROP CONSTRAINT %I', con.conname);
    END LOOP;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'google_provider_users'::regclass
          AND conname = 'google_provider_users_email_oauth_client_type_key'
    ) THEN
        ALTER TABLE google_provider_users
            ADD CONSTRAINT google_provider_users_email_oauth_client_type_key
            UNIQUE (email, oauth_client_type);
    END IF;

    -- Recreate the FK against the composite key if no FK survived.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'google_provider_users'::regclass
          AND contype = 'f'
          AND confrelid = 'auth_providers'::regclass
    ) THEN
        ALTER TABLE google_provider_users
            ADD CONSTRAINT google_provider_users_id_oauth_client_type_fkey
            FOREIGN KEY (id, oauth_client_type)
            REFERENCES auth_providers (provider_id, oauth_client_type);
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS google_provider_users;
DROP TABLE IF EXISTS auth_providers;
DROP TYPE IF EXISTS google_oauth_type;
DROP TYPE IF EXISTS auth_provider_type;
-- +goose StatementEnd
