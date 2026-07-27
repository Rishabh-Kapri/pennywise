-- +goose Up
-- +goose StatementBegin
--
-- Least-privilege role for the agent's SQL tools.
--
-- Budget isolation for agent-generated SQL was previously prose in a system
-- prompt: nothing stopped a hallucinated or prompt-injected query from reading
-- another budget. This migration moves that boundary into Postgres.
--
-- pennywise_agent_ro is a NOLOGIN group role, so no credential is created here.
-- Grant it to a login role whose password you set out of band, and point
-- AGENT_DB_URL at that login role:
--
--   CREATE ROLE pennywise_agent LOGIN PASSWORD '<secret>';
--   GRANT pennywise_agent_ro TO pennywise_agent;
--
-- Policies below are scoped TO pennywise_agent_ro specifically, so the
-- application role is unaffected and keeps full access.
--
-- The migrating role usually lacks CREATEROLE, so this creates the role when it
-- can and otherwise fails with the exact SQL an administrator needs to run.
-- Failing loudly is deliberate: the policies below reference this role, and
-- silently skipping them would leave the agent with unrestricted reads while
-- appearing to have succeeded.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pennywise_agent_ro') THEN
        BEGIN
            CREATE ROLE pennywise_agent_ro NOLOGIN;
        EXCEPTION WHEN insufficient_privilege THEN
            RAISE EXCEPTION
                'role pennywise_agent_ro does not exist and this role cannot create it. '
                'Run as a superuser, then re-run this migration: '
                'CREATE ROLE pennywise_agent_ro NOLOGIN; '
                'CREATE ROLE pennywise_agent LOGIN PASSWORD ''<secret>''; '
                'GRANT pennywise_agent_ro TO pennywise_agent;';
        END;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- Read access to budget data only. Deliberately absent: auth_users, api_keys,
-- auth_providers, working_memory, observational_memory, conversations,
-- conversation_messages, agent_runs, and every embedding table. The agent has
-- no reason to read credentials, other users, or its own transcripts, and a
-- missing GRANT is a stronger guarantee than a policy.
GRANT USAGE ON SCHEMA public TO pennywise_agent_ro;

GRANT SELECT ON
    budgets,
    accounts,
    categories,
    category_groups,
    payees,
    tags,
    transactions,
    monthly_budgets,
    loan_metadata,
    category_balances_by_month
TO pennywise_agent_ro;
-- +goose StatementEnd

-- +goose StatementBegin
-- Row-level security keyed on app.budget_id, which the agent's tools set with
-- set_config(..., true) inside a read-only transaction. current_setting's
-- second argument returns NULL rather than erroring when the setting is
-- absent, so a connection that forgets to set it sees no rows at all — failing
-- closed instead of leaking.
DO $$
DECLARE
    target_table text;
BEGIN
    -- Tables carrying budget_id directly. budgets and loan_metadata are handled
    -- separately below: budgets keys on id, and loan_metadata has no budget_id
    -- at all, reaching its budget through accounts.
    FOREACH target_table IN ARRAY ARRAY[
        'accounts', 'categories', 'category_groups', 'payees', 'tags',
        'transactions', 'monthly_budgets'
    ]
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', target_table);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', target_table);
        EXECUTE format('DROP POLICY IF EXISTS agent_budget_isolation ON %I', target_table);
        EXECUTE format($f$
            CREATE POLICY agent_budget_isolation ON %I
            FOR SELECT TO pennywise_agent_ro
            USING (budget_id = current_setting('app.budget_id', true)::uuid)
        $f$, target_table);

        -- FORCE ROW LEVEL SECURITY applies to the table owner too, so without
        -- this the application would lose access to its own data.
        EXECUTE format('DROP POLICY IF EXISTS app_full_access ON %I', target_table);
        EXECUTE format($f$
            CREATE POLICY app_full_access ON %I
            FOR ALL TO PUBLIC
            USING (pg_has_role(current_user, 'pennywise_agent_ro', 'MEMBER') IS NOT TRUE)
            WITH CHECK (pg_has_role(current_user, 'pennywise_agent_ro', 'MEMBER') IS NOT TRUE)
        $f$, target_table);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- budgets keys on id rather than budget_id.
ALTER TABLE budgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE budgets FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_budget_isolation ON budgets;
CREATE POLICY agent_budget_isolation ON budgets
    FOR SELECT TO pennywise_agent_ro
    USING (id = current_setting('app.budget_id', true)::uuid);

DROP POLICY IF EXISTS app_full_access ON budgets;
CREATE POLICY app_full_access ON budgets
    FOR ALL TO PUBLIC
    USING (pg_has_role(current_user, 'pennywise_agent_ro', 'MEMBER') IS NOT TRUE)
    WITH CHECK (pg_has_role(current_user, 'pennywise_agent_ro', 'MEMBER') IS NOT TRUE);
-- +goose StatementEnd

-- +goose StatementBegin
-- loan_metadata has no budget_id column; it reaches its budget through accounts.
-- The accounts row referenced here is itself subject to the policy above, so a
-- loan whose account belongs to another budget is invisible twice over.
ALTER TABLE loan_metadata ENABLE ROW LEVEL SECURITY;
ALTER TABLE loan_metadata FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_budget_isolation ON loan_metadata;
CREATE POLICY agent_budget_isolation ON loan_metadata
    FOR SELECT TO pennywise_agent_ro
    USING (EXISTS (
        SELECT 1 FROM accounts a
        WHERE a.id = loan_metadata.account_id
          AND a.budget_id = current_setting('app.budget_id', true)::uuid
    ));

DROP POLICY IF EXISTS app_full_access ON loan_metadata;
CREATE POLICY app_full_access ON loan_metadata
    FOR ALL TO PUBLIC
    USING (pg_has_role(current_user, 'pennywise_agent_ro', 'MEMBER') IS NOT TRUE)
    WITH CHECK (pg_has_role(current_user, 'pennywise_agent_ro', 'MEMBER') IS NOT TRUE);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    target_table text;
BEGIN
    FOREACH target_table IN ARRAY ARRAY[
        'budgets', 'accounts', 'categories', 'category_groups', 'payees', 'tags',
        'transactions', 'monthly_budgets', 'loan_metadata'
    ]
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS agent_budget_isolation ON %I', target_table);
        EXECUTE format('DROP POLICY IF EXISTS app_full_access ON %I', target_table);
        EXECUTE format('ALTER TABLE %I NO FORCE ROW LEVEL SECURITY', target_table);
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', target_table);
    END LOOP;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pennywise_agent_ro') THEN
        REVOKE ALL ON
            budgets, accounts, categories, category_groups, payees, tags,
            transactions, monthly_budgets, loan_metadata, category_balances_by_month
        FROM pennywise_agent_ro;
        REVOKE USAGE ON SCHEMA public FROM pennywise_agent_ro;
    END IF;
END
$$;
-- +goose StatementEnd
