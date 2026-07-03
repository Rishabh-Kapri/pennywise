# Agent Guidelines for go-gmail

## Overview

`go-gmail` is a Go microservice that watches Gmail via Google Cloud Pub/Sub for bank transaction emails, parses them using regex, and starts a Temporal `EmailToTransactionWorkflow` that classifies them via `cipher`'s `PredictionActivity` and creates transactions in the Pennywise API (`go-pennywise-api`). The old direct `python-mlp /predict` path is deprecated.

```
Gmail Push (Pub/Sub)
        │
        ▼
┌───────────────┐     Parse      ┌──────────┐
│  pubsub.go    │ ─────────────► │ parser/  │
│  (subscriber) │                │ email.go │
└───────────────┘                └──────────┘
        │
        ▼
┌────────────────┐   Start workflow   ┌──────────────────────────────┐
│  runner.go     │ ─────────────────► │ Temporal                     │
│  (orchestrator)│                    │ EmailToTransactionWorkflow   │
└────────────────┘                    └──────────────────────────────┘
                                          │ cipher PredictionActivity
                                          ▼
                                  ┌──────────────────┐
                                  │ go-pennywise-api │
                                  │ (REST API)       │
                                  └──────────────────┘
```

## Project Structure

```
go-gmail/
├── cmd/main.go                 # Entry point: HTTP server + Pub/Sub listener + Temporal activity worker
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
│   ├── prediction/service.go   # Legacy python-mlp /predict client (deprecated path)
│   ├── client/
│   │   ├── pennywise.go        # REST client for go-pennywise-api (users, history IDs)
│   │   └── cipher.go           # REST client for cipher
│   ├── temporal/               # Workflow starters (fetchAndParseEmail, watchGmail)
│   ├── pubsub/pubsub.go        # Google Cloud Pub/Sub subscriber + event processing
│   ├── runner/runner.go        # Main pipeline orchestrator: ties all services together
│   └── storage/storage.go      # Firestore client for refresh tokens + history IDs
└── test.go                     # Standalone payee/account resolution utilities (unused by active code)
```

## Build / Test / Lint Commands

| Action | Command |
|--------|---------|
| Build | `go build ./cmd` |
| Test all | `go test ./...` |
| Test parser only | `go test -v ./pkg/parser/...` |
| Test single | `go test -run TestName ./pkg/parser` |
| Test API clients | `go test -v ./pkg/client/...` |
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
   - Starts the Temporal `EmailToTransactionWorkflow`, which classifies via cipher's `PredictionActivity` and creates the transaction + cipher prediction through `go-pennywise-api` activities

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

### Service Clients (`pkg/client/`)
`pennywise.go` and `cipher.go` call the other Go services through the shared transport, which injects correlation/caller/origin headers and `X-Internal-Token` from context.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_ANDROID_CLIENT_ID` | Google OAuth2 clients |
| `GOOGLE_APPLICATION_CREDENTIALS_JSON` | GCP service account JSON |
| `CALLBACK_URL` | OAuth2 redirect URL |
| `PROJECT_ID` | Google Cloud project ID |
| `PUBSUB_TOPIC` | Gmail push notification topic name |
| `SUB_NAME` | Pub/Sub subscription name |
| `DATABASE_URL` | Database connection string |
| `PENNYWISE_SERVICE_URL` | go-pennywise-api URL (e.g. `http://localhost:5151`) |
| `CIPHER_SERVICE_URL` | cipher URL (e.g. `http://localhost:5160`) |
| `INTERNAL_AUTH_TOKEN` | Shared token for verified service-to-service calls |
| `TEMPORAL_SERVER_HOST` / `TEMPORAL_SERVER_PORT` | Temporal server |
| `MLP_SERVICE_URL` | python-mlp service URL (legacy, deprecated path) |
| `NTFY_TOPIC` | ntfy.sh notification topic |
| `PORT` | HTTP port (default `5170`) |

## Key Types

| Type | Package | Purpose |
|------|---------|---------|
| `parser.EmailDetails` | `pkg/parser` | Parsed email: text, date, amount, transaction type |
| `prediction.PredictedFields` | `pkg/prediction` | Legacy ML predictions with confidence scores |
| `runner.EventData` | `pkg/runner` | Pub/Sub event: email + historyId |

## Known Issues & Tech Debt

- `pkg/gmail/gmail-transactions.go`: types duplicate `parser.EmailDetails`
- `pkg/database/database.go`: Unused stub
- `pkg/prediction/`: legacy python-mlp client, kept for reference
- `test.go`: Contains payee/account resolution functions not used by the active pipeline
- `authInit/`: Separate Go module — only used for Google Cloud Functions deployment, not part of the main build

## Testing

- **Parser tests** (`pkg/parser/email_test.go`): 20 test cases covering date/amount/type extraction and full email parsing with fixture files in `testdata/`
- **Client tests** (`pkg/client/pennywise_test.go`, `pkg/client/cipher_test.go`): Integration tests using `httptest.NewServer` to mock the downstream services
- When adding new email formats, add a test fixture in `pkg/parser/testdata/` and corresponding test cases
