# Agent Guidelines for Pennywise

## Overview

Pennywise is a **personal finance/budgeting application** with ML-powered transaction classification from email parsing. The system is a monorepo with 4 microservices:

```
┌─────────────────┐    Gmail Push    ┌──────────────┐    Predict    ┌─────────────┐
│  Gmail + PubSub │ ───────────────► │  go-gmail    │ ────────────► │ python-mlp  │
└─────────────────┘                  └──────────────┘               └─────────────┘
                                            │
                                            │ Create Transaction
                                            ▼
┌─────────────────┐                  ┌──────────────────┐
│ Angular/React   │ ◄──────────────► │ go-pennywise-api │ (PostgreSQL)
│ Frontend        │   REST API       │                  │
└─────────────────┘                  └──────────────────┘
```

### Service Responsibilities

- **go-pennywise-api** (`backend/go-pennywise-api`): Core REST API with handler -> service -> repository layering. PostgreSQL via pgx.
- **go-gmail** (`backend/go-gmail`): Watches Gmail via Pub/Sub, parses bank emails with regex, creates transactions via the API.
- **python-mlp** (`backend/python-mlp`): MLP + sentence-transformer models predicting payee/category/account from email text.
- **frontend** (`frontend`): Angular 17 app with NGXS state management.
- **react-frontend** (`react-frontend`): React 19 + Vite + Redux Toolkit (active development).
- **file-parser** (`backend/file-parser`): Clojure service for bulk-uploading transactions from files.

## Key Data Flow

1. **Email -> Transaction**: Gmail push notification -> `go-gmail` parses email via regex patterns in `backend/go-gmail/pkg/parser/email.go` -> calls `python-mlp /predict` endpoint -> creates transaction via `go-pennywise-api`.
2. **Budget Context**: All API calls require `X-Budget-ID` header - extracted via `utils.GetBudgetId(c)` in handlers.
3. **ML Prediction Corrections**: When a user updates an ML-created transaction, `updatePrediction` logic in `backend/go-pennywise-api/internal/service/transaction.go` tracks corrections (`UserCorrectedPayee`, `UserCorrectedCategory`, etc.) for model improvement.
4. **State Management**: Angular uses NGXS (`store/dashboard/states/`), React uses Redux Toolkit (`features/*/store/`).

## Build/Lint/Test Commands

| Service | Build | Test Single | Test All | Lint |
|---------|-------|-------------|----------|------|
| Angular Frontend | `cd frontend && npm run build` | `ng test --include="**/name.spec.ts"` | `npm test` | Follow TypeScript strict mode |
| React Frontend | `cd react-frontend && npm run build` | `npm test -- filename` | `npm test` | Follow TypeScript strict mode |
| Go API | `cd backend/go-pennywise-api && go build ./cmd/api` | `go test -run TestName ./internal/service` | `go test ./...` | `go fmt ./... && go vet ./...` |
| Go Gmail | `cd backend/go-gmail && go build` | `go test -run TestName ./pkg/parser` | `go test ./...` | `go fmt ./... && go vet ./...` |
| Python MLP | `cd backend/python-mlp && pip install -r requirements.txt && python mlp_predict_server.py` | - | - | - |
| Docker (Full Stack) | `docker-compose up --build` | - | - | - |

### Development Servers

- **Angular Frontend**: `cd frontend && npm start` (port 5000)
- **React Frontend**: `cd react-frontend && npm run dev`
- **Go API**: Built binary runs on port 5151
- **Python MLP**: `python mlp_predict_server.py` (port 8000)
- **Docker Compose ports**: Frontend (5000), Gmail (5000), MLP (5050), API (5151)

## Code Patterns & Style

### Go API (Gin Framework)

```go
// Handler: Parse request, call service, return JSON response
func (h *transactionHandler) Create(c *gin.Context) {
    ctx, err := utils.GetBudgetId(c)  // Always extract budget context first
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // ... service call
}

// Service: Business logic, orchestrates repositories
type TransactionService interface {
    Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error)
}

// Repository: Database operations with optional transaction support
func (r *transactionRepo) GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error)
```

- **Structure**: `internal/` for private API code, `pkg/` for shared utilities
- **Imports**: Standard library, third-party (gin, uuid, pgx), then local packages - grouped with blank lines
- **Naming**: PascalCase for exported, camelCase for private (Go conventions)
- **Error handling**: Return error as last value, handle all errors with `gin.H{"error": err.Error()}` and appropriate HTTP status
- **Database transactions**: Use `*Tx` suffixed repository methods when atomicity is required
- **Interfaces**: Define in handler/service layers, implement dependency injection via constructors

### Angular Frontend (TypeScript)

- **Components**: Mix of standalone and module-based components (follow existing patterns in each area)
- **State**: NGXS actions/selectors, defined per feature in `store/dashboard/states/`
- **Services**: `HttpService` wraps HttpClient with base URL from environment
- **DI**: Constructor injection preferred (existing codebase pattern), avoid `inject()` for consistency
- **Imports**: Angular core first, third-party libraries, then local imports (models, services, constants)
- **Naming**: PascalCase for classes/interfaces, camelCase for methods/properties, kebab-case for files
- **Types**: Explicit interfaces for all models in `frontend/src/app/models/`, strict TypeScript mode enabled
- **Styling**: SCSS files with Tailwind CSS classes, component-scoped styles

### React Frontend (Active Development)

- **State**: Redux Toolkit with feature-based slices in `features/*/store/`
- **API**: `apiClient` singleton in `react-frontend/src/utils/api.ts` - auto-injects `x-budget-id` header
- **Structure**: Feature folders (`features/transactions/`, `features/budget/`) containing `components/`, `hooks/`, `store/`, `types/`
- **Middleware**: Custom middlewares in `react-frontend/src/app/middlewares.ts` for data fetching and date changes
- **Hooks**: Use typed `useAppDispatch` and `useAppSelector` from `react-frontend/src/app/hooks.ts`
- **UI**: HeroUI components, Lucide icons, Tailwind CSS v4, Recharts for charts
- **Naming**: PascalCase for components, camelCase for hooks/utilities, kebab-case for files

### Python MLP

- **ML Models**: Sentence-transformers for embeddings and similarity matching
- **API**: HTTP server exposing `/predict` endpoint
- **File Structure**: Separate `.parms` model files for payee, category, and account prediction

### Cross-Service Communication

- **go-gmail -> python-mlp**: HTTP POST to `/predict` with `{type, email_text, amount}`
- **go-gmail -> go-pennywise-api**: HTTP calls via `backend/go-gmail/pkg/pennywise-api/`
- **Frontend -> API**: REST via `HttpService` (Angular) or `apiClient` (React), budget ID injected via header interceptor

## Key Files Reference

| Purpose | Path |
|---------|------|
| API routes | `backend/go-pennywise-api/cmd/api/main.go` |
| Email parsing patterns | `backend/go-gmail/pkg/parser/email.go` |
| Transaction model (Go) | `backend/go-pennywise-api/internal/model/transaction.go` |
| Transaction model (TS) | `frontend/src/app/models/transaction.model.ts` |
| Angular state config | `frontend/src/app/store/store.config.ts` |
| React API client | `react-frontend/src/utils/api.ts` |
| React Redux store | `react-frontend/src/app/store.ts` |
| React middlewares | `react-frontend/src/app/middlewares.ts` |
| Docker Compose | `docker-compose.yml` |
| CI/CD workflow | `.github/workflows/workflow.yml` |

## Environment & Deployment

### Required Environment Files

- `backend/go-gmail/.env`: Gmail API credentials and Google Cloud configuration
- `backend/go-pennywise-api/.env`: PostgreSQL connection string and API configuration
- Service account JSON for Google Cloud Platform authentication

### Database

- **PostgreSQL**: Primary database for Go API, connected via pgx driver (`internal/db/db.go`)
- **Firestore**: Legacy - Angular frontend has Firestore integration; migration to PostgreSQL via `cmd/migrations/main.go`

### Deployment

- **Primary**: Docker Compose on self-hosted Unraid server, deployed via GitHub Actions CI (`.github/workflows/workflow.yml`)
- **Secondary**: Railway.app for Go API (`railway.json`)
- CI workflow detects which services changed and selectively rebuilds via docker-compose

## Testing Strategy

### Angular Frontend
- Karma + Jasmine for unit tests
- Component testing with Angular testing utilities
- Test files co-located with components (`.spec.ts`)

### React Frontend
- Test files co-located with source code

### Go Backend
- Go standard testing package with `testify` for assertions
- Repository layer testing with database mocks
- Service layer business logic testing
- HTTP handler testing with Gin test context
- Email parser has comprehensive tests with test data files in `pkg/parser/`

### Python MLP
- Model evaluation via `process_data.py` training pipeline
