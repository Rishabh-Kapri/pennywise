# go-pennywise-api

Core REST API for Pennywise: auth, budgets, accounts, transactions, categories, payees, tags, loan metadata, predictions, agent runs, and websocket fanout. Built with Gin and PostgreSQL (pgx), layered as handler → service → repository (repositories live in [`backend/shared/db`](../shared)).

## Run

```bash
cp .env.example .env   # fill in real values as needed
make run               # or: go run ./cmd/api
```

Listens on `PORT` (default `5151`). Requires PostgreSQL with the pgvector extension; Redis and Temporal are optional (websocket agent streaming and workflow features degrade gracefully without them).

## Common commands

```bash
make build          # build ./bin/api
make test           # go test ./...
make check          # fmt + vet + test
make migrate-up     # apply pending migrations
make migrate-status # show migration state
make migrate-baseline # mark legacy seed migrations (1-6) as applied
```

On a **fresh database** the legacy Go seed migrations (00002/00003) fail; the workaround is `make migrate-up` (applies 00001, fails at 00002) → `make migrate-baseline` → `make migrate-up`. `CREATE EXTENSION vector` must exist before 00009.

## Routes

All routes are under `/api` and wired in `cmd/api/main.go`.

| Group | Auth | Notes |
|-------|------|-------|
| `POST /auth/google`, `POST /auth/refresh` | public | Google auth-code flow → 15-min access / 30-day refresh JWTs |
| `POST /auth/demo` | public, only when `DEMO_MODE=true` | logs into the seeded demo user (see below) |
| `GET /auth/users/me` | user | current user |
| `GET /auth/gmail`, `POST /auth/gmail/{pause,resume,sync,reconnect}` | user | mailbox controls across budgets; requests select `providerId` + `oauthClientType`; reconnect also requires a Google auth code |
| `/budgets`, `/keys` | user | global (not budget-scoped) resources |
| `/accounts`, `/transactions`, `/categories`, `/category-groups`, `/payees` (+ `/payees/:id/rules`), `/tags`, `/predictions` (+ `/predictions/cipher`), `/loan-metadata`, `/agent`, `/users` | user + budget | require `X-Budget-ID`; ownership enforced by `BudgetIdMiddleware` |
| `/ws` | user | websocket connect + session inspection |

`AuthMiddleware` accepts `Authorization: Bearer`, the `access_token` cookie, or `X-API-Key`. Internal service traffic is trusted only after the shared internal-request middleware verifies `X-Internal-Token` and marks the context `VerifiedInternal`.

Gmail controls require migration `00026`. Pausing persists across logins and disables scheduled watch renewal and new email imports; imports already running may finish. Resume renews the Gmail watch without replacing the saved ingestion cursor. Sync starts a manual Temporal workflow and returns `202` with `workflowId` and `runId`; progress and failed-import retries are available in Settings → Activity. Manual sync checks Gmail's retained history from the saved cursor, rather than importing the entire mailbox. Reconnecting uses the existing web Google auth-code flow, checks that the selected Google identity matches the owned connection, and preserves an intentional pause. Android credentials must be reauthorized through Android sign-in. Demo accounts cannot change Gmail controls.

Migration `00027` repairs legacy `last_gmail_sync` columns from `TIMESTAMP` to `TIMESTAMPTZ`, interpreting their existing wall-clock values as `Asia/Kolkata` (IST). Databases already using `TIMESTAMPTZ` are unchanged. Legacy databases populated in another timezone need that source timezone substituted before applying this repair. Watch expiry remains Unix milliseconds and does not need conversion.

## Demo mode

`DEMO_MODE=true` enables `POST /api/auth/demo`, which logs into a persistent demo user (`demo@pennywise.local`). On first login `internal/service/demo.go` + `demo_seed.go` seed — in a **single database transaction** (bulk `CopyFrom`, safe to interrupt, idempotent) — a full budget: categories, 3 accounts, payees, ~4 months of transactions (with linked credit-card transfer pairs and carryover balances), payee rules, and cipher predictions across all sources (RULE/VECTOR/LLM/UNCATEGORIZED, including user corrections).

## Temporal

`internal/temporal/activities` holds activities (e.g. create-prediction → create-transaction) executed by the [workflows worker](../workflows). Correlation metadata crosses workflow boundaries via `backend/shared/temporal/propagator.go`.

## Environment

See `.env.example`. Key variables: `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET`, `DOMAIN`, `INTERNAL_AUTH_TOKEN`, `REDIS_URL`, `CIPHER_SERVICE_URL`, `GMAIL_SERVICE_URL`, `TEMPORAL_SERVER_HOST`/`PORT`, `DEMO_MODE`, `PORT`.
