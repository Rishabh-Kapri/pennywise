-- Usage:
--   psql "$DATABASE_URL" -f backend/check_carryovers.sql
--
-- Optional: scope to one budget by uncommenting the budget_id filter below.

WITH active_categories AS (
  SELECT
    c.budget_id,
    c.id AS category_id,
    c.name
  FROM categories c
  WHERE c.deleted = FALSE
    AND c.is_system = FALSE
    -- AND c.budget_id = 'YOUR_BUDGET_ID'::uuid
),
budgeted_totals AS (
  SELECT
    mb.budget_id,
    mb.category_id,
    ROUND(SUM(mb.budgeted)::numeric, 2) AS total_budgeted
  FROM monthly_budgets mb
  GROUP BY mb.budget_id, mb.category_id
),
activity_totals AS (
  SELECT
    t.budget_id,
    t.category_id,
    ROUND(SUM(t.amount)::numeric, 2) AS total_activity
  FROM transactions t
  WHERE t.deleted = FALSE
    AND t.category_id IS NOT NULL
  GROUP BY t.budget_id, t.category_id
),
latest_stored AS (
  SELECT DISTINCT ON (mb.budget_id, mb.category_id)
    mb.budget_id,
    mb.category_id,
    mb.month,
    ROUND(mb.carryover_balance::numeric, 2) AS current_amount
  FROM monthly_budgets mb
  ORDER BY mb.budget_id, mb.category_id, mb.month DESC
),
results AS (
  SELECT
    c.budget_id,
    c.category_id,
    c.name AS category_name,
    ls.month AS current_month,
    COALESCE(ls.current_amount, 0)::numeric(12,2) AS current_amount,
    ROUND((
      COALESCE(bt.total_budgeted, 0) +
      COALESCE(at.total_activity, 0)
    )::numeric, 2) AS correct_amount
  FROM active_categories c
  LEFT JOIN budgeted_totals bt
    ON bt.budget_id = c.budget_id
   AND bt.category_id = c.category_id
  LEFT JOIN activity_totals at
    ON at.budget_id = c.budget_id
   AND at.category_id = c.category_id
  LEFT JOIN latest_stored ls
    ON ls.budget_id = c.budget_id
   AND ls.category_id = c.category_id
)
SELECT
  budget_id,
  category_id,
  category_name,
  current_month,
  current_amount,
  correct_amount,
  ROUND((current_amount - correct_amount)::numeric, 2) AS diff,
  (current_amount = correct_amount) AS is_correct
FROM results
ORDER BY is_correct, ABS(current_amount - correct_amount) DESC, category_name;
