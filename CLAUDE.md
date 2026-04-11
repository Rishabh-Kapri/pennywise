# CLAUDE.md

## Project Overview

Pennywise is a personal finance/budgeting app with ML-powered transaction classification from email parsing. Monorepo with 5 services:

- **go-pennywise-api** (`backend/go-pennywise-api`): Core REST API (Gin, PostgreSQL/pgx). Handler → service → repository layers.
- **go-gmail** (`backend/go-gmail`): Gmail Pub/Sub watcher, parses bank emails with regex, creates transactions.
- **python-mlp** (`backend/python-mlp`): MLP + sentence-transformer models for payee/category/account prediction.
- **frontend** (`frontend`): Angular 17 + NGXS state management.
- **react-frontend** (`react-frontend`): React 19 + Vite + Redux Toolkit (active development).
- **file-parser** (`backend/file-parser`): Clojure service for bulk transaction uploads.

## Build & Test Commands

```bash
# Go API
cd backend/go-pennywise-api && go build ./cmd/api
cd backend/go-pennywise-api && go test ./...
cd backend/go-pennywise-api && go test -run TestName ./internal/service
cd backend/go-pennywise-api && go fmt ./... && go vet ./...

# Go Gmail
cd backend/go-gmail && go build
cd backend/go-gmail && go test ./...
cd backend/go-gmail && go test -run TestName ./pkg/parser
cd backend/go-gmail && go fmt ./... && go vet ./...

# React Frontend
cd react-frontend && npm run build
cd react-frontend && npm test
cd react-frontend && npm test -- filename

# Angular Frontend
cd frontend && npm run build
cd frontend && npm test
cd frontend && npx ng test --include="**/name.spec.ts"

# Full Stack
docker-compose up --build
```

## Key Data Flow

1. Gmail push → `go-gmail` parses email (regex in `pkg/parser/email.go`) → calls `python-mlp /predict` → creates transaction via `go-pennywise-api`
2. All API calls require `X-Budget-ID` header — extracted via `utils.GetBudgetId(c)` in handlers
3. ML prediction corrections tracked in `internal/service/transaction.go` (`UserCorrectedPayee`, `UserCorrectedCategory`, etc.)

## Code Conventions

### Go (Gin Framework)
- **Layering**: Handler (parse request, return JSON) → Service (business logic) → Repository (DB operations)
- **Imports**: stdlib → third-party (gin, uuid, pgx) → local packages, separated by blank lines
- **Error handling**: Return `gin.H{"error": err.Error()}` with appropriate HTTP status
- **Database transactions**: Use `*Tx` suffixed repository methods when atomicity is required
- **Naming**: PascalCase exported, camelCase private

### React Frontend
- **State**: Redux Toolkit with feature-based slices in `features/*/store/`
- **API**: `apiClient` singleton in `src/utils/api.ts` auto-injects `x-budget-id` header
- **Structure**: Feature folders (`features/transactions/`, `features/budget/`) with `components/`, `hooks/`, `store/`, `types/`
- **Hooks**: Use typed `useAppDispatch` and `useAppSelector` from `src/app/hooks.ts`
- **UI**: HeroUI components, Lucide icons, Tailwind CSS v4, Recharts for charts

### Angular Frontend
- **State**: NGXS actions/selectors in `store/dashboard/states/`
- **DI**: Constructor injection (not `inject()`)
- **Types**: Explicit interfaces in `src/app/models/`
- **Styling**: SCSS + Tailwind CSS

### TypeScript (Both Frontends)
- Strict mode enabled
- Explicit interfaces for all models

### Cross-Service Communication
- `go-gmail` → `python-mlp`: HTTP POST to `/predict` with `{type, email_text, amount}`
- `go-gmail` → `go-pennywise-api`: HTTP calls via `pkg/pennywise-api/`
- Frontend → API: REST with budget ID in header interceptor

## Key Files

| Purpose | Path |
|---------|------|
| API routes | `backend/go-pennywise-api/cmd/api/main.go` |
| Email parsing | `backend/go-gmail/pkg/parser/email.go` |
| Transaction model (Go) | `backend/go-pennywise-api/internal/model/transaction.go` |
| Transaction model (TS) | `frontend/src/app/models/transaction.model.ts` |
| React API client | `react-frontend/src/utils/api.ts` |
| React Redux store | `react-frontend/src/app/store.ts` |
| Docker Compose | `docker-compose.yml` |
| CI/CD | `.github/workflows/workflow.yml` |

## Maintenance

After completing any new feature, bug fix, or task, update this CLAUDE.md file if the change affects architecture, conventions, key files, build commands, or data flow.

## Environment

- **Database**: PostgreSQL via pgx (`internal/db/db.go`)
- **Auth**: None currently. Google OAuth planned.
- **Deployment**: Docker Compose on self-hosted Unraid, deployed via GitHub Actions CI. Secondary: Railway.app for Go API.
- **Env files**: `backend/go-gmail/.env`, `backend/go-pennywise-api/.env`
