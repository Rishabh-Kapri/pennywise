-- +goose Up
-- +goose StatementBegin
-- 00007 created auth_providers/google_provider_users with CREATE TABLE IF NOT
-- EXISTS, so databases where those tables already existed kept their old
-- constraints (e.g. UNIQUE(provider_id) alone). That breaks Android login: the
-- same Google account must be able to hold one row per oauth_client_type
-- (web + android). Reconcile such databases with the schema declared in 00007.
-- Every step is guarded, so this is a no-op on databases already in shape.
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
-- Constraint reconciliation toward the shape declared in 00007; nothing to undo.
SELECT 1;
-- +goose StatementEnd
