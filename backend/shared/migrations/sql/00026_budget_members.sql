-- +goose Up
-- A budget can have several people. budgets.user_id stays the owner (their AI
-- keys run the budget's pipeline); budget_members says who else may open it
-- and what they may do. Every budget's owner gets a row here, so the members
-- table alone answers "may this user open this budget".
CREATE TYPE budget_role AS ENUM ('owner', 'editor', 'viewer');

CREATE TABLE budget_members (
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    role budget_role NOT NULL,
    -- The budget this person last opened. Per member, not on budgets, so one
    -- person switching budgets doesn't switch the others'.
    is_selected BOOLEAN NOT NULL DEFAULT FALSE,
    invited_by UUID REFERENCES auth_users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (budget_id, user_id)
);
CREATE INDEX idx_budget_members_user ON budget_members (user_id);
CREATE UNIQUE INDEX idx_budget_members_one_owner ON budget_members (budget_id) WHERE role = 'owner';

-- budgets.is_selected is no longer read or written; its value moves here.
INSERT INTO budget_members (budget_id, user_id, role, is_selected)
SELECT id, user_id, 'owner', COALESCE(is_selected, FALSE)
FROM budgets
WHERE user_id IS NOT NULL;

-- An invitation to a budget, addressed to a Google email. The invitee sees it
-- after logging in with that email and accepts or declines it in the app.
CREATE TABLE budget_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    -- Stored lowercase so "Me@Gmail.com" matches the login "me@gmail.com".
    email TEXT NOT NULL CHECK (email = lower(email) AND email <> ''),
    role budget_role NOT NULL CHECK (role <> 'owner'),
    invited_by UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    -- At most one of these is set; all NULL = open (until expires_at).
    accepted_at TIMESTAMPTZ,
    accepted_by UUID REFERENCES auth_users(id) ON DELETE SET NULL,
    declined_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_budget_invites_open
    ON budget_invites (budget_id, email)
    WHERE accepted_at IS NULL AND declined_at IS NULL AND revoked_at IS NULL;
CREATE INDEX idx_budget_invites_email ON budget_invites (email)
    WHERE accepted_at IS NULL AND declined_at IS NULL AND revoked_at IS NULL;

-- +goose Down
-- Puts each owner's selection back on budgets before dropping the tables.
UPDATE budgets b SET is_selected = m.is_selected
FROM budget_members m
WHERE m.budget_id = b.id AND m.role = 'owner';
DROP TABLE budget_invites;
DROP TABLE budget_members;
DROP TYPE budget_role;
