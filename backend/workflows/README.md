# workflows

Temporal worker and workflow definitions for the email → transaction ingestion path.

## Workflows (`internal/workflow`)

| Workflow | Task queue | Purpose |
|----------|-----------|---------|
| `EmailToTransactionWorkflow` | `PennywiseTaskQueue` | Started by `go-gmail` per Gmail push; invokes Cipher's `PredictionActivity` (on `CipherActivitiesTaskQueue`), then creates predictions/transactions via `go-pennywise-api` activities |
| `ParsedEmailToTransactionWorkflow` | `PennywiseTaskQueue` | Same pipeline, starting from already-parsed email data |
| `RefreshGmailWatchWorkflow` | `PennywiseTaskQueue` | Periodically renews the Gmail Pub/Sub watch |

Activities are owned by the services themselves: Cipher registers `PredictionActivity`, the Go API registers create-prediction/create-transaction activities. Correlation metadata is propagated across hops by `backend/shared/temporal/propagator.go`.

## Run

```bash
make run    # go run ./cmd/worker
make build  # binary into ./bin
make test
```

Requires a running Temporal server (`TEMPORAL_SERVER_HOST`/`TEMPORAL_SERVER_PORT`; the compose stack provides one with UI on port 8233).
