-- +goose Up

-- Per-category, per-month budgeted / activity / available.
--
-- Driven by the union of (budget, category, month) keys present on either side
-- and left-joined to both, so a month with activity but no assigned budget and a
-- month with a budget but no activity are both represented. Transfers between
-- the user's own accounts (transfer_account_id) are not spending and are
-- excluded from activity.
--
-- security_invoker is required, not cosmetic: a Postgres view runs with the
-- view owner's permissions by default, which would bypass the row-level
-- security policies in 00009 entirely. With it, the view is evaluated as the
-- querying role and RLS applies.
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

-- +goose Down
DROP VIEW category_balances_by_month;
