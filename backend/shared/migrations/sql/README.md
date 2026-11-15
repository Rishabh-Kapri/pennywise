# Database migrations

Goose SQL files, embedded in `shared/migrations/sql` (`sql.Migrations`) and applied by
`go-pennywise-api/cmd/migrations`. The set is written as if for a brand-new
database: each file creates the final shape of one area, so the schema reads
top to bottom with no ALTER history to replay.

| File | Owns |
|---|---|
| `00001_extensions` | `vector`, `pg_trgm` |
| `00002_auth` | users, OAuth providers, API keys, push tokens |
| `00003_budgets` | budgets, accounts, payees, categories, goals, tags, loans, recurring |
| `00004_transactions` | transactions, documents, document blobs, statement imports |
| `00005_classification` | payee rules, predictions, embeddings (cipher) |
| `00006_pipeline_runs` | email pipeline run history |
| `00007_agent` | chat conversations, runs, memory |
| `00008_views` | `category_balances_by_month` |
| `00009_agent_readonly_rls` | `pennywise_agent_ro` role, grants, row-level security |
| `00010_budget_onboarding` | `budgets.onboarding` (wizard progress) |
| `00011_user_llm_providers` | users' own AI provider keys (encrypted); deliberately not granted to the agent role |
| `00012_api_key_ids` | real `api_keys.key_id` values, non-empty constraint |
| `00013_api_key_allowed_referrers` | renames the legacy `allowed_referers` column |
| `00014_mail_connections` | connected mailboxes for ingest (budgets, sealed credentials, cursor, watch, pause) |
| `00015_jobs` | job queue: `jobs` (also the outbox, with named queues), `job_events` history, `job_schedules` |
| `00016_google_provider_identity_only` | `google_provider_users`: drops the Gmail columns, which moved to `mail_connections` |
| `00017_user_llm_models` | AI models split from keys: `user_llm_models` (chat, decision or embedding; one default per kind; enable and order); drops `model` and `priority` from `user_llm_providers` |
| `00018_budget_pipeline_steps` | a budget's changes to the steps of the system pipelines (`shared/pipeline`), including fallback |
| `00019_transaction_sources` | `transaction_sources` (the emails/statement lines behind a transaction, and who wrote them) and `counterparty_aliases` (`CHANNEL \| MERCHANT` → payee); not granted to the agent role |
| `00020_llm_provider_typesafe` | adds `typesafe` to `llm_provider` (decision models only) |
| `00021_cipher_predictions_follow_deletes` | soft-deletes the cipher predictions of transactions already deleted |
| `00022_inbound_messages` | one row per bank message with its status (`needs_review`, `done`, ...), written when its run reports; backfilled from `pipeline.report` jobs; not granted to the agent role |
| `00023_statement_import_jobs` | `statement_imports.job_id`: the cipher job reading the statement |
| `00024_statement_import_files` | `statement_imports.file_path`: the uploaded PDF's key in the document store |
| `00025_account_statement_passwords` | `account_statement_passwords`: an account's statement PDF password, sealed; not granted to the agent role |
| `00026_budget_members` | `budget_members` (who may open a budget: owner/editor/viewer, and each member's selected budget; backfilled with every owner) and `budget_invites` (by email, 14-day expiry); not granted to the agent role |

## Commands (from `backend/go-pennywise-api`)

```bash
make migrate-up | migrate-down | migrate-status | migrate-version
make migrate-create name=add_foo   # next sequential file in this directory
```

## Writing a migration

- **Change to a table already in the schema: add a new numbered file** with the
  `ALTER`. Never edit an applied file; databases that already ran it will not
  see the change. A new area gets a new file with `CREATE` statements.
- Name it `NNNNN_snake_case.sql`, numbers sequential from 1 with no gaps. The
  test in this directory fails on gaps, duplicates and a missing Down section.
  After rebasing onto a branch that added migrations, renumber yours to the end.
- Every file has `-- +goose Up` and `-- +goose Down`. Down undoes exactly what
  Up did, in reverse dependency order.
- Plain `CREATE`, not `IF NOT EXISTS`: a migration runs once, and silently
  skipping an object that already exists hides drift.
- Wrap a statement in `-- +goose StatementBegin` / `StatementEnd` only if it
  contains a semicolon of its own (`DO $$ ... $$`, functions).
- Put each type next to the table that uses it, indexes right after their table,
  and the *why* in comments (the schema already says the *what*).
- A new table the agent may read must also be added to the table lists in
  `00009_agent_readonly_rls` (GRANT, policy loop, Down) or in a new RLS
  migration of its own. Forgetting leaves the agent with no access, not too much.
- The only `ALTER`s in the baseline are the `accounts` <-> `payees` foreign key
  (the two tables reference each other) and enabling RLS.
- `document_blobs.body` uses inline `STORAGE EXTERNAL`, which needs PostgreSQL 16+.

## Existing databases

Databases built by the old 29-file history keep `goose_db_version` rows for
versions 1..29. Run `make migrate-baseline` once per such database (it must be at
the old version 29): it rewrites that table to versions 1..9 without touching the
schema. Until then, a new migration numbered 10+ would be refused as "missing".
Delete the `baseline` command once every environment has been adopted.
