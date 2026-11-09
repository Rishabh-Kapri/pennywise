-- +goose Up
-- One row per connected mailbox (ingest). A mailbox is read once and its
-- messages go to every budget in budget_ids; an empty array means disabled
-- (the Gmail watch should be stopped). Login identity stays in
-- google_provider_users; this table is only "what Pennywise may read".
CREATE TABLE mail_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,                       -- gmail
    -- Stored lowercase so a push for "Me@Gmail.com" finds "me@gmail.com".
    account TEXT NOT NULL CHECK (account = lower(account)),
    -- No foreign key: Postgres can't enforce one on array elements, and
    -- budgets are soft-deleted. Readers skip budgets that no longer exist.
    budget_ids UUID[] NOT NULL DEFAULT '{}',
    -- Sealed by the provider adapter (MAIL_CREDENTIALS_KEY); opaque here.
    credentials BYTEA NOT NULL,
    -- The provider's position (Gmail history ID); opaque here. '' = not yet watched.
    sync_cursor TEXT NOT NULL DEFAULT '',
    watch_expires_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'needs_reconnect', 'error')),
    last_sync_at TIMESTAMPTZ,
    last_error TEXT,
    -- A user's choice, kept apart from status, which sync rewrites on every run.
    -- NULL = syncing.
    paused_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
-- A push names the mailbox; this finds its connection.
CREATE UNIQUE INDEX idx_mail_connections_account
    ON mail_connections (provider, account) WHERE deleted_at IS NULL;
-- "Which mailboxes feed this budget?" ($1 = ANY(budget_ids)).
CREATE INDEX idx_mail_connections_budgets ON mail_connections USING GIN (budget_ids);
-- Watch renewal: enabled connections whose watch runs out soon.
CREATE INDEX idx_mail_connections_watch
    ON mail_connections (watch_expires_at)
    WHERE deleted_at IS NULL AND budget_ids <> '{}';

-- +goose Down
DROP TABLE mail_connections;
