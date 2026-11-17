#!/usr/bin/env bash
# Check every category's monthly balance against its transactions, and with
# --fix, correct them. Without --fix nothing is written.
#
#   scripts/check-balances.sh [-b <budget>] [-c <category>] [--all] [--txns] [--fix [-y]]
#
#   -b, --budget     a budget's id or name (case-insensitive, exact)
#   -c, --category   a category's id, or part of its name (case-insensitive:
#                    "groceries" finds "🛒 Groceries"); every match is checked
#   --all            every month that's off, not just where each mismatch started
#   --txns           list the transactions in each month a mismatch started,
#                    deleted ones too, and mark the likely culprits
#   --fix            after the report, set every wrong balance to what its
#                    assignments and transactions add up to, add a missing
#                    month a category has transactions in, and drop the extra
#                    copies of a month that has two (when they agree on what
#                    was assigned; otherwise that category is left alone).
#                    Asks first, and saves an undo file. Only what -b and -c
#                    pick is touched.
#   -y, --yes        with --fix, don't ask
#
# A category's stored balance for a month (monthly_budgets.carryover_balance)
# should be everything assigned to it plus everything spent from it, in that
# month and every month before. The API keeps it up to date by adding each
# change to the stored figures, so a missed or doubled update stays wrong in
# every later month. This adds it all up again from scratch and compares.
#
# By default it prints only the month each mismatch started, which is where
# to look for the transaction or assignment that went wrong. --all prints
# every month that's off.
#
# Which database, in order:
#   1. $DATABASE_URL           any Postgres URL (psql runs inside the local container)
#   2. the local container     LOCAL_PG_CONTAINER, LOCAL_PG_USER, LOCAL_PG_DB
#
# Exits 1 when something is off, so it can run in a check; --fix exits 0 once
# everything it could fix is fixed.
set -euo pipefail

container="${LOCAL_PG_CONTAINER:-pennywise-local-pg}"
db_user="${LOCAL_PG_USER:-pennywise}"
db_name="${LOCAL_PG_DB:-pennywise}"
budget=""
category=""
all=false
txns=false
fix=false
yes=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    -b | --budget) budget="${2:?--budget needs an id or name}"; shift 2 ;;
    -c | --category) category="${2:?--category needs an id or name}"; shift 2 ;;
    --all) all=true; shift ;;
    --txns) txns=true; shift ;;
    --fix) fix=true; shift ;;
    -y | --yes) yes=true; shift ;;
    -h | --help) sed -n '2,39p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null | grep -q true \
  || { echo "error: container '$container' is not running" >&2; exit 2; }

psql_cmd=(docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 -v budget="$budget" -v category="$category")
if [[ -n "${DATABASE_URL:-}" ]]; then
  psql_cmd+=("$DATABASE_URL")
else
  psql_cmd+=(-U "$db_user" -d "$db_name")
fi

# One row per category and month that has an assignment or a transaction,
# with the expected running balance and what's stored.
read -r -d '' checked <<'SQL' || true
-- a temporary view: gone when the session ends, nothing is written
CREATE TEMP VIEW started AS
WITH budget_inflow AS (
    SELECT id AS budget_id, NULLIF(metadata->>'inflowCategoryId', '')::uuid AS inflow_id
    FROM budgets
    WHERE NOT COALESCE(deleted, false)
      AND (:'budget' = '' OR id::text = :'budget' OR lower(name) = lower(:'budget'))
),
activity AS (
    -- what the API adds to a balance: every categorised transaction, transfers
    -- included (a transfer to a tracking account can carry a category)
    SELECT t.budget_id, t.category_id,
           to_char(t.date::date, 'YYYY-MM') AS month,
           SUM(t.amount) AS activity,
           COUNT(*) AS txns
    FROM transactions t
    WHERE NOT t.deleted AND t.category_id IS NOT NULL
    GROUP BY 1, 2, 3
),
monthly AS (
    -- one row per category and month; a second one is a fault of its own,
    -- and the newest is the one the app most likely shows
    SELECT DISTINCT ON (budget_id, category_id, month)
           budget_id, category_id, month, id, budgeted, carryover_balance,
           COUNT(*) OVER (PARTITION BY budget_id, category_id, month) AS copies
    FROM monthly_budgets
    ORDER BY budget_id, category_id, month, updated_at DESC
),
keys AS (
    SELECT budget_id, category_id, month FROM monthly
    UNION
    SELECT budget_id, category_id, month FROM activity
),
months AS (
    SELECT k.budget_id, b.name AS budget_name, g.name AS group_name, c.name AS category,
           k.category_id, k.month,
           mb.id IS NOT NULL AS has_row,
           mb.id AS row_id,
           COALESCE(mb.copies, 0) AS copies,
           COALESCE(mb.budgeted, 0) AS budgeted,
           COALESCE(a.activity, 0) AS activity,
           COALESCE(a.txns, 0) AS txns,
           mb.carryover_balance AS stored
    FROM keys k
    JOIN budget_inflow bi ON bi.budget_id = k.budget_id
    JOIN budgets b ON b.id = k.budget_id
    JOIN categories c ON c.id = k.category_id AND NOT COALESCE(c.deleted, false)
     AND (:'category' = '' OR c.id::text = :'category'
          OR c.name ILIKE '%' || replace(replace(:'category', '%', '\%'), '_', '\_') || '%')
    LEFT JOIN category_groups g ON g.id = c.category_group_id
    LEFT JOIN monthly mb
           ON mb.budget_id = k.budget_id AND mb.category_id = k.category_id AND mb.month = k.month
    LEFT JOIN activity a
           ON a.budget_id = k.budget_id AND a.category_id = k.category_id AND a.month = k.month
    -- Ready to Assign has no stored balance: the API skips it
    WHERE k.category_id IS DISTINCT FROM bi.inflow_id
),
checked AS (
    SELECT *,
           ROUND(SUM(budgeted + activity) OVER w, 2) AS expected,
           ROUND(COALESCE(stored, 0) - SUM(budgeted + activity) OVER w, 2) AS off_by
    FROM months
    WINDOW w AS (PARTITION BY budget_id, category_id ORDER BY month)
),
started AS (
    -- how much more is off than the month before: non-zero where it went wrong
    SELECT *, off_by - COALESCE(LAG(off_by) OVER (PARTITION BY budget_id, category_id ORDER BY month), 0) AS new_error
    FROM checked
)
SELECT * FROM started;
SQL

if $all; then
  where="off_by <> 0 OR copies > 1 OR (NOT has_row AND activity <> 0)"
  title="Every month that is off"
else
  where="new_error <> 0 OR copies > 1 OR (NOT has_row AND activity <> 0)"
  title="Where each mismatch started"
fi

# The transactions in each month a mismatch started. Nothing records what
# changed a balance, so these are suspects, not proof: the culprit is usually
# one marked below. A transaction moved out to another category or month
# isn't listed here; look for it under the month it went to.
suspects=""
if $txns; then
  suspects=$(cat <<'SQL'
\echo
\echo 'Transactions in the month each mismatch started (likely culprits first)'
\echo 'Nothing records what changed a balance: these are suspects, not proof.'
WITH broken AS (
    SELECT * FROM started WHERE has_row AND (new_error <> 0 OR copies > 1)
),
listed AS (
    SELECT s.category, s.month, s.new_error,
           t.date::date AS day, t.amount, COALESCE(p.name, '') AS payee, a.name AS account,
           t.deleted, t.created_at, t.updated_at, t.id,
           COUNT(*) OVER (PARTITION BY s.category_id, s.month, t.date, t.amount, t.payee_id) AS same
    FROM broken s
    JOIN transactions t
      ON t.budget_id = s.budget_id AND t.category_id = s.category_id
     AND to_char(t.date::date, 'YYYY-MM') = s.month
    LEFT JOIN payees p ON p.id = t.payee_id
    LEFT JOIN accounts a ON a.id = t.account_id
),
marked AS (
    SELECT *, concat_ws(', ',
        CASE WHEN new_error <> 0 AND ABS(ABS(amount) - ABS(new_error)) < 0.005
             THEN 'same amount as the mismatch' END,
        CASE WHEN same > 1 THEN same || ' identical (date, amount, payee)' END,
        -- deleting doesn't set updated_at, so when it was deleted isn't known
        CASE WHEN deleted THEN 'deleted' END,
        CASE WHEN NOT deleted AND updated_at > created_at + interval '1 minute'
             THEN 'edited ' || to_char(updated_at, 'DD Mon YYYY') END
    ) AS why
    FROM listed
)
SELECT category, month,
       CASE WHEN new_error = 0 THEN 'duplicate month' ELSE new_error::text END AS mismatch,
       day, created_at::date AS added, amount, payee, account,
       CASE WHEN deleted THEN 'deleted' ELSE '' END AS state,
       why AS "why it's a suspect", id AS transaction
FROM marked
ORDER BY category, month, (why = '') , ABS(ABS(amount) - ABS(new_error)), day;
SQL
)
fi

out=$("${psql_cmd[@]}" <<SQL
$checked
SELECT format('%s categories, %s months checked', COUNT(DISTINCT category_id), COUNT(*)) FROM started;
\\echo
\\echo '$title (off by = stored minus expected; to fix, find what changed it that month)'
SELECT budget_name AS budget, COALESCE(group_name, '') AS "group", category, month,
       budgeted, activity, txns AS "txns",
       CASE WHEN has_row THEN stored::text ELSE 'no row' END AS stored,
       expected, off_by,
       CASE WHEN copies > 1 THEN copies || ' monthly rows for this month (newest shown)'
            WHEN NOT has_row THEN 'transactions but no monthly row'
            WHEN new_error <> 0 AND off_by = new_error THEN 'started here'
            WHEN new_error <> 0 THEN 'changed by ' || new_error
            ELSE '' END AS note
FROM started
WHERE $where
ORDER BY budget_name, group_name, category, month;
\\echo
SELECT format('%s categories off now, by %s in total',
              COUNT(*) FILTER (WHERE off_by <> 0),
              COALESCE(SUM(off_by) FILTER (WHERE off_by <> 0), 0))
FROM (SELECT DISTINCT ON (budget_id, category_id) * FROM started
      ORDER BY budget_id, category_id, month DESC) latest;
$suspects
SELECT 'mismatches: ' || COUNT(*) FROM started WHERE $where;
SQL
)
echo "$out"

if [[ -n "$budget$category" ]] && grep -q ' 0 categories, 0 months checked' <<<"$out"; then
  echo "error: nothing matches${budget:+ budget '$budget'}${category:+ category '$category'}" >&2
  exit 2
fi

grep -q 'mismatches: 0$' <<<"$out" && exit 0
$fix || exit 1

# ── --fix ─────────────────────────────────────────────────────────────────
# One database transaction: drop the extra copies of a month, then set every
# balance still off to the running total and add the missing months. Writes
# to both tables are held off meanwhile, so a change made by the app can't
# land between the sums and the update. Every row it changes or removes is
# printed first as SQL that puts it back, kept in the undo file.

target="${DATABASE_URL:+$(sed -E 's#//[^@]*@#//***@#' <<<"$DATABASE_URL")}"
target="${target:-$container / $db_name}"
echo
echo "--fix will correct the balances above in: $target"
if ! $yes; then
  read -r -p "Type 'fix' to change them: " answer
  [[ "$answer" == "fix" ]] || { echo "nothing changed"; exit 1; }
fi

undo="balance-fix-undo-$(date +%Y%m%d-%H%M%S).sql"
fixed=$("${psql_cmd[@]}" -At <<SQL
BEGIN;
LOCK TABLE monthly_budgets IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE transactions IN SHARE MODE;
$checked

-- A month with more than one row: keep the newest, the one the app shows,
-- when they all agree on what was assigned. When they don't, which one is
-- right is a person's call, so that category is left as it is.
CREATE TEMP TABLE copies AS
SELECT mb.*, ROW_NUMBER() OVER (PARTITION BY mb.budget_id, mb.category_id, mb.month
                                ORDER BY mb.updated_at DESC, mb.created_at DESC) AS n,
       MIN(mb.budgeted) OVER w <> MAX(mb.budgeted) OVER w AS disagree
FROM monthly_budgets mb
JOIN (SELECT DISTINCT budget_id, category_id FROM started) picked USING (budget_id, category_id)
WHERE (mb.budget_id, mb.category_id, mb.month) IN (
    SELECT budget_id, category_id, month FROM monthly_budgets
    GROUP BY 1, 2, 3 HAVING COUNT(*) > 1)
WINDOW w AS (PARTITION BY mb.budget_id, mb.category_id, mb.month);
CREATE TEMP TABLE left_alone AS
SELECT DISTINCT budget_id, category_id FROM copies WHERE disagree;

SELECT 'UNDO:' || format(
    'INSERT INTO monthly_budgets (id, month, budget_id, category_id, budgeted, carryover_balance, created_at, updated_at) VALUES (%L, %L, %L, %L, %s, %s, %L, %L);',
    id, month, budget_id, category_id, budgeted, carryover_balance, created_at, updated_at)
FROM copies WHERE n > 1 AND NOT disagree;
SELECT 'DROPPED:' || COUNT(*) FROM copies WHERE n > 1 AND NOT disagree;
DELETE FROM monthly_budgets WHERE id IN (SELECT id FROM copies WHERE n > 1 AND NOT disagree);

-- started is a view, so it now sums without the dropped copies
CREATE TEMP TABLE wrong AS
SELECT s.* FROM started s
WHERE ((s.has_row AND s.off_by <> 0) OR (NOT s.has_row AND s.activity <> 0))
  AND (s.budget_id, s.category_id) NOT IN (SELECT budget_id, category_id FROM left_alone);

SELECT 'UNDO:' || format('UPDATE monthly_budgets SET carryover_balance = %s WHERE id = %L;', stored, row_id)
FROM wrong WHERE has_row;
SELECT 'SET:' || COUNT(*) FROM wrong WHERE has_row;
UPDATE monthly_budgets mb SET carryover_balance = w.expected, updated_at = NOW()
FROM wrong w WHERE w.has_row AND mb.id = w.row_id;

SELECT 'ADDED:' || COUNT(*) FROM wrong WHERE NOT has_row;
INSERT INTO monthly_budgets (month, budget_id, category_id, budgeted, carryover_balance)
SELECT month, budget_id, category_id, 0, expected FROM wrong WHERE NOT has_row
RETURNING 'UNDO:' || format('DELETE FROM monthly_budgets WHERE id = %L;', id);

SELECT 'SKIPPED:' || format('%s, %s', b.name, c.name)
FROM left_alone la JOIN budgets b ON b.id = la.budget_id JOIN categories c ON c.id = la.category_id;
SELECT 'STILL OFF:' || COUNT(*) FROM started
WHERE (off_by <> 0 OR copies > 1 OR (NOT has_row AND activity <> 0))
  AND (budget_id, category_id) NOT IN (SELECT budget_id, category_id FROM left_alone);
COMMIT;
SQL
) || { echo "error: the fix failed and was rolled back; nothing changed" >&2; exit 1; }

{
  echo "-- Undo for check-balances.sh --fix, $(date -Iseconds), on $target"
  echo "-- Puts back every monthly_budgets row it changed, added or removed."
  echo "-- Changes the app made to those rows since are overwritten too."
  echo "BEGIN;"
  sed -n 's/^UNDO://p' <<<"$fixed"
  echo "COMMIT;"
} >"$undo"

val() { sed -n "s/^$1://p" <<<"$fixed" | head -1; }
echo
echo "Fixed:"
echo "  $(val SET) monthly balances set to what their transactions add up to"
echo "  $(val ADDED) missing months added"
echo "  $(val DROPPED) duplicate month rows removed"
skipped=$(sed -n 's/^SKIPPED://p' <<<"$fixed")
[[ -n "$skipped" ]] && echo "Left alone (its duplicate months disagree on what was assigned; decide which is right):" \
  && sed 's/^/  /' <<<"$skipped"
echo "Undo file: $undo"
if [[ -n "${DATABASE_URL:-}" ]]; then
  echo "  to undo: DATABASE_URL=... docker exec -i $container psql \"\$DATABASE_URL\" < $undo"
else
  echo "  to undo: docker exec -i $container psql -U $db_user -d $db_name < $undo"
fi
[[ "$(val 'STILL OFF')" == "0" ]] || { echo "error: $(val 'STILL OFF') still off after fixing" >&2; exit 1; }
exit 0
