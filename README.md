# Pennywise

Personal finance/budgeting app with ML-powered transaction classification from email parsing.

## Setup

After cloning, enable git hooks:

```bash
git config core.hooksPath .githooks
```

This runs `go mod vendor` automatically when `go.mod` or `shared/` changes before a commit.

## What This Repo Contains

Pennywise is a personal finance/budgeting monorepo with multiple services.

| Service | Path | Status | Responsibility |
|---------|------|--------|----------------|
| Go API | `backend/go-pennywise-api` | Active | Core REST API (Gin + PostgreSQL), auth, budgets, transactions, tags, loan metadata |
| Gmail watcher | `backend/go-gmail` | Active | Gmail Pub/Sub ingestion, email parsing, MLP prediction calls, transaction/prediction creation |
| Python MLP | `backend/python-mlp` | Active | `/predict` inference and retraining/augmentation endpoints |
| Cipher | `backend/cipher` | Active (partial integration) | Prediction orchestrator (Ollama + pgvector + MLP/LLM fallback) |
| Shared Go module | `backend/shared` | Active | Shared logging, context propagation, transport abstraction, DB base repository |
| Temporal workflows | `backend/workflows` | Experimental | Workflow definitions and worker scaffolding |
| React frontend | `react-frontend` | Active development | Main web app (React + Redux Toolkit + Vite) |
| Angular frontend | `frontend` | Legacy/maintenance | Older app (Angular + NGXS + Firestore remnants) |
| File parser | `backend/file-parser` | Experimental | Clojure scaffold for bulk upload flows |

`backend/setu` currently exists as a placeholder module.

## High-Level Flow

Primary path (today):

1. Gmail push event arrives.
2. `go-gmail` fetches and parses new transaction emails.
3. `go-gmail` calls `python-mlp` for predictions.
4. `go-gmail` creates transactions/predictions via `go-pennywise-api`.
5. React/Angular frontends consume the API.

Additional AI path:

- `cipher` can generate embeddings, perform pgvector similarity lookup, and fall back to MLP/LLM.

## Quick Start

### 1) Prerequisites

- Go (for Go services)
- Python 3.x (for `backend/python-mlp`)
- Node.js + npm (for both frontends)
- PostgreSQL
- Optional: Docker + Docker Compose
- Optional: Ollama/OpenAI-compatible endpoint (for Cipher)

### 2) Run services individually

Go API:

```bash
cd backend/go-pennywise-api && go build ./cmd/api
cd backend/go-pennywise-api && go run ./cmd/api
```

Gmail watcher:

```bash
cd backend/go-gmail && go build .
cd backend/go-gmail && go run .
```

Python MLP:

```bash
cd backend/python-mlp && python mlp_predict_server.py
```

Cipher:

```bash
cd backend/cipher && go build ./cmd/api
cd backend/cipher && go run ./cmd/api
```

React frontend:

```bash
cd react-frontend && npm run dev
```

Angular frontend (legacy):

```bash
cd frontend && npm start
```

### 3) Run local stack with Docker

```bash
docker-compose up --build
```

Current compose file includes: `go-gmail`, `python-mlp`, `go-pennywise-api`, and Angular frontend.

## Common Commands

| Component | Build/Run | Test | Lint/Format |
|-----------|-----------|------|-------------|
| Go API | `cd backend/go-pennywise-api && go build ./cmd/api` | `cd backend/go-pennywise-api && go test ./...` | `cd backend/go-pennywise-api && go fmt ./... && go vet ./...` |
| Go Gmail | `cd backend/go-gmail && go build .` | `cd backend/go-gmail && go test ./...` | `cd backend/go-gmail && go fmt ./... && go vet ./...` |
| Cipher | `cd backend/cipher && go build ./cmd/api` | `cd backend/cipher && go test ./...` | `cd backend/cipher && go fmt ./... && go vet ./...` |
| Shared | - | `cd backend/shared && go test ./...` | `cd backend/shared && go fmt ./... && go vet ./...` |
| Workflows | `cd backend/workflows && go build ./cmd/worker` | `cd backend/workflows && go test ./...` | `cd backend/workflows && go fmt ./... && go vet ./...` |
| Python MLP | `cd backend/python-mlp && python mlp_predict_server.py` | Manual/API-level validation | - |
| React frontend | `cd react-frontend && npm run dev` / `npm run build` | No dedicated test suite currently | `cd react-frontend && npm run lint` |
| Angular frontend | `cd frontend && npm start` / `npm run build` | `cd frontend && npm test` | TypeScript strict mode |
| File parser | `cd backend/file-parser && clojure -M:run-m` | `cd backend/file-parser && clojure -T:build test` | - |

## Database Migrations (Go API)

```bash
cd backend/go-pennywise-api && go run ./cmd/migrations -dir ./db/migrations up
cd backend/go-pennywise-api && go run ./cmd/migrations -dir ./db/migrations status
```

Custom baseline command is available for marking initial seed migrations as applied.

## Ports

- Go API: `5151`
- Go Gmail: `8080`
- Python MLP: `8000`
- Cipher: `5160`
- Angular dev server: `5000`
- React dev server: `5173` (Vite default)

## Environment Files

Common env locations:

- `backend/go-pennywise-api/.env`
- `backend/go-gmail/.env`
- `backend/cipher/.env`
- `react-frontend/.env*`

See `AGENTS.md` for a fuller architecture map, caveats, and service-level details.
