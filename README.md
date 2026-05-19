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

**Prerequisites:** Go, Node.js + npm, PostgreSQL, Ollama, Docker (optional)

```bash
# Go API
cd backend/go-pennywise-api && go run ./cmd/api

# Gmail watcher
cd backend/go-gmail && go run .

# Cipher
cd backend/cipher && go run ./cmd/api

# React frontend
cd react-frontend && npm run dev

# Full stack (Docker)
docker-compose up --build
```

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
| Go Gmail | `8080` |
| Cipher | `5160` |
| React dev | `5173` |
| Angular dev | `5000` |

## Docs

- [AGENTS.md](AGENTS.md) — full service map, auth flow, caveats
- [docs/cipher.md](docs/cipher.md) — classification pipeline architecture
- [docs/observability.md](docs/observability.md) — OTel, logging, metrics
- [docs/transport-architecture.md](docs/transport-architecture.md) — inter-service transport layer
