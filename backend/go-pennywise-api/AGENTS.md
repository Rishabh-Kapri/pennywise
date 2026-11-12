# AGENTS.md — go-pennywise-api

Core REST API for Pennywise (Gin + pgx/PostgreSQL). Layering: handler → service → repository. Repository ports live in `backend/shared/ports/repository` (Postgres adapter in `shared/adapters/postgres`), not here. Services depend on ports only; transactions go through the injected `repository.DBTxn`.

## Commands

```bash
make run              # go run ./cmd/api
make build            # ./bin/api
make test             # go test ./...
make check            # fmt + vet + test
make vendor           # GOWORK=off go mod vendor
make migrate-up       # also: migrate-one, migrate-down, migrate-redo, migrate-status, migrate-version
make migrate-create name=add_x # next migration file in backend/shared/migrations/sql
make migrate-baseline # one-off: adopt the squashed history on a DB built by the old 29 migrations
go test -run TestName ./internal/service
```

- `make vet` currently fails in `internal/temporal/activities` (test file passes `slog.Default()` where `*logger.ContextLogger` is expected). Pre-existing; don't mistake it for your change.

## Layout

- `cmd/api/main.go` — wiring: config, DB, Redis, all routes, background jobs.
- `cmd/migrations/` — goose runner (`up|down|status|version|redo|create|baseline`). The SQL lives in `backend/shared/migrations/sql`, embedded via that package.
- `internal/handler/` — Gin handlers; parse request, call service, map errors to JSON.
- `internal/service/` — business logic, background jobs, Cipher/Gmail/Temporal clients wiring.
- `internal/middleware/` — `RouteAuthMiddleware` (API key scopes), rate limit. User auth (JWT, API keys) is shared `middleware.Authenticate` built from `factory.NewAuthenticators`; it, `BudgetIdMiddleware` and internal-request auth live in `backend/shared/middleware`.
- `internal/storage/` — document blob `Store`: `postgres.go` (default) and `local.go` (`DOCUMENT_STORAGE=local`, `UPLOADS_DIR`).
- `internal/client/` — outbound HTTP (Expo push).
- `internal/temporal/activities/` — Temporal activities (`CreateTransaction`, `CreateCipherPrediction`, pipeline status, Gmail watch). Kept, but the API no longer connects to Temporal or runs a worker, so nothing registers them.
- `internal/worker/` — job worker for the message pipeline's last steps (`transaction.create`, `transaction.learn`, `pipeline.report`). Off without `JOB_DRIVER`. The logic is in `internal/service/messageJob.go`. `pipeline.report` saves the message's outcome to `inbound_messages` (`MessageJobService.RecordReport`), then queues learning; nothing writes `pipeline_runs` any more (the table and its old rows remain); `CreateTransaction` is still a `NOT_IMPLEMENTED` stub. `/api/jobs` (`service/job.go`) serves the Jobs screen, and needs a job store, so it reports `enabled: false` on the memory driver. It is budget-scoped: `mailbox.changed` jobs have no budget, so they're visible through the budget's mail connections or a job of the budget under them. Keep `ListTrees` and `jobService.visibleJob` agreeing on that rule.
- **Message status lives in `inbound_messages`, not in the job chain.** One row per message (budget + source + provider + external ID, so a continued or re-run message stays one row), created by its first report. Moves are `model.InboundMessageStatus.CanMoveTo`: a report moves only a `processing` row; continue (`needs_review`→`processing`), run again (`done`→`processing`) and dismiss (→`dismissed`) move it through `jobService.moveMessage` before touching jobs, and put it back if queueing fails. Don't decode `pipeline.report` payloads to learn a message's state; a message whose report predates its row gets one from that report on its first action (`jobService.inboundMessage`). `GET /api/messages?status=needs_review` lists them with per-status counts.
- `POST /api/jobs/:id/continue` queues a parked message's next job (`review.resume`, built by cipher, with its own `RunID`) and soft-deletes only the parked report. The message's dedupe hash is account + bank ref when verify read a ref, else the older account + date + amount + text hash; a ref message also checks the older hash so re-imports still match.
- Email and SMS of one payment: card alerts carry no ref, so `CreateTransaction` also matches a transaction with the same account and signed amount whose source from the other channel says the payment happened within 2 minutes (`otherChannelMatch`). A match adds this message as a second source and creates nothing; a missing time parks it as `needs_review` (step `create`, resume with `AllowDuplicate`). It runs under `LockForDedupe` and saves the source in the same database transaction as the create, so two messages arriving together can't both create. Source times are IST (cipher reads the bank's clock in Asia/Kolkata).
- Learning: a prediction becomes an alias on its own only above 0.8 confidence and with a real category (`learnable`); otherwise approving or editing the transaction teaches it (a MANUAL alias). `ClaimLearning` (`cipher_predictions.learned_at`) makes it happen once. Approving or correcting stamps `reviewed_at`; deleting a transaction soft-deletes its prediction. `metadata` holds each step's score and model.
- `GET /api/jobs/schedules` (`service/jobSchedule.go`) is read only: the budget's own schedules plus its mailboxes' renewals, which have no budget and are found by `model.MailboxWatchScheduleName`. Reports `enabled: false` without a schedule store.
- `/api/mail-connections` (`service/mailConnection.go`) forwards to ingest, which owns the Gmail adapter and the sealing key. It's off without `INGEST_SERVICE_URL`, and ingest must accept this service's token.
- Job updates: every service's broker publishes to the Redis channel `pennywise:job-updates` (`REDIS_URL`, set by `shared/factory.ConnectJob`); `service.JobUpdateForwarder` sends them to the budget's websocket clients. They're hints to reload, so they can be dropped.
- `internal/websocket/` — budget-scoped hub; Redis stream listener rebroadcasts Cipher agent events.
- `internal/config/config.go` — all env parsing.
- `vendor/` — required for Docker builds (`go build -mod=vendor`).

## Conventions

- Budget scoping: `BudgetIdMiddleware` reads the `X-Budget-ID` header, checks ownership, and puts the id on the context. Services call `utils.MustBudgetID(ctx)` (`backend/shared/utils`). Handlers do not parse the header themselves.
- Errors: `c.JSON(status, gin.H{"error": err.Error()})`. Extra machine-readable codes go in a sibling `"code"` field (e.g. statements `password_required`, `no_text`). Map sentinel errors with `errors.Is` in the handler (see `handler/categoryGoal.go`).
- Repositories: use `*Tx` methods (e.g. `GetByIdTx(ctx, tx, budgetId, id)`) when a service operation must be atomic.
- Internal service calls: `InternalRequestAuth` verifies `X-Internal-Token` against `SERVICE_TOKENS` (falls back to `INTERNAL_AUTH_TOKEN` when unset); only `VerifiedInternal` context may bypass user auth.
- Imports: stdlib / third-party / local, separated by blank lines.

## Gotchas

- **Migration versions must be sequential and unique.** goose panics on duplicates and blocks all pending migrations. When rebasing onto a branch that added migrations, renumber yours to the end. `cd backend/shared && go test ./migrations/sql` guards this. Changing the migrations means changing `shared`, so re-vendor (below). Authoring rules: `backend/shared/migrations/sql/README.md`.
- **Shared module changes:** `go.mod` uses `replace ... => ../shared` and `go.work` puts local builds in workspace mode. After any change under `backend/shared`, re-vendor (`make vendor` here, and in every consumer service) and verify with `GOWORK=off go build -mod=vendor ./cmd/api`. Docker only sees `shared` through `vendor/`.
- **Transaction status moves only through `UpdateStatus`.** Allowed moves are `model.TransactionStatus.CanMoveTo` (`UNAPPROVED`→`APPROVED`/`REJECTED`, `REJECTED`→`UNAPPROVED`); asking for the current status is a no-op, so approving twice can't queue learning twice. The repo's `TransitionStatus` writes only from the status the caller read and leaves `updated_at` alone (it orders the list and the page cursor). A transfer's other half moves with it. A refused move is `TRANSACTION_INVALID_STATUS`, 409.
- **Recurring transactions** materialize from an hourly `time.Ticker` in `cmd/api/main.go` (`RunDueAllBudgets`), not Temporal.
- **Statement PDFs need `pdftotext`** (poppler). It is installed in the Docker image; a local run without it fails statement uploads. Binary path is the `pdftotextBinary` var in `statementText.go`.
- **Long-running work is async and single-flighted.** Returns `202`, clients poll:
  - Statement extraction: serialised per process via `extractSlot`; any import still `processing` at startup is marked failed (`FailInterrupted`). Retry via `POST /api/statements/:id/retry` (text is stored, not the PDF).
  - Learning backfill (`transactionService.BackfillLearning`): guarded by `learningBackfillRunning` (atomic CAS); progress is in `cipher_predictions.learned_at`/`learn_error`, not in the response.
- **Login grants no Gmail access.** `google_provider_users` is identity only (migration 00016); `LoginWithGoogle` keeps no Google token and starts no watch. Mailboxes connect through `/api/mail-connections`. Android still requests `mail.google.com` at login, but nothing uses that grant. `POST /api/onboarding/email-backfill` returns `ErrNoGmailConnected` until ingest offers a backfill, and the Temporal Gmail activities are no-ops until Temporal is removed.
- **User AI keys** (`/api/ai-providers`, one per provider) are sealed with `LLM_CREDENTIALS_KEY` (AES-GCM, `shared/utils/secretbox.go`) and never returned; responses carry only `hasApiKey`/`apiKeyHint`. Cipher opens them with the same key, so the two services must share it. Without it, saving a key returns 503. Deleting a key deletes its models. **Models** (`/api/ai-models`) say which model runs with which key: kind `chat` or `decision` (decision models also carry the user's endpoint: https, or http only for a private address, because the key is sent there). A model is tested through cipher before it's created or changed; a failed test is 422 and saves nothing. Each kind has one default; deleting it promotes the first remaining one by priority. A model can be disabled without deleting it (`PUT /:id/enabled`), and `PUT /order` sets the order of a kind's models. A disabled default stays the default, and the next model by priority stands in until it's enabled. Demo users can't change keys or models.
- **Onboarding** state is `budgets.onboarding` (`/api/onboarding`). Steps are `model.OnboardingSteps`; keep the web app's `ONBOARDING_STEPS` in step. `POST /api/onboarding/email-backfill` needs a chat model on an enabled key (409 `ai_provider_required`).
- **SMS ingest is a decisions test harness** (`POST /api/sms/ingest`, `service/sms.go`). The Android app now posts to ingest's `POST /api/ingest/sms`; only older APKs call this: each SMS goes to cipher's `/api/test-decisions` and is appended to `SMS_DECISIONS_LOG` (default `./data/sms-decisions.jsonl`) with its cost and a running `totalCost` (re-summed from the file on first write after a restart). No dedupe: every re-import is billed again. Uses `DecisionsClient`, not `CipherClient`, so the CipherClient fakes don't grow.
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
| Migrations | `backend/shared/migrations/sql/`, its `README.md`, `cmd/migrations/main.go` |
| Dockerfile | `Dockerfile` (vendored build, poppler-utils runtime) |
