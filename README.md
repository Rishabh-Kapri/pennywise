# Pennywise

Personal finance/budgeting app with ML-powered transaction classification from email parsing.

## Setup

```bash
git config core.hooksPath .githooks
```

Runs `go mod vendor` automatically when `go.mod` or `shared/` changes before a commit.

## Services

| Service | Path | Status |
|---------|------|--------|
| Go API | `backend/go-pennywise-api` | Active |
| Gmail watcher | `backend/go-gmail` | Active |
| Cipher | `backend/cipher` | Active |
| Shared Go module | `backend/shared` | Active |
| Temporal workflows | `backend/workflows` | Experimental |
| React frontend | `react-frontend` | Active development |
| Angular frontend | `frontend` | Legacy/maintenance |
| Python MLP | `backend/python-mlp` | Deprecated |
| File parser | `backend/file-parser` | Experimental |

For a full architecture map, service responsibilities, and caveats see [AGENTS.md](AGENTS.md).

## High-Level Flow

1. Gmail push event → `go-gmail` starts a Temporal `EmailToTransactionWorkflow`.
2. Workflow invokes `cipher`'s `PredictionActivity` (Ollama extraction → payee rules → pgvector → LLM fallback).
3. Transaction is created via `go-pennywise-api`.
4. React frontend consumes the API.

See [docs/cipher.md](docs/cipher.md) for the full classification pipeline.

## Quick Start

**Prerequisites:** Docker and Docker Compose for the full backend stack.

```bash
docker compose up --build
```

The compose stack starts the backend services and their local dependencies:
PostgreSQL with pgvector, Redis, Temporal + Temporal UI, Go API, Gmail watcher,
Cipher, and workflows worker. Frontend, Android, file-parser, and the legacy
Python MLP service are intentionally not part of this stack.

Copy `.env.example` to `.env` only when you need to override defaults or add
real Google/OAuth/LLM credentials.

Cipher still needs Ollama for local extraction/embedding. Run Ollama on the
host (`ollama serve`) and keep `OLLAMA_URL=http://host.docker.internal:11434`,
or point `OLLAMA_URL` at another reachable Ollama endpoint.

## Common Commands

| Component | Build | Test |
|-----------|-------|------|
| Go API | `go build ./cmd/api` | `go test ./...` |
| Go Gmail | `go build .` | `go test ./...` |
| Cipher | `go build ./cmd/api` | `go test ./...` |
| Shared | — | `go test ./...` |
| React frontend | `npm run build` | `npm run lint` |
| Angular frontend | `npm run build` | `npm test` |

All Go commands run from the respective `backend/<service>` directory.

## Database Migrations

```bash
cd backend/go-pennywise-api
go run ./cmd/migrations -dir . up
go run ./cmd/migrations -dir . status
```

## Ports

| Service | Port |
|---------|------|
| Go API | `5151` |
| Go Gmail | `5170` |
| Cipher | `5160` |
| Temporal UI | `8233` |
| React dev | `5173` |
| Angular dev | `5000` |

## Docs

- [AGENTS.md](AGENTS.md) — full service map, auth flow, caveats
- [docs/cipher.md](docs/cipher.md) — classification pipeline architecture
- [docs/observability.md](docs/observability.md) — OTel, logging, metrics
- [docs/transport-architecture.md](docs/transport-architecture.md) — inter-service transport layer
