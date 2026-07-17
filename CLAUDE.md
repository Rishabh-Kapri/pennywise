# CLAUDE.md

## Project Overview

Pennywise is a personal finance/budgeting app with AI-powered transaction classification from email parsing. Monorepo services (each directory has its own README.md):

- **go-pennywise-api** (`backend/go-pennywise-api`): Core REST API (Gin, PostgreSQL/pgx). Handler → service → repository layers (repos in `backend/shared/db`).
- **cipher** (`backend/cipher`): Classification pipeline (Ollama extraction → payee rules → pgvector → LLM fallback) + budget agent runtime.
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

## Key Data Flow

1. Gmail push → `go-gmail` starts Temporal `EmailToTransactionWorkflow` with a deterministic ID (`email-to-txn-<email>-<historyId>`, duplicate pushes dedupe at Temporal) and its `FetchEmailData` activity returns raw email bodies → cipher processes **one email per activity** (`ParseEmail` extracts via local LLM, `PredictEmail` classifies; Ollama — the old regex parser in `pkg/parser/email.go` is deprecated). Non-transaction emails / unknown accounts are recorded skips; infrastructure failures retry per email, and only still-failed emails park on the retry signals (`retry-email-parse`/`retry-predict`, cipher `POST /api/workflows/:workflowId/retry-*`). Each round's successes are committed immediately via `go-pennywise-api`'s `CreateTransactionAndCipherPrediction`, whose insert is deduped on `(budget_id, dedupe_hash)` — retries never duplicate transactions. Cipher activities heartbeat before every LLM step (`internal/progress`); each LLM call is bounded by `CIPHER_LLM_CALL_TIMEOUT` (default 3m). Legacy batch activities (`ParseEmailData`/`Predict`) remain registered for pre-rollout workflows (`workflow.GetVersion` gate "per-email-pipeline").
2. All API calls require `X-Budget-ID` header — extracted via `utils.GetBudgetId(c)` in handlers
3. Internal service calls use shared request metadata headers (`X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, `X-Internal-Token`) and are trusted only after shared internal-request verification marks context as verified.
4. Temporal workflow/activity hops propagate `correlation_id` and `origin_service` through `backend/shared/temporal/propagator.go`; each activity restamps its local service name before downstream HTTP calls.
5. ML prediction corrections tracked in `internal/service/transaction.go` (`UserCorrectedPayee`, `UserCorrectedCategory`, etc.)
6. Transaction geolocation: transactions carry nullable `location_lat/lng/name/source` (source `auto`|`manual`). `PATCH /api/transactions/:id/location` sets/clears just the location (null lat+lng clears); missing place names are reverse-geocoded server-side via Nominatim (`internal/service/geocode.go`, best-effort, 1 req/s + in-process cache, `NOMINATIM_URL` env). The pipeline's `CreateTransactionAndCipherPrediction` sends a best-effort Expo push (`internal/client/expopush.go`, tokens in `device_push_tokens` registered via `POST /api/devices/push-token`) for newly created rows only; the Android app's background notification task (`android-frontend/src/features/notifications/locationSnapTask.ts`) then attaches the phone's last-known location with source `auto`; when the fix is stale (>15 min) or permission is missing it instead queues a "tag location?" prompt (AsyncStorage, 1h TTL) that fires on notification tap / app foreground and saves a fresh fix as source `manual`. Production Android push needs FCM credentials configured via EAS.
7. Receipts/documents: `transaction_documents` rows + files on local disk under `UPLOADS_DIR` (compose volume `uploads_data`, path validated against traversal). `POST/GET /api/transactions/:id/documents` (multipart field `file`, 10MB cap, content-sniffed mime whitelist: jpeg/png/webp/heic/pdf), `GET /api/documents/:docId/content` (authenticated streaming; frontends fetch with auth headers → blob/object URL since `<img src>` can't carry Authorization), `DELETE /api/documents/:docId` (soft-delete + file removal).
8. Pipeline observability: the email workflows report progress via `StartPipelineRun`/`ReportPipelineStatus` activities (hosted by the go-pennywise-api worker) into `pipeline_runs` + `pipeline_run_events`; the per-email loop emits one timeline event per email (parse/predict succeeded/skipped/failed, keyed by `messageId`) and each run change is broadcast budget-wide as the `pennywise::pipeline::update` websocket event (`sharedModel.EventPipelineUpdate`). The React `/activity` page lists runs, shows per-email extraction/prediction detail, and can retry parked workflows through `POST /api/pipeline/runs/:id/retry` (signals Temporal directly, choosing `retry-email-parse` vs `retry-predict` from the run's `current_step`). Reporting is best-effort — status write failures never fail the pipeline — and is gated by `workflow.GetVersion` marker "pipeline-observability" so pre-rollout in-flight workflows replay unchanged.

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
| Expo push client / device tokens | `backend/go-pennywise-api/internal/client/expopush.go`, `backend/shared/db/devicePushToken.go` |
| Location/receipts UI (React) | `react-frontend/src/features/transactions/components/TransactionDetailPanel/TransactionLocationSection.tsx`, `TransactionDocumentsSection.tsx` |
| Location snap task (Android) | `android-frontend/src/features/notifications/locationSnapTask.ts` |
| React Redux store | `react-frontend/src/app/store.ts` |
| Pipeline run tracking (workflow side) | `backend/workflows/internal/workflow/pipelineStatus.go` |
| Pipeline status activity | `backend/go-pennywise-api/internal/temporal/activities/reportPipelineStatus.go` |
| Pipeline runs repo / API (`/api/pipeline/runs`) | `backend/shared/db/pipelineRun.go`, `backend/go-pennywise-api/internal/service/pipeline.go` |
| Pipeline UI (Activity page, `/activity`) | `react-frontend/src/features/pipeline/` |
| Docker Compose | `docker-compose.yml` |
| CI/CD | `.github/workflows/workflow.yml` |

## Maintenance

After completing any new feature, bug fix, or task, update this CLAUDE.md file if the change affects architecture, conventions, key files, build commands, or data flow.

## Environment

- **Database**: PostgreSQL via pgx (`internal/db/db.go`). Migrations via goose (`make migrate-up` in `backend/go-pennywise-api`).
- **Auth**: Google OAuth (auth-code flow) + JWT. `POST /api/auth/google` issues 15-min access / 30-day refresh tokens; `AuthMiddleware` accepts Bearer header, `access_token` cookie, or `X-API-Key`. Budget ownership enforced by `BudgetIdMiddleware` (`budgets.user_id` must match the authenticated `auth_users` row).
- **Demo mode**: `DEMO_MODE=true` (API) enables `POST /api/auth/demo` — logs into a persistent seeded demo user (`demo@pennywise.local`, budget + categories + accounts + ~4 months of transactions + payee rules + cipher predictions across all sources, seeded idempotently in a single DB transaction on first login by `internal/service/demo.go`/`demo_seed.go`). Frontend shows a "Try Demo" login button when `VITE_DEMO_MODE=true`; for the demo user (`selectIsDemoUser` in `features/auth/store/authSlice.ts`) AI config editing and budget creation are disabled. No Google account needed.
- **API port**: `PORT` env var (default 5151).
- **Deployment**: Docker Compose on self-hosted Unraid, deployed via GitHub Actions CI. Secondary: Railway.app for Go API.
- **Env files**: `backend/go-gmail/.env`, `backend/go-pennywise-api/.env`, `backend/cipher/.env`
- **Internal service auth**: Go services now expect a shared `INTERNAL_AUTH_TOKEN` for verified service-to-service HTTP calls
