# Migration Improvements TODO

Remaining improvements for `backend/go-pennywise-api/cmd/migrations/main.go`.

## 5. insertCategories has a hidden side effect
`insertCategories` calls `insertMonthlyBudgets` inside its batch loop, creating a hidden
dependency. Monthly budget insertion should be separated into its own migration step, or at
minimum extracted outside the batch loop.

## 6. insertTransactions called alterTransactionsTable mid-batch (FIXED partially)
The ALTER TABLE call was moved to `createSchema()`, but the broader concern remains: FK
constraints are applied at schema creation time before data is loaded. If data has referential
integrity issues, inserts will fail. Consider deferring constraint creation to after data
migration, or using `SET CONSTRAINTS ... DEFERRED`.

## 7. Commented-out and dead code
- `createMonthlyBudgetsBackupTable` is only reachable via commented-out code in `main()`
- `getBalances` function is never called from anywhere
- `main()` has two commented-out calls (`run`, `createMonthlyBudgetsBackupTable`)
- Remove dead code; it's preserved in git history if needed

## 8. ~~No migration versioning or idempotency tracking~~ (FIXED)
A `schema_migrations` table now tracks applied versions. The `run()` loop checks
`isMigrationApplied()` before each step, skips already-applied migrations, and records
newly applied ones via `recordMigration()`. Re-runs are safe due to `ON CONFLICT DO NOTHING`
on both data inserts and the version record.

## 9. Typo: `unqiueMonthCat` in updateCarryover
Fixed to `uniqueMonthCat` in the current refactor, but verify no other typos exist.

## 10. updateCarryover is an N+1 query problem
For each unique (budget_id, category_id) pair, it queries all monthly budgets, then for each
month queries transactions individually. This is O(categories * months) database round trips.
Rewrite as a single SQL query with window functions (similar to what `getBalances` attempts),
or batch the transaction activity queries.

## 11. Month key manipulation is fragile in insertMonthlyBudgets
`insertMonthlyBudgets` splits on "-", parses the second part as float, adds 1, and reformats.
This is brittle: parsing a month as float and adding 1 will produce "13" for month "12".
Use `time.Parse` to properly handle date arithmetic.
