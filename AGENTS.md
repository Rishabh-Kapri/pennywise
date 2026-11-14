# Pennywise

Personal budgeting app (YNAB-style) with AI transaction classification from bank emails.
**Before working in a directory, read its `AGENTS.md`.**

| Path | What | Guide |
|---|---|---|
| `backend/go-pennywise-api` | Core REST API (Gin, pgx) | [AGENTS.md](backend/go-pennywise-api/AGENTS.md) |
| `backend/cipher` | Email classification pipeline + chat agent | [AGENTS.md](backend/cipher/AGENTS.md) |
| `backend/go-gmail` | Gmail Pub/Sub watcher → Temporal. Replaced by `ingest`; CI no longer builds or deploys it | [AGENTS.md](backend/go-gmail/AGENTS.md) |
| `backend/ingest` | Message ingestion (email, SMS, API) → job queue. Skeleton | [AGENTS.md](backend/ingest/AGENTS.md) |
| `backend/workflows` | Temporal worker + workflows. Replaced by the job queue; CI no longer builds or deploys it | [AGENTS.md](backend/workflows/AGENTS.md) |
| `backend/shared` | Shared Go module (repos, models, middleware, transport) | [AGENTS.md](backend/shared/AGENTS.md) |
| `react-frontend` | Main web app (React 19, Redux Toolkit) | [AGENTS.md](react-frontend/AGENTS.md) |
| `android-frontend` | Expo React Native app | [AGENTS.md](android-frontend/AGENTS.md) |
| `frontend` | Angular 17 app, legacy and maintenance only | [AGENTS.md](frontend/AGENTS.md) |

`backend/file-parser` (experimental) is not run anywhere. Don't extend it. `docs/` holds architecture notes.

## Main flow

Gmail push → `go-gmail` → Temporal `EmailToTransactionWorkflow` → cipher `ParseEmail`/`PredictEmail` (one email per activity) → go-pennywise-api creates the transaction (deduped on `(budget_id, dedupe_hash)`) → websocket → clients.

## Cross-cutting rules

- **Changed `backend/shared`? Re-vendor every Go service.** Docker and CI build with `-mod=vendor`. See [backend/shared/AGENTS.md](backend/shared/AGENTS.md).
- **Migration numbers must be sequential and unique.** goose panics on a duplicate. Renumber yours to the end after rebasing (`backend/shared/migrations/sql`, guide in its `README.md`). A change to an existing table is a new file with an `ALTER`; never edit an applied one.
- Every API call is budget-scoped by the `X-Budget-ID` header. Service-to-service calls send the caller's own `INTERNAL_AUTH_TOKEN`; receivers accept the tokens listed in `SERVICE_TOKENS`.
- Keep `workflow.GetVersion` gates intact. In-flight workflows replay against them.

## Run

```bash
docker-compose up --build   # postgres, redis, temporal, api, cipher, gmail-watch, workflows
```

Deploy: CI (`.github/workflows/workflow.yml`) ships the backend to Railway on push to `dev`. Self-hosted via Docker Compose.

**Agents: clean up what you start.** Before you report back, stop every dev server, `go run`, watcher and container you started to test something, and remove the containers. Don't leave them running for the user to look at; they start their own. Never stop a process or container you didn't start: other sessions and the user's own servers run alongside yours.

## Maintenance

When a change makes an `AGENTS.md` wrong, fix it in the same change. Keep these files lean: rules and gotchas only, no feature narratives.
