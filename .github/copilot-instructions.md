# Pennywise - AI Coding Instructions

## Architecture Overview

Pennywise is a **personal finance/budgeting application** with ML-powered transaction classification from email parsing. The system consists of 4 services:

```
┌─────────────────┐    Gmail Push    ┌──────────────┐    Predict    ┌─────────────┐
│  Gmail + PubSub │ ───────────────► │  go-gmail    │ ────────────► │ python-mlp  │
└─────────────────┘                  └──────────────┘               └─────────────┘
                                            │                              
                                            │ Create Transaction           
                                            ▼                              
┌─────────────────┐                  ┌──────────────────┐           
│ Angular/React   │ ◄───────────────►│ go-pennywise-api │ (PostgreSQL)
│ Frontend        │   REST API       │                  │           
└─────────────────┘                  └──────────────────┘           
```

### Service Responsibilities
- **go-pennywise-api** (`backend/go-pennywise-api`): Core REST API with handler→service→repository layering
- **go-gmail** (`backend/go-gmail`): Watches Gmail via Pub/Sub, parses bank emails, creates transactions
- **python-mlp** (`backend/python-mlp`): MLP model predicting payee/category/account from email text
- **frontend** (`frontend`): Angular 17 app with NGXS state management (deployed)
- **react-frontend** (`react-frontend`): React 18 + Vite + Redux Toolkit (deployed, active development)

> **Auth**: Currently none. Google OAuth planned for future.

## Key Data Flow

1. **Email → Transaction**: Gmail push notification → `go-gmail` parses email via regex patterns in [pkg/parser/email.go](backend/go-gmail/pkg/parser/email.go) → calls `python-mlp` for predictions → creates transaction via `go-pennywise-api`
2. **Budget Context**: All API calls require `X-Budget-ID` header - extracted via `utils.GetBudgetId(c)` in handlers
3. **ML Prediction Corrections**: When user updates an ML-created transaction, the `updatePrediction` logic in [transaction.go](backend/go-pennywise-api/internal/service/transaction.go) tracks corrections (`UserCorrectedPayee`, `UserCorrectedCategory`, etc.) for model improvement
4. **State Management**: Angular uses NGXS (`store/dashboard/states/`), React uses Redux Toolkit (`features/*/store/`)

## Build & Test Commands

| Service | Build | Test Single | Test All |
|---------|-------|-------------|----------|
| Angular Frontend | `cd frontend && npm run build` | `ng test --include="**/name.spec.ts"` | `npm test` |
| React Frontend | `cd react-frontend && npm run build` | `npm test -- filename` | `npm test` |
| Go API | `cd backend/go-pennywise-api && go build ./cmd/api` | `go test -run TestName ./internal/service` | `go test ./...` |
| Go Gmail | `cd backend/go-gmail && go build` | `go test -run TestName ./pkg/parser` | `go test ./...` |
| Docker | `docker-compose up --build` | - | - |

## Code Patterns

### Go Backend (Gin Framework)
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

### Angular Frontend
- **State**: Use NGXS actions/selectors, defined per feature in `store/dashboard/states/`
- **Services**: `HttpService` wraps HttpClient with base URL from environment
- **Models**: Explicit interfaces in [frontend/src/app/models](frontend/src/app/models/)
- **DI**: Constructor injection (not `inject()`) for consistency with existing code

### React Frontend (Active Development)
- **State**: Redux Toolkit with feature-based slices in `features/*/store/`
- **API**: `apiClient` singleton in [src/utils/api.ts](react-frontend/src/utils/api.ts) - auto-injects `x-budget-id` header
- **Structure**: Feature folders (`features/transactions/`, `features/budget/`) contain `components/`, `hooks/`, `store/`, `types/`
- **Middleware**: Custom middlewares in [src/app/middlewares.ts](react-frontend/src/app/middlewares.ts) for data fetching, date changes
- **Hooks**: Use typed `useAppDispatch` and `useAppSelector` from [src/app/hooks.ts](react-frontend/src/app/hooks.ts)

### Cross-Service Communication
- **go-gmail → python-mlp**: HTTP POST to `/predict` with `{type, email_text, amount}`
- **go-gmail → go-pennywise-api**: HTTP calls via [pkg/pennywise-api](backend/go-gmail/pkg/pennywise-api/)
- **Frontend → API**: REST via `HttpService`, budget ID in header interceptor

## Important Conventions

1. **Go imports**: stdlib → third-party (gin, uuid) → local packages
2. **Error handling**: Return `gin.H{"error": err.Error()}` with appropriate HTTP status
3. **Database transactions**: Use `*Tx` suffixed methods when atomicity required
4. **TypeScript**: Strict mode enabled, explicit interfaces for all models
5. **Styling**: Tailwind CSS + component-scoped SCSS

## Key Files Reference

- API routes: [cmd/api/main.go](backend/go-pennywise-api/cmd/api/main.go#L77-148)
- Email parsing patterns: [pkg/parser/email.go](backend/go-gmail/pkg/parser/email.go#L34-41)
- Frontend state config: [store/store.config.ts](frontend/src/app/store/store.config.ts)
- Transaction model (Go): [internal/model/transaction.go](backend/go-pennywise-api/internal/model/transaction.go)
- Transaction model (TS): [models/transaction.model.ts](frontend/src/app/models/transaction.model.ts)
