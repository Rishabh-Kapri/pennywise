# AGENTS.md — go-pennywise-api

Core REST API for Pennywise (Gin + pgx/PostgreSQL). Layering: handler → service → repository. Repositories live in `backend/shared/db`, not here.

## Commands

```bash
make run              # go run ./cmd/api
make build            # ./bin/api
make test             # go test ./...
make check            # fmt + vet + test
make vendor           # GOWORK=off go mod vendor
make migrate-up       # also: migrate-one, migrate-down, migrate-redo, migrate-status, migrate-version
make migrate-create name=add_x # next migration file in backend/shared/sql/migrations
make migrate-baseline # one-off: adopt the squashed history on a DB built by the old 29 migrations
go test -run TestName ./internal/service
```

- `make vet` currently fails in `internal/temporal/activities` (test file passes `slog.Default()` where `*logger.ContextLogger` is expected). Pre-existing; don't mistake it for your change.

## Layout

- `cmd/api/main.go` — wiring: config, DB, Redis, Temporal (skipped when `RAILWAY_ENVIRONMENT_NAME=local` or no `TEMPORAL_SERVER_HOST`), all routes, background jobs.
- `cmd/migrations/` — goose runner (`up|down|status|version|redo|create|baseline`). The SQL lives in `backend/shared/sql/migrations`, embedded via `shared/sql`.
- `internal/handler/` — Gin handlers; parse request, call service, map errors to JSON.
- `internal/service/` — business logic, background jobs, Cipher/Gmail/Temporal clients wiring.
- `internal/middleware/` — API auth (`AuthMiddleware`), `RouteAuthMiddleware` (scopes), rate limit. Shared `BudgetIdMiddleware` and internal-request auth live in `backend/shared/middleware`.
- `internal/storage/` — document blob `Store`: `postgres.go` (default) and `local.go` (`DOCUMENT_STORAGE=local`, `UPLOADS_DIR`).
- `internal/client/` — outbound HTTP (Expo push).
- `internal/temporal/activities/` — activities registered on the API-hosted worker (`CreateTransaction`, `CreateCipherPrediction`, pipeline status, Gmail watch).
- `internal/websocket/` — budget-scoped hub; Redis stream listener rebroadcasts Cipher agent events.
- `internal/config/config.go` — all env parsing.
- `vendor/` — required for Docker builds (`go build -mod=vendor`).

## Conventions

- Budget scoping: `BudgetIdMiddleware` reads the `X-Budget-ID` header, checks ownership, and puts the id on the context. Services call `utils.MustBudgetID(ctx)` (`backend/shared/utils`). Handlers do not parse the header themselves.
- Errors: `c.JSON(status, gin.H{"error": err.Error()})`. Extra machine-readable codes go in a sibling `"code"` field (e.g. statements `password_required`, `no_text`). Map sentinel errors with `errors.Is` in the handler (see `handler/categoryGoal.go`).
- Repositories: use `*Tx` methods (e.g. `GetByIdTx(ctx, tx, budgetId, id)`) when a service operation must be atomic.
- Internal service calls: `InternalRequestAuth` verifies `X-Internal-Token` (`INTERNAL_AUTH_TOKEN`); only `VerifiedInternal` context may bypass user auth.
- Imports: stdlib / third-party / local, separated by blank lines.

## Gotchas

- **Migration versions must be sequential and unique.** goose panics on duplicates and blocks all pending migrations. When rebasing onto a branch that added migrations, renumber yours to the end. `cd backend/shared && go test ./sql` guards this. Changing the migrations means changing `shared`, so re-vendor (below). Authoring rules: `backend/shared/sql/README.md`.
- **Shared module changes:** `go.mod` uses `replace ... => ../shared` and `go.work` puts local builds in workspace mode. After any change under `backend/shared`, re-vendor (`make vendor` here, and in every consumer service) and verify with `GOWORK=off go build -mod=vendor ./cmd/api`. Docker only sees `shared` through `vendor/`.
- **Recurring transactions** materialize from an hourly `time.Ticker` in `cmd/api/main.go` (`RunDueAllBudgets`), not Temporal. Works without the Temporal worker.
- **Statement PDFs need `pdftotext`** (poppler). It is installed in the Docker image; a local run without it fails statement uploads. Binary path is the `pdftotextBinary` var in `statementText.go`.
- **Long-running work is async and single-flighted.** Returns `202`, clients poll:
  - Statement extraction: serialised per process via `extractSlot`; any import still `processing` at startup is marked failed (`FailInterrupted`). Retry via `POST /api/statements/:id/retry` (text is stored, not the PDF).
  - Learning backfill (`transactionService.BackfillLearning`): guarded by `learningBackfillRunning` (atomic CAS); progress is in `cipher_predictions.learned_at`/`learn_error`, not in the response.
- **User AI provider keys** (`/api/ai-providers`) are sealed with `LLM_CREDENTIALS_KEY` (AES-GCM, `shared/utils/secretbox.go`) and never returned; responses carry only `hasApiKey`/`apiKeyHint`. Cipher opens them with the same key, so the two services must share it. Without it, saving a provider returns 503. Demo users can't save providers.
- **Onboarding** state is `budgets.onboarding` (`/api/onboarding`). Steps are `model.OnboardingSteps`; keep the web app's `ONBOARDING_STEPS` in step. `POST /api/onboarding/email-backfill` needs an enabled AI provider (409 `ai_provider_required`) and calls go-gmail `/api/backfill`.
- **Go version:** `go.mod` says `go 1.26.0`, but the Dockerfile builds with `golang:1.25-alpine`. Keep them in agreement when touching either.

## Key files

| Purpose | Path |
|---|---|
| Routes and wiring | `cmd/api/main.go` |
| Config / env | `internal/config/config.go` |
| Budget context helpers | `backend/shared/utils/context.go` (`MustBudgetID`) |
| Budget middleware | `backend/shared/middleware/budget.go` |
| Internal auth middleware | `backend/shared/middleware/internalRequestAuth.go` |
| Transaction service (side effects, learning) | `internal/service/transaction.go` |
| Statement import pipeline | `internal/service/statement.go`, `statementReconcile.go`, `statementText.go` |
| Document storage | `internal/storage/postgres.go`, `internal/service/document.go` |
| Recurring scheduler | `internal/service/recurringTransaction.go` |
| Migrations | `backend/shared/sql/migrations/`, `backend/shared/sql/README.md`, `cmd/migrations/main.go` |
| Dockerfile | `Dockerfile` (vendored build, poppler-utils runtime) |
