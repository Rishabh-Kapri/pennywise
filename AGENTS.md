# Agent Guidelines for Pennywise

## Overview

Pennywise is a personal finance/budgeting monorepo. The repo currently contains production paths, active migration work, and a few experimental services.

### Current service map

| Service | Path | Status | Responsibility |
|---------|------|--------|----------------|
| Go API | `backend/go-pennywise-api` | Active | Core REST API (Gin + PostgreSQL), auth, budgets, transactions, tags, loan metadata, websocket fanout |
| Gmail watcher | `backend/go-gmail` | Active | Gmail Pub/Sub ingestion, email parsing, MLP prediction calls, transaction/prediction creation |
| Python MLP | `backend/python-mlp` | Active | `/predict` inference and retraining/augmentation endpoints |
| Cipher | `backend/cipher` | Active but partial integration | Prediction orchestrator (Ollama + pgvector + MLP/LLM fallback), corrections, embedding backfill |
| Shared Go module | `backend/shared` | Active | Common logging, context propagation, transport abstraction, DB base repository |
| Temporal workflows | `backend/workflows` | Experimental | Workflow definitions and worker scaffolding |
| React frontend | `react-frontend` | Active development | Main web app (React 19 + Redux Toolkit + Vite) |
| Angular frontend | `frontend` | Legacy/maintenance | Older app (Angular 17 + NGXS + Firestore remnants) |
| File parser | `backend/file-parser` | Experimental | Clojure service scaffold for bulk upload flows |

`backend/setu` currently exists as a placeholder module.

## High-level architecture

### Current primary flow

```
Gmail Push (Pub/Sub)
        |
        v
    go-gmail
        |
        | parse email + call /predict (python-mlp)
        v
 go-pennywise-api (PostgreSQL)
        ^
        |
 React frontend / Angular frontend
```

### Additional AI flow (newer path)

```
cipher
  |- generates embeddings (Ollama/OpenAI-compatible model)
  |- checks pgvector similarity in transaction_embeddings
  |- falls back to MLP/LLM
  |- publishes agent stream deltas to Redis stream `pubsub`
  |- handles correction upserts and backfill tooling

Redis stream `pubsub`
        |
        v
go-pennywise-api websocket listener
        |
        v
budget-scoped browser websocket clients
```

## Key data and auth flow

1. **React auth**: Google credential -> `POST /api/auth/google` -> access + refresh tokens.
2. **Internal service auth**: service-to-service HTTP calls propagate `X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, `X-Budget-ID`, and `X-Internal-Token`; shared middleware verifies internal requests and sets `VerifiedInternal` in context.
3. **API auth middleware**: accepts `Authorization: Bearer ...` or `X-API-Key` for user traffic, and trusts only shared `VerifiedInternal` context for internal bypass.
4. **Budget scoping**: budget-scoped routes require `X-Budget-ID`; middleware verifies ownership for user traffic and trusts only verified internal requests for service traffic.
5. **Gmail ingestion**: Pub/Sub event -> parser -> 3-step MLP prediction (account/payee/category) -> create transaction + prediction via API.
6. **Prediction corrections**: transaction updates on MLP-sourced records update `predictions.has_user_corrected` fields in API service logic.
7. **Agent streaming**: Cipher writes `eventName`, `budgetId`, and `data` fields to Redis stream `pubsub`; Go API reads new stream entries and broadcasts them to websocket clients scoped to the same budget.

## Build, test, lint

| Component | Build/Run | Test | Lint/Format |
|-----------|-----------|------|-------------|
| Go API | `cd backend/go-pennywise-api && go build ./cmd/api` | `cd backend/go-pennywise-api && go test ./...` | `cd backend/go-pennywise-api && go fmt ./... && go vet ./...` |
| Go Gmail | `cd backend/go-gmail && go build .` | `cd backend/go-gmail && go test ./...` | `cd backend/go-gmail && go fmt ./... && go vet ./...` |
| Cipher | `cd backend/cipher && go build ./cmd/api` | `cd backend/cipher && go test ./...` | `cd backend/cipher && go fmt ./... && go vet ./...` |
| Shared | - | `cd backend/shared && go test ./...` | `cd backend/shared && go fmt ./... && go vet ./...` |
| Workflows | `cd backend/workflows && go build ./cmd/worker` | `cd backend/workflows && go test ./...` | `cd backend/workflows && go fmt ./... && go vet ./...` |
| Python MLP | `cd backend/python-mlp && python mlp_predict_server.py` | manual/API-level validation | - |
| React frontend | `cd react-frontend && npm run dev` / `npm run build` | no dedicated test suite currently | `cd react-frontend && npm run lint` |
| Angular frontend | `cd frontend && npm start` / `npm run build` | `cd frontend && npm test` | TypeScript strict mode |
| File parser | `cd backend/file-parser && clojure -M:run-m` | `cd backend/file-parser && clojure -T:build test` | - |

Each Go module has a local `Makefile` with common aliases such as `make run`, `make build`, `make test`, `make fmt`, `make vet`, `make check`, `make tidy`, and `make vendor` where applicable. The Go API Makefile also exposes migration aliases such as `make migrate-up`, `make migrate-one`, `make migrate-down`, and `make migrate-status`.

### Database migrations (Go API)

- `cd backend/go-pennywise-api && go run ./cmd/migrations -dir . up`
- `cd backend/go-pennywise-api && go run ./cmd/migrations -dir . status`
- `baseline` command exists for marking initial seed migrations as applied.

### Backfill command (Cipher)

- `cd backend/cipher && go run ./cmd/backfill -data <path-to-json>`
- Without `-data`, it fetches `/api/predictions` from `PENNYWISE_API` using `BUDGET_ID`.

## Runtime ports and local dev

- Go API: `5151`
- Go Gmail: `5170`
- Cipher: `5160` (default)
- Angular dev server: `5000`
- React dev server: Vite default (`5173`)

`docker-compose.yml` currently defines the backend/dev dependency stack: PostgreSQL with pgvector, Redis, Temporal + UI, `go-pennywise-api`, `go-gmail`, `cipher`, and `workflows`. Frontend, Android, Python MLP, file-parser, and Ollama are intentionally excluded from that compose stack. Cipher expects `OLLAMA_URL` to point at a reachable external Ollama endpoint.

## Code patterns and conventions

### Go API (`backend/go-pennywise-api`)

- Layering is `handler -> service -> repository`.
- Services read tenant context from `context.Context` via shared utils (`MustUserID`, `MustBudgetID`).
- Route protection is middleware-first:
  - auth only for global user resources (`/api/budgets`, `/api/keys`)
  - auth + budget middleware for budget-scoped resources (`accounts`, `transactions`, `categories`, `payees`, `tags`, `loan-metadata`, etc.)
- Transaction service contains side effects in one place (carryovers, transfers, prediction correction sync).
- Websocket fanout uses `internal/websocket.ConnectionHub`; Redis stream events from Cipher are consumed by `RedisStreamListener` and rebroadcast through the same budget-scoped hub.
- Agent run routes (`POST /api/agent/runs`, `GET /api/agent/runs/:id`, `POST /api/agent/runs/:id/cancel`) are budget-scoped API routes. The Go API owns conversation/run/message persistence and dispatches execution to Cipher through the shared transport client.
- Agent persistence uses `conversations` for long-lived chat threads, `agent_runs` for per-prompt/per-agent executions, and `conversation_messages` for ordered transcript rows. Metadata is stored at all three levels as JSONB.
- Errors increasingly use typed shared error wrappers from `backend/shared/errors`.

### Shared module (`backend/shared`)

- `transport.Client` + `httpclient` implement a protocol-agnostic client/engine split.
- Context headers are propagated centrally via `utils.GetHeaders`, including canonical caller/origin/correlation headers and `X-Internal-Token` when a service context is seeded with `INTERNAL_AUTH_TOKEN`.
- `middleware.RequestMetadata` normalizes ingress metadata, `middleware.InternalRequestAuth` verifies internal traffic, and `middleware.BudgetIdMiddleware` now keys off shared verified-internal context instead of raw headers.
- `shared/temporal.RequestMetadataPropagator` bridges `correlation_id` and `origin_service` across Temporal workflow/activity boundaries.

### Go Gmail (`backend/go-gmail`)

- Main operational path uses Pub/Sub + `runner.ProcessGmailHistoryId`.
- Email parsing is regex-based in `pkg/parser/email.go`; extraction order matters (type before amount sign).
- Predictions are currently 3 sequential calls to Python MLP (`account -> payee -> category`) with confidence gating.

### React frontend (`react-frontend`)

- Feature-first Redux slices in `src/features/*/store/`.
- `apiClient` auto-adds:
  - `Authorization` bearer token (except auth endpoints)
  - `x-budget-id` from selected budget (except budget endpoints)
- Automatic refresh flow retries 401s once using `POST /auth/refresh`.
- Route protection is handled by `features/auth/components/ProtectedRoute.tsx`.

### Angular frontend (`frontend`)

- NGXS-based state remains in `src/app/store/dashboard/states/`.
- `HeadersInterceptor` still injects `X-Budget-ID`.
- Firestore-backed services remain present for legacy paths.

### Python MLP (`backend/python-mlp`)

- Main endpoints:
  - `POST /predict`
  - `POST /retrain`, `GET /retrain/:id`
  - `POST /fetch`, `POST /augment`, `POST /rollback`
  - `GET /backups`, `GET /health`
- Models are loaded from `.parms` files, with Docker entrypoint seeding `/data` on first run.

## Key files reference

| Purpose | Path |
|---------|------|
| API routes and middleware wiring | `backend/go-pennywise-api/cmd/api/main.go` |
| Auth middleware | `backend/go-pennywise-api/internal/middleware/auth.go` |
| Budget ownership middleware | `backend/go-pennywise-api/internal/middleware/budget.go` |
| Transaction side effects/corrections | `backend/go-pennywise-api/internal/service/transaction.go` |
| Agent run proxy handler/service | `backend/go-pennywise-api/internal/handler/agent.go`, `backend/go-pennywise-api/internal/service/agent.go` |
| Redis-to-websocket stream listener | `backend/go-pennywise-api/internal/websocket/redis_stream_listener.go` |
| Gmail pipeline orchestrator | `backend/go-gmail/pkg/runner/runner.go` |
| Gmail email parser | `backend/go-gmail/pkg/parser/email.go` |
| Python MLP server endpoints | `backend/python-mlp/mlp_predict_server.py` |
| Cipher prediction orchestration | `backend/cipher/internal/service/prediction.go` |
| Cipher agent streaming publisher | `backend/cipher/agent/runtime/agent.go` |
| Shared transport abstraction | `backend/shared/transport/client.go` |
| Shared HTTP transport implementation | `backend/shared/httpclient/transport.go` |
- Shared internal request verifier | `backend/shared/middleware/internalRequestAuth.go` |
- Shared Temporal propagator | `backend/shared/temporal/propagator.go` |
| React app routes | `react-frontend/src/app/App.tsx` |
| React API client | `react-frontend/src/utils/api.ts` |
| React store | `react-frontend/src/app/store.ts` |
| Angular store config | `frontend/src/app/store/store.config.ts` |
| CI workflow | `.github/workflows/workflow.yml` |
| Local stack compose | `docker-compose.yml` |

## Environment and deployment notes

### Common env files

- `backend/go-pennywise-api/.env`: `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `DOMAIN`, `INTERNAL_AUTH_TOKEN`, optional `REDIS_URL`
- `backend/go-gmail/.env`: Gmail/PubSub credentials + `MLP_API`, `PENNYWISE_API`, Temporal host/port, `INTERNAL_AUTH_TOKEN`
- `backend/cipher/.env`: `DATABASE_URL`, `OLLAMA_URL`, `MLP_API`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `ANTHROPIC_API_KEY`, `AGENT_PROVIDER`, `PORT`, `INTERNAL_AUTH_TOKEN`, optional `REDIS_URL`
- `react-frontend/.env*`: `VITE_API_URL`, `VITE_GOOGLE_CLIENT_ID`

`python-mlp` primarily uses runtime env vars (`PORT`, optional `VOLUME_DIR`) and data/model files.

### CI/deploy

- GitHub Actions workflow (`master` push) copies env files on self-hosted runner, then runs Docker Compose.
- Current changed-service mapping in CI mainly targets: `go-gmail`, `python-mlp`, `go-pennywise-api`, and Angular frontend.

### Git hooks

- Root README expects: `git config core.hooksPath .githooks`
- Pre-commit hook re-runs `go mod vendor` for Go API when `backend/go-pennywise-api/go.mod|go.sum` or `backend/shared/` changes.

## Current caveats (important for agents)

- `go-gmail/pkg/pennywise-api/service.go` currently hardcodes `X-Budget-ID`.
- `cipher/internal/service/prediction.go` currently hardcodes a budget ID in `Predict` and does not yet use request budget header.
- Temporal integration is partial:
  - `go-gmail/main.go` registers workflow/activity but does not start worker run loop.
  - `backend/workflows/cmd/worker/main.go` references `HelloWorldWorkflow` which is not present.
- API embedding service (`internal/service/embedding.go`) is mostly stubbed.
- React frontend calls `POST /auth/logout`, but logout endpoint is currently commented out in API routes.
- `file-parser` and `setu` are not production-ready.

## Testing snapshot

- Go API tests are focused in service/repository/handler packages (notably transaction and loan metadata flows).
- Go Gmail has parser and API client tests.
- Shared module has text-cleaning utility tests.
- React frontend currently has no committed automated test suite.
- Angular frontend still has Karma/Jasmine tests.

## Maintenance

When architecture, routes, service responsibilities, or build/test commands change, update this file in the same PR.
