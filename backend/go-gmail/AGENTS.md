# Agent Guidelines for go-gmail

## Overview

`go-gmail` is a Go microservice that watches Gmail via Google Cloud Pub/Sub for bank transaction emails, parses them using regex, classifies them using an ML prediction API (`python-mlp`), and creates transactions in the Pennywise API (`go-pennywise-api`).

```
Gmail Push (Pub/Sub)
        │
        ▼
┌───────────────┐     Parse      ┌──────────┐    Predict    ┌─────────────┐
│  pubsub.go    │ ─────────────► │ parser/  │ ────────────► │ python-mlp  │
│  (subscriber) │                │ email.go │               │ /predict    │
└───────────────┘                └──────────┘               └─────────────┘
        │                                                          │
        ▼                                                          │
┌───────────────┐                                                  │
│  runner.go    │ ◄────────────────────────────────────────────────┘
│  (orchestrator)│
└───────────────┘
        │
        ▼ Create Transaction + Prediction
┌──────────────────┐
│ go-pennywise-api │
│ (REST API)       │
└──────────────────┘
```

## Project Structure

```
go-gmail/
├── main.go                     # Entry point: HTTP server + Pub/Sub listener
├── Dockerfile                  # Multi-stage Docker build
├── go.mod / go.sum
├── authInit/                   # Separate Go module for Google Cloud Functions OAuth init
│   ├── authInit.go
│   └── go.mod / go.sum
├── pkg/
│   ├── auth/auth.go            # OAuth2 config + token refresh
│   ├── config/config.go        # Environment variable loading via godotenv
│   ├── database/database.go    # Stub (unused, legacy from Firestore migration)
│   ├── gmail/
│   │   ├── service.go          # Gmail API: watch setup, message history fetch, transaction email detection
│   │   ├── gmail-transactions.go  # Type definitions (EmailData, Transaction, etc.) + dead Init()
│   │   └── auth.go             # Entirely commented out (legacy)
│   ├── parser/
│   │   ├── email.go            # Regex-based email parsing: extracts date, amount, type, text
│   │   ├── email_test.go       # Comprehensive tests (20 test cases)
│   │   └── testdata/           # Test email fixtures (5 .txt files)
│   ├── prediction/service.go   # Calls python-mlp /predict API for account/payee/category
│   ├── pennywise-api/
│   │   ├── service.go          # REST client for go-pennywise-api (transactions, predictions, users)
│   │   └── service_test.go     # Tests with httptest server mocking
│   ├── pubsub/pubsub.go        # Google Cloud Pub/Sub subscriber + event processing
│   ├── runner/runner.go        # Main pipeline orchestrator: ties all services together
│   └── storage/storage.go      # Firestore client for refresh tokens + history IDs
├── gmail-watch.go              # Entirely commented out (legacy Cloud Functions code)
└── test.go                     # Standalone payee/account resolution utilities (unused by active code)
```

## Build / Test / Lint Commands

| Action | Command |
|--------|---------|
| Build | `go build .` |
| Test all | `go test ./...` |
| Test parser only | `go test -v ./pkg/parser/...` |
| Test single | `go test -run TestName ./pkg/parser` |
| Test pennywise-api | `go test -v ./pkg/pennywise-api/...` |
| Lint | `go fmt ./... && go vet ./...` |
| Docker build | `docker build -t go-gmail .` |

## Data Flow: Email → Transaction

1. **Pub/Sub receives** Gmail push notification with `{emailAddress, historyId}`
2. **`runner.ProcessGmailHistoryId`** orchestrates the pipeline:
   - Fetches refresh token from Firestore (`storage`)
   - Exchanges for access token (`auth`)
   - Gets previous history ID from Pennywise API, updates with new one (`pennywise-api`)
   - Fetches new Gmail messages since last history ID (`gmail/service`)
   - Checks if email is a transaction alert (`gmail.IsTransactionEmail`)
   - Parses email body for amount, date, type (`parser.ParseEmail`)
   - Gets ML predictions for account, payee, category (`prediction.GetPredictedFields`)
   - Creates transaction + prediction record via API (`pennywise-api`)

## Code Style & Patterns

### Service Pattern
All packages follow a constructor-based service pattern:

```go
type Service struct {
    config *config.Config
}

func NewService(config *config.Config) *Service {
    return &Service{config: config}
}
```

### Error Handling
- Return `error` as last value, always check
- Use `fmt.Errorf("context: %w", err)` for wrapping
- Runner returns errors to stop processing; individual parse failures log and continue

### Naming
- **Packages**: lowercase single word (`parser`, `runner`, `storage`, `prediction`)
- **Structs**: PascalCase (`EmailParser`, `PredictedFields`)
- **Methods**: PascalCase exported, camelCase private (`ParseEmail`, `extractDate`)

### Email Parsing (`pkg/parser/email.go`)
Email parsing uses compiled regex patterns. **Order matters**: `extractType` must run before `extractAmount` because amount sign depends on transaction type (debit = negative).

```go
parser := NewEmailParser()  // compiles regexes once
details, err := parser.ParseEmail(htmlBody)
// details.Amount is negative for debits, positive for credits
```

### ML Prediction (`pkg/prediction/service.go`)
Predictions cascade with a confidence threshold (0.7):
1. Predict account → if confidence < 0.7, use fallback, stop
2. Predict payee → if confidence < 0.7, use "Unexpected", stop
3. Predict category → if confidence < 0.7, use "❗ Unexpected expenses"

### Pennywise API Client (`pkg/pennywise-api/service.go`)
All Pennywise API calls go through `makePennywiseRequest`, which:
- Encodes query parameters
- Sets `Content-Type: application/json` and `X-Budget-ID` header
- Handles both array and object JSON responses

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | Google OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth2 client secret |
| `CALLBACK_URL` | OAuth2 redirect URL |
| `GCLOUD_SECRETS_FILE` | Path to GCP service account JSON |
| `PROJECT_ID` | Google Cloud project ID |
| `PUBSUB_TOPIC` | Gmail push notification topic name |
| `SUB_NAME` | Pub/Sub subscription name |
| `DATABASE_URL` | Database connection string |
| `MLP_API` | python-mlp service URL (e.g. `http://localhost:8000`) |
| `PENNYWISE_API` | go-pennywise-api URL (e.g. `http://localhost:5151`) |
| `NTFY_TOPIC` | ntfy.sh notification topic |

## Key Types

| Type | Package | Purpose |
|------|---------|---------|
| `parser.EmailDetails` | `pkg/parser` | Parsed email: text, date, amount, transaction type |
| `prediction.PredictedFields` | `pkg/prediction` | ML predictions with confidence scores |
| `pennywise.Transaction` | `pkg/pennywise-api` | Transaction sent to/from Pennywise API |
| `pennywise.PredictionReq` | `pkg/pennywise-api` | ML prediction audit record |
| `runner.EventData` | `pkg/runner` | Pub/Sub event: email + historyId |

## Known Issues & Tech Debt

- `gmail-watch.go`, `pkg/gmail/auth.go`: Entirely commented out legacy Cloud Functions code — should be deleted
- `pkg/gmail/gmail-transactions.go`: `Init()` returns nil, types duplicate `parser.EmailDetails`
- `pkg/database/database.go`: Unused stub
- `test.go`: Contains active payee/account resolution functions not used by the active pipeline
- `pennywise-api/service.go`: `X-Budget-ID` header is hardcoded
- `pennywise-api/service.go`: Creates a new `http.Client` per request instead of reusing
- `authInit/`: Separate Go module — only used for Google Cloud Functions deployment, not part of the main build

## Testing

- **Parser tests** (`pkg/parser/email_test.go`): 20 test cases covering date/amount/type extraction and full email parsing with fixture files in `testdata/`
- **Pennywise API tests** (`pkg/pennywise-api/service_test.go`): Integration tests using `httptest.NewServer` to mock the Pennywise API
- When adding new email formats, add a test fixture in `pkg/parser/testdata/` and corresponding test cases
