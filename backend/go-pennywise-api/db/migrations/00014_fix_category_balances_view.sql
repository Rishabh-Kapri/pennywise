-- +goose Up
-- +goose StatementBegin
--
-- category_balances_by_month existed only in the live database, never in a
-- migration, and had four problems:
--
--  1. It joined monthly_budgets with an INNER JOIN, so the view was driven
--     entirely by rows in that table. A category with real spending in a month
--     but no assigned budget for it did not appear at all, silently
--     under-reporting spend; conversely every category carrying a budget row
--     showed up as though live, whether or not anything had happened in it.
--  2. The join was on category_id alone, ignoring budget_id, while projecting
--     categories.budget_id. Nothing enforced that the two agreed.
--  3. Activity counted transfers between the user's own accounts as spending.
--  4. hidden and is_system were dropped, so no consumer could tell an archived
--     or system category from an active one.
--
-- The rewrite drives from the union of (budget, category, month) keys present
-- on either side and left-joins both, so a month with activity but no budget
-- and a month with a budget but no activity are both represented.
--
-- security_invoker is required, not cosmetic: a Postgres view runs with the
-- view owner's permissions by default, which would bypass the row-level
-- security policies in the next migration entirely. With it, the view is
-- evaluated as the querying role and RLS applies.
--
-- DROP then CREATE, not CREATE OR REPLACE: the new definition inserts hidden and
-- is_system into the column list, and CREATE OR REPLACE VIEW can only append
-- columns, never reorder or remove them. Against the live database, which
-- already has the old view, a replace would fail outright.
DROP VIEW IF EXISTS category_balances_by_month;

CREATE VIEW category_balances_by_month
WITH (security_invoker = true) AS
WITH activity AS (
    SELECT
        t.budget_id,
        t.category_id,
        to_char(t.date::date, 'YYYY-MM') AS month,
        SUM(t.amount) AS activity
    FROM transactions t
    WHERE t.deleted = FALSE
      AND t.category_id IS NOT NULL
      AND t.transfer_account_id IS NULL
    GROUP BY t.budget_id, t.category_id, to_char(t.date::date, 'YYYY-MM')
),
keys AS (
    SELECT budget_id, category_id, month FROM monthly_budgets
    UNION
    SELECT budget_id, category_id, month FROM activity
)
SELECT
    c.id                        AS category_id,
    c.name                      AS category_name,
    c.budget_id,
    c.hidden,
    c.is_system,
    k.month,
    -- Left NULL rather than 0 when the category was never budgeted that month:
    -- carryover_balance is a stored running balance, not something derivable
    -- per month, and "never budgeted" is not the same as "nothing available".
    mb.carryover_balance        AS available_balance,
    COALESCE(mb.budgeted, 0)    AS budgeted,
    COALESCE(a.activity, 0)     AS monthly_activity
FROM keys k
JOIN categories c
    ON c.id = k.category_id
   AND c.budget_id = k.budget_id
   AND c.deleted = FALSE
LEFT JOIN monthly_budgets mb
    ON mb.category_id = k.category_id
   AND mb.budget_id = k.budget_id
   AND mb.month = k.month
LEFT JOIN activity a
    ON a.category_id = k.category_id
   AND a.budget_id = k.budget_id
   AND a.month = k.month;
-- +goose StatementEnd

-- +goose StatementBegin
-- Dropping the view drops its grants with it. On a first run the agent role does
-- not exist yet and this is a no-op; if the view is ever rebuilt after the RLS
-- migration, this keeps the agent's read access from silently disappearing.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pennywise_agent_ro') THEN
        GRANT SELECT ON category_balances_by_month TO pennywise_agent_ro;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- The activity CTE previously scanned the whole transactions table on every
-- query of the view, with no predicate to push down.
CREATE INDEX IF NOT EXISTS idx_transactions_budget_category_date
    ON transactions (budget_id, category_id, date)
    WHERE deleted = FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_transactions_budget_category_date;
-- +goose StatementEnd

-- +goose StatementBegin
-- Restore the previous definition. Note this reinstates the inner join and
-- drops security_invoker, so re-applying it while the RLS migration is in
-- place would let the view read across budgets.
DROP VIEW IF EXISTS category_balances_by_month;

CREATE VIEW category_balances_by_month AS
WITH monthly_activity AS (
    SELECT
        transactions.category_id,
        to_char(transactions.date::date::timestamp with time zone, 'YYYY-MM'::text) AS transaction_month,
        COALESCE(sum(transactions.amount), 0::numeric) AS activity
    FROM transactions
    WHERE transactions.deleted = false
    GROUP BY transactions.category_id, (to_char(transactions.date::date::timestamp with time zone, 'YYYY-MM'::text))
)
SELECT
    c.id AS category_id,
    c.name AS category_name,
    c.budget_id,
    mb.month,
    mb.carryover_balance AS available_balance,
    mb.budgeted,
    COALESCE(ma.activity, 0::numeric) AS monthly_activity
FROM categories c
JOIN monthly_budgets mb ON c.id = mb.category_id
LEFT JOIN monthly_activity ma ON c.id = ma.category_id AND mb.month = ma.transaction_month
WHERE c.deleted = false;
-- +goose StatementEnd
