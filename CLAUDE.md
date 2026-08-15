# CLAUDE.md

## Project Overview

Pennywise is a personal finance/budgeting app with AI-powered transaction classification from email parsing. Monorepo services (each directory has its own README.md):

- **go-pennywise-api** (`backend/go-pennywise-api`): Core REST API (Gin, PostgreSQL/pgx). Handler → service → repository layers (repos in `backend/shared/db`).
- **cipher** (`backend/cipher`): Classification pipeline (Ollama extraction → payee rules → pgvector → LLM fallback) + budget agent runtime (`agent/`).
- **go-gmail** (`backend/go-gmail`): Gmail Pub/Sub watcher, parses bank emails with regex, starts Temporal ingestion workflows.
- **workflows** (`backend/workflows`): Temporal worker + workflow definitions for email → transaction ingestion.
- **shared** (`backend/shared`): Shared Go module: repositories, models, transport, middleware, logging.
- **react-frontend** (`react-frontend`): React 19 + Vite + Redux Toolkit (active development).
- **android-frontend** (`android-frontend`): Expo React Native client mirroring react-frontend.
- **frontend** (`frontend`): Angular 17 + NGXS state management (legacy/maintenance).
- **python-mlp** (`backend/python-mlp`): Deprecated former prediction service, replaced by cipher.
- **file-parser** (`backend/file-parser`): Experimental Clojure service for bulk transaction uploads.

## Build & Test Commands

```bash
# Go API
cd backend/go-pennywise-api && go build ./cmd/api
cd backend/go-pennywise-api && go test ./...
cd backend/go-pennywise-api && go test -run TestName ./internal/service
cd backend/go-pennywise-api && go fmt ./... && go vet ./...

# Go Gmail
cd backend/go-gmail && go build ./cmd
cd backend/go-gmail && go test ./...
cd backend/go-gmail && go test -run TestName ./pkg/parser
cd backend/go-gmail && go fmt ./... && go vet ./...

# Cipher / Workflows / Shared
cd backend/cipher && go build ./cmd/api && go test ./...
cd backend/workflows && go build ./cmd/worker && go test ./...
cd backend/shared && go test ./...

# React Frontend (no test suite; build type-checks)
cd react-frontend && npm run build
cd react-frontend && npm run lint

# Angular Frontend
cd frontend && npm run build
cd frontend && npm test
cd frontend && npx ng test --include="**/name.spec.ts"

# Full Stack
docker-compose up --build
```

### After changing `backend/shared`

`backend/go.work` puts local builds in workspace mode, where `shared` resolves from
source and each service's `vendor/` is ignored. Docker and CI build each service in
isolation with `go build -mod=vendor`, and `shared` reaches the image **only** through
`vendor/` — so a local build passing proves nothing about the image building.

Re-vendor every consumer after any change under `backend/shared`, and verify the way
Docker does:

```bash
cd backend
for d in cipher go-pennywise-api go-gmail workflows; do (cd $d && GOWORK=off go mod vendor); done
(cd cipher && GOWORK=off go build -mod=vendor ./cmd/api)
(cd go-pennywise-api && GOWORK=off go build -mod=vendor ./cmd/api)
(cd go-gmail && GOWORK=off go build -mod=vendor -o /tmp/gmail ./cmd)
(cd workflows && GOWORK=off go build -mod=vendor ./cmd/worker)
```

## Key Data Flow

1. Gmail push → `go-gmail` starts Temporal `EmailToTransactionWorkflow` with a deterministic ID (`email-to-txn-<email>-<historyId>`, duplicate pushes dedupe at Temporal) and its `FetchEmailData` activity returns raw email bodies → cipher processes **one email per activity** (`ParseEmail` extracts via local LLM, `PredictEmail` classifies; Ollama — the old regex parser in `pkg/parser/email.go` is deprecated). Non-transaction emails / unknown accounts are recorded skips; infrastructure failures retry per email, and only still-failed emails park on the retry signals (`retry-email-parse`/`retry-predict`, cipher `POST /api/workflows/:workflowId/retry-*`). Each round's successes are committed immediately via `go-pennywise-api`'s `CreateTransactionAndCipherPrediction`, whose insert is deduped on `(budget_id, dedupe_hash)` — retries never duplicate transactions. Cipher activities heartbeat before every LLM step (`internal/progress`); each LLM call is bounded by `CIPHER_LLM_CALL_TIMEOUT` (default 3m). The pipeline's LLM steps (`parse:extract`, `predict:summarize`, `predict:llm_fallback`) run through an ordered provider chain set by `EMAIL_PIPELINE_PROVIDERS` (e.g. `ollama=gemma4:12b,openrouter=google/gemini-2.5-flash`, default `ollama=gemma4:12b`): each target is tried in turn and any failure falls through to the next, so an Ollama outage no longer stalls ingestion (`predictionService.chatWithFallback`). Embeddings have their own chain, `EMAIL_EMBEDDING_PROVIDERS` (same syntax, default `ollama=bge-m3`, `predictionService.embedWithFallback`) — every target must serve the **same** model (bge-m3, 1024 dims; OpenRouter serves it as `baai/bge-m3`), since a different model would put query vectors in a different space and invalidate every stored pgvector row. Matches record the `embedding_provider`/`embedding_model` that produced the query vector. Legacy batch activities (`ParseEmailData`/`Predict`) remain registered for pre-rollout workflows (`workflow.GetVersion` gate "per-email-pipeline").
2. All API calls require `X-Budget-ID` header — extracted via `utils.GetBudgetId(c)` in handlers
3. Internal service calls use shared request metadata headers (`X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, `X-Internal-Token`) and are trusted only after shared internal-request verification marks context as verified.
4. Temporal workflow/activity hops propagate `correlation_id` and `origin_service` through `backend/shared/temporal/propagator.go`; each activity restamps its local service name before downstream HTTP calls.
5. ML prediction corrections tracked in `internal/service/transaction.go` (`UserCorrectedPayee`, `UserCorrectedCategory`, etc.) The review queue (`/api/predictions/review`) surfaces cipher predictions least-confident-first; correcting one goes through `transactionService.Update`, which marks the cipher prediction corrected and queues payee-rule learning.
6. Transaction geolocation: transactions carry nullable `location_lat/lng/name/source` (source `auto`|`manual`). `PATCH /api/transactions/:id/location` sets/clears just the location (null lat+lng clears); missing place names are reverse-geocoded server-side via Nominatim (`internal/service/geocode.go`, best-effort, 1 req/s + in-process cache, `NOMINATIM_URL` env). The pipeline's `CreateTransactionAndCipherPrediction` sends a best-effort Expo push (`internal/client/expopush.go`, tokens in `device_push_tokens` registered via `POST /api/devices/push-token`) for newly created rows only; the Android app's background notification task (`android-frontend/src/features/notifications/locationSnapTask.ts`) then attaches the phone's last-known location with source `auto`; when the fix is stale (>15 min) or permission is missing it instead queues a "tag location?" prompt (AsyncStorage, 1h TTL) that fires on notification tap / app foreground and saves a fresh fix as source `manual`. Android FCM credentials live in `android-frontend/google-services.json`, which is **gitignored** (public repo) and resolved by `android-frontend/app.config.ts` as `process.env.GOOGLE_SERVICES_JSON ?? './google-services.json'` — local builds read the untracked file, EAS builds get it from a file-type env var (`GOOGLE_SERVICES_JSON`, see `android-frontend/README.md`). The API sends via Expo's push service, so it needs no FCM server key.
7. Receipts/documents: `transaction_documents` rows + bodies in Postgres (`document_blobs`, migration `00019`). `internal/storage` has two `Store` implementations sharing one key scheme: `postgres.go` (default; `bytea`, `SET STORAGE EXTERNAL` so already-compressed PDFs/JPEGs skip a pointless compression pass) and `local.go` (disk under `UPLOADS_DIR`, opt in with `DOCUMENT_STORAGE=local`). Keeping bodies in the database means one `pg_dump` backs everything up and a body cannot outlive its row; the cost is database size, which is why the 10MB/40MB upload caps matter. Keys are `<budgetId>/<transactionId>/<Payee>_<YYYYMMDD>-<n><ext>` (e.g. `McDonalds_20260819-1.pdf`) — the index is always present so adding a second document never renames the first, and the transaction segment stops two same-payee same-day transactions colliding. `POST/GET /api/transactions/:id/documents` (multipart field `file`, 10MB cap, content-sniffed mime whitelist: jpeg/png/webp/heic/pdf), `GET /api/documents/:docId/content` (authenticated streaming; frontends fetch with auth headers → blob/object URL since `<img src>` can't carry Authorization), `DELETE /api/documents/:docId` (soft-delete + file removal). Multi-page scanning: the Android app uses ML Kit's document scanner (`react-native-document-scanner-plugin` — edge detection, perspective correction, enhance filters; needs a prebuilt dev client, not Expo Go) and posts the pages to `POST /api/transactions/:id/documents/scan` (repeated `pages` parts, ≤20 pages, 40MB total), which merges them into one A4 PDF via `internal/service/documentScan.go` and stores it as a single document. The React panel exposes the same endpoint as "Combine images to PDF".
8. Pipeline observability: the email workflows report progress via `StartPipelineRun`/`ReportPipelineStatus` activities (hosted by the go-pennywise-api worker) into `pipeline_runs` + `pipeline_run_events`; the fetch step emits one `fetch_emails` event per email carrying `from`/`subject`/`snippet` (captured from Gmail headers + snippet in `FetchEmailData`) so the Activity page can show sender/preview/message id before extraction runs, the run row stores the triggering `gmail_history_id`, and the per-email loop emits one timeline event per email (parse/predict succeeded/skipped/failed, keyed by `messageId`) and each run change is broadcast budget-wide as the `pennywise::pipeline::update` websocket event (`sharedModel.EventPipelineUpdate`). The React `/activity` page lists runs, shows per-email extraction/prediction detail, and can retry parked workflows through `POST /api/pipeline/runs/:id/retry` (signals Temporal directly, choosing `retry-email-parse` vs `retry-predict` from the run's `current_step`). Reporting is best-effort — status write failures never fail the pipeline — and is gated by `workflow.GetVersion` marker "pipeline-observability" so pre-rollout in-flight workflows replay unchanged.
9. Recurring transactions materialize from their `next_date` via an hourly ticker in `cmd/api/main.go` (`RunDueAllBudgets`), not Temporal — the Temporal worker is skipped in local/Docker-only setups. Each occurrence advances the rule immediately, so restarts and missed ticks catch up without duplicating.

## Chat Agent (cipher `agent/`)

The chat agent is a tool-calling loop (`agent/runtime/agent.go`) over a multi-provider
LLM client, streaming deltas to the React panel via Redis.

- **Tool failures are recoverable.** A failed tool returns an `IsError` tool result to the
  model rather than being dropped — an unanswered `tool_use` block is rejected by the
  provider on the next turn, and dropping it also denies the model any chance to correct a
  bad query. Budget exhaustion (`AGENT_MAX_TURNS`, `AGENT_MAX_TOOL_CALLS`) takes a final
  tools-disabled turn instead of surfacing an error.
- **Tool selection.** Prefer purpose-built tools over `execute_sql`: `get_spending_summary`
  (totals by category/payee/tag), `get_top_transactions` (bounded detail), `get_budget_info`
  (which entities exist). `execute_sql` is the documented fallback. `get_schema` returns the
  query rules those queries must follow — keep its table map in sync with the migrations.
- **Tool arguments are decoded strictly** via `decodeToolArgs` (`DisallowUnknownFields`).
  `encoding/json` drops unknown keys by default, which turns "a filter this tool doesn't
  implement" into an unfiltered answer presented as a filtered one. Rejecting instead produces
  an `IsError` result the model can correct from.
- **Filtered queries are built with squirrel**, matching `shared/db` (`sq` alias, the shared
  `psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)` in `agent/tools/query.go`). The
  filter vocabulary is defined once as `entityFilters` (`categoryName`, `payeeName`, `tagName`)
  and shared by `get_spending_summary` and `get_top_transactions`, so the two cannot drift —
  a filter honored by one and dropped by the other is a silently wrong answer. Add new filters
  to `entityFilters.apply`, not to an individual tool. `get_spending_summary` must apply the
  same scope to its **total** query as to its grouping queries, or the model reports a filtered
  breakdown against an unfiltered denominator.
- **Tags** are a `UUID[]` column on `transactions` (`tag_ids`), not a join table. Join with
  `tg.id = ANY(t.tag_ids)`; any-of is the `&&` overlap operator.
- **Budget isolation is enforced by Postgres**, not the prompt. Read-only tools run through
  `tools.withBudgetScopedTx`, which opens a read-only transaction and sets `app.budget_id`;
  RLS policies (migration `00015`) key off it. `AGENT_DB_URL` must point at a login role
  granted `pennywise_agent_ro` or the isolation is inactive (cipher warns at startup).
  `category_balances_by_month` is `security_invoker` — without that a view bypasses RLS.
- **Prompt caching.** The system prompt is split: `SystemPromptStatic` is byte-identical
  across requests and carries the cache breakpoint, `SystemPromptDynamic` holds date,
  learned preferences, and budget id. Never move varying content into the static half, and
  keep `ToolRegistry` registration order stable — tools render first in the prompt, so
  reordering them invalidates everything. Check `cacheReadTokens` in `agent_runs.metadata`
  to confirm it still works.
- **Observation timestamps come from the transcript, not the model.** Each observer entry is
  prefixed `N. [YYYY-MM-DD HH:MM]`, rendered in `AGENT_TIMEZONE` from `AgentMessage.CreatedAt`
  (carried over from `ConversationMessage` on replay; zero means "this run", i.e. now). The LLM
  is told to copy them, and `repairObservationTimes` clamps anything unparseable or outside the
  observed window — instructing a model to report a fact it was never given produces a constant
  hallucination, which is what stored every observation at 14:00.
- **Context budget.** `messageTokens` (8k) is deliberately low to control cost.
  `enforceTokenBudget` makes it a real ceiling: it shrinks old tool-result bodies first,
  then drops whole tool-call groups oldest-first. Groups must stay intact for the same
  `tool_use`/`tool_result` pairing reason as above.

## Navigation

The top `Navbar` is the only mounted nav chrome (`Layout` does not render
`components/layout/Sidebar`). Keep it to the daily destinations — Home,
Transactions, Budget, Payees, Reports. Occasional or configuration-adjacent
screens live as **Settings sections** instead, addressed by
`/settings?section=<id>` and registered in `SECTIONS` in
`features/settings/components/Settings.tsx`: loans (`&account=<id>`),
recurring, activity, review, tags, ai. Their former top-level paths
(`/loans/:id`, `/activity`, `/recurring`, `/review`) redirect to the matching
section, so older links keep working.

## Code Conventions

### Go (Gin Framework)
- **Layering**: Handler (parse request, return JSON) → Service (business logic) → Repository (DB operations)
- **Imports**: stdlib → third-party (gin, uuid, pgx) → local packages, separated by blank lines
- **Error handling**: Return `gin.H{"error": err.Error()}` with appropriate HTTP status
- **Database transactions**: Use `*Tx` suffixed repository methods when atomicity is required
- **Naming**: PascalCase exported, camelCase private
- **Internal traffic**: normalize ingress metadata with shared middleware, trust only `VerifiedInternal` context for internal bypasses, and seed outbound contexts with `INTERNAL_AUTH_TOKEN`

### React Frontend
- **State**: Redux Toolkit with feature-based slices in `features/*/store/`
- **API**: `apiClient` singleton in `src/utils/api.ts` auto-injects `x-budget-id` header
- **Structure**: Feature folders (`features/transactions/`, `features/budget/`) with `components/`, `hooks/`, `store/`, `types/`
- **Hooks**: Use typed `useAppDispatch` and `useAppSelector` from `src/app/hooks.ts`
- **UI**: HeroUI components, Phosphor icons (`@phosphor-icons/react`), Tailwind CSS v4, Recharts for charts

### Angular Frontend
- **State**: NGXS actions/selectors in `store/dashboard/states/`
- **DI**: Constructor injection (not `inject()`)
- **Types**: Explicit interfaces in `src/app/models/`
- **Styling**: SCSS + Tailwind CSS

### TypeScript (Both Frontends)
- Strict mode enabled
- Explicit interfaces for all models

### Cross-Service Communication
- `go-gmail` → Temporal → `cipher` → `go-pennywise-api`: email ingestion runs through the `EmailToTransactionWorkflow`; classification is cipher's `PredictionActivity`
- Go services → Go services: shared HTTP transport injects canonical correlation/caller/origin headers plus `X-Internal-Token` from context
- `cipher` → frontends: agent stream deltas via Redis stream `pubsub`, rebroadcast by the API's websocket hub. Run failures publish an `error`-type chat stream event on the same path; `AgentChat.tsx` renders it as a red error bubble and stops the loader. The failed run's stored `error` field carries the real upstream message (cipher returns it in its 500 body; the shared HTTP transport appends non-2xx response bodies to its errors).
- Frontend → API: REST with budget ID in header interceptor

## Key Files

| Purpose | Path |
|---------|------|
| API routes | `backend/go-pennywise-api/cmd/api/main.go` |
| Email extraction (local LLM) | `backend/cipher/internal/client/ollama.go` (`ExtractEmailData`) |
| Transaction model (Go) | `backend/go-pennywise-api/internal/model/transaction.go` |
| Transaction model (TS) | `frontend/src/app/models/transaction.model.ts` |
| React API client | `react-frontend/src/utils/api.ts` |
| Reports API (`/api/reports/{spending,income-expense,networth}`) | `backend/shared/db/report.go`, `backend/go-pennywise-api/internal/service/report.go` |
| Reports UI (donut/bar/net-worth charts) | `react-frontend/src/features/reports/` |
| Tag manager UI (Settings → Tags) | `react-frontend/src/features/settings/components/TagSettings.tsx` |
| Geocoding (Nominatim reverse) | `backend/go-pennywise-api/internal/service/geocode.go` |
| Receipt storage + API | `backend/go-pennywise-api/internal/storage/local.go`, `internal/service/document.go`, `backend/shared/db/transactionDocument.go` |
| Scan → single PDF (image merge) | `backend/go-pennywise-api/internal/service/documentScan.go` |
| Expo push client / device tokens | `backend/go-pennywise-api/internal/client/expopush.go`, `backend/shared/db/devicePushToken.go` |
| Location/receipts UI (React) | `react-frontend/src/features/transactions/components/TransactionDetailPanel/TransactionLocationSection.tsx`, `TransactionDocumentsSection.tsx` |
| Location snap task (Android) | `android-frontend/src/features/notifications/locationSnapTask.ts` |
| Headless API client (Android background tasks) | `android-frontend/src/utils/headlessApi.ts` |
| Android home-screen widgets | `android-frontend/src/features/widgets/` |
| Settings shell + section registry (`/settings?section=`) | `react-frontend/src/features/settings/components/Settings.tsx` |
| Loans UI (Settings → Loans, `?account=<id>`) | `react-frontend/src/features/loans/` |
| Pipeline activity UI (Settings → Activity) | `react-frontend/src/features/pipeline/` |
| Recurring transactions (`/api/recurring-transactions`) | `backend/shared/db/recurringTransaction.go`, `backend/go-pennywise-api/internal/service/recurringTransaction.go` |
| Recurring transactions UI (Settings → Recurring) | `react-frontend/src/features/recurring/` |
| Prediction review (`/api/predictions/review`) | `backend/shared/db/predictionReview.go`, `backend/go-pennywise-api/internal/service/predictionReview.go` |
| Prediction review UI (Settings → Prediction Review) | `react-frontend/src/features/predictionReview/` |
| React Redux store | `react-frontend/src/app/store.ts` |
| Pipeline run tracking (workflow side) | `backend/workflows/internal/workflow/pipelineStatus.go` |
| Pipeline status activity | `backend/go-pennywise-api/internal/temporal/activities/reportPipelineStatus.go` |
| Pipeline runs repo / API (`/api/pipeline/runs`) | `backend/shared/db/pipelineRun.go`, `backend/go-pennywise-api/internal/service/pipeline.go` |
| Pipeline UI (Activity page, `/activity`) | `react-frontend/src/features/pipeline/` |
| Chat agent loop | `backend/cipher/agent/runtime/agent.go` |
| Agent tools | `backend/cipher/agent/tools/` (`getSpendingSummary`, `getTopTransactions`, `getBudgetInfo`, `getSchema`, `executeSQL`, `getToday`, `updateMemory`) |
| Agent system prompt | `backend/cipher/agent/context/prompts.go` (`SystemPromptStatic` / `SystemPromptDynamic`) |
| Agent context budget | `backend/cipher/agent/memory/memory.go` (`PrepareContext`, `enforceTokenBudget`) |
| Docker Compose | `docker-compose.yml` |
| CI/CD | `.github/workflows/workflow.yml` |

## Maintenance

After completing any new feature, bug fix, or task, update this CLAUDE.md file if the change affects architecture, conventions, key files, build commands, or data flow.

## Environment

- **Database**: PostgreSQL via pgx (`internal/db/db.go`). Migrations via goose (`make migrate-up` in `backend/go-pennywise-api`).
  Migration versions must be unique: goose **panics** when two files share one, which blocks every pending
  migration, not just the clashing pair. Two branches each adding "the next number" merge cleanly in git and
  then fail at runtime as a missing column, so renumber to the end of the sequence when rebasing onto a branch
  that added migrations. `db/migrations/migrations_test.go` guards this.
  `00001` builds its tables with `CREATE TABLE IF NOT EXISTS`, so on a database that predates it the
  column constraints it declares were never applied — `transactions.status` was nullable there, and rows
  imported without one held NULL. Migration `00022` backfills those to `MANUAL` and enforces the
  constraint; reads still go through `scannedStatus` (`shared/db/transaction.go`), which treats a NULL
  status as `MANUAL`, since a non-pointer scan destination turns one legacy row into a failed lookup.
- **Auth**: Google OAuth (auth-code flow) + JWT. `POST /api/auth/google` issues 15-min access / 30-day refresh tokens; `AuthMiddleware` accepts Bearer header, `access_token` cookie, or `X-API-Key`. Budget ownership enforced by `BudgetIdMiddleware` (`budgets.user_id` must match the authenticated `auth_users` row).
- **Demo mode**: `DEMO_MODE=true` (API) enables `POST /api/auth/demo` — logs into a persistent seeded demo user (`demo@pennywise.local`, budget + categories + accounts + ~4 months of transactions + payee rules + cipher predictions across all sources, seeded idempotently in a single DB transaction on first login by `internal/service/demo.go`/`demo_seed.go`). Frontend shows a "Try Demo" login button when `VITE_DEMO_MODE=true`; for the demo user (`selectIsDemoUser` in `features/auth/store/authSlice.ts`) AI config editing and budget creation are disabled. No Google account needed.
- **API port**: `PORT` env var (default 5151).
- **Deployment**: Docker Compose on self-hosted Unraid, deployed via GitHub Actions CI. Secondary: Railway.app for Go API.
- **Env files**: `backend/go-gmail/.env`, `backend/go-pennywise-api/.env`, `backend/cipher/.env`
- **Agent env** (cipher): `AGENT_PROVIDER`, `AGENT_TITLE_MODEL` (`provider/model`; unset uses the
  default provider's default model), `AGENT_MAX_TURNS`, `AGENT_MAX_TOOL_CALLS`, `AGENT_TIMEZONE`
  (IANA zone for `get_today`; containers default to UTC), `AGENT_DB_URL` (least-privilege
  read-only connection for the agent's SQL tools — see migration `00015`, which also documents
  the roles an administrator must create, since the migrating role usually cannot `CREATE ROLE`)
- **Internal service auth**: Go services now expect a shared `INTERNAL_AUTH_TOKEN` for verified service-to-service HTTP calls
