# workflows

Temporal worker (`pennywise-tasks` queue) that defines the email-to-transaction workflows. It registers workflows only; activities run in go-gmail, cipher, and go-pennywise-api on their own queues.

## Commands
- `make run` / `make build` (binary in `./bin/worker`) / `make test` / `make check`
- `make vendor` re-vendors shared modules (`GOWORK=off go mod vendor`). Re-vendor after any `backend/shared` change.

## Layout
- `cmd/worker/main.go`: dials Temporal, registers the three workflows by explicit name
- `internal/workflow/emailToTransactionWorkflow.go`: `EmailToTransactionWorkflow` (parent) and `ParsedEmailToTransactionWorkflow` (per-email child)
- `internal/workflow/pipelineStatus.go`: best-effort run tracking helpers (`pipelineReporter`)
- `internal/workflow/gmailWatchWorkflow.go`: `RefreshGmailWatchWorkflow`, scheduled every 2 days
- `internal/model/`: empty

## Activities and queues
| Queue | Activities |
|---|---|
| `pennywise-activities` (go-pennywise-api) | `GetGoogleUserByEmail`, `UpdateGmailHistoryID`, `CreateTransactionAndCipherPrediction`, `StartPipelineRun`, `ReportPipelineStatus`, `ListGoogleUsersNeedingWatchRefresh`, `UpdateGmailWatchState` |
| `gmail-activities` (go-gmail) | `FetchEmailData`, `GmailWatchCall` |
| `cipher-activities` (cipher) | `ParseEmail`, `PredictEmail` (per-email); `ParseEmailData`, `Predict` (legacy batch) |

## Invariants
- Parent runs in `PennywiseTaskQueue`. Child ID is `<parent-id>-parsed`.
- One email per activity: each `ParseEmail`/`PredictEmail` handles a single email, so one bad email fails alone. Successes are committed right away through `CreateTransactionAndCipherPrediction` (deduped on budget + hash, so retries are safe).
- Per-email cipher activities: 15m StartToClose, 4m HeartbeatTimeout, 3 attempts. Skips are recorded, not retried.
- Emails that still fail park the workflow on signals `retry-email-parse` / `retry-predict` (`shared/model/temporal.go`) for up to `RetryPredictWaitTimeout` (24h). Only the parked emails re-run.
- Version gates: `workflow.GetVersion` markers must stay in place for replay of in-flight executions.
  - `pipeline-observability` (parent and child): gates all run-tracking activity calls.
  - `per-email-pipeline` (child only): `DefaultVersion` replays the legacy batch `ParseEmailData`/`Predict` path. Keep the legacy activities registered in cipher while any such run can still be parked.
- Pipeline status (`pipelineStatus.go`) is observability only. Its activities use 3 attempts and errors are swallowed. A zero run ID disables reporting. Never let a status write fail the pipeline.
- The parent uses the same status helpers as the child. Standalone child runs (no parent `PipelineRunID`) create their own run with trigger `manual`.
- Correlation: `correlation_id` and `origin_service` cross Temporal hops through `backend/shared/temporal` (`ContextPropagators()`). The worker and go-gmail both must register it.

## Env
- `TEMPORAL_SERVER_HOST`, `TEMPORAL_SERVER_PORT` (loaded from `.env`)

## Key files
| Purpose | Path |
|---|---|
| Parent + per-email child workflows | `internal/workflow/emailToTransactionWorkflow.go` |
| Status reporting helpers | `internal/workflow/pipelineStatus.go` |
| Worker registration | `cmd/worker/main.go` |
| Signal names, queues, timeouts | `backend/shared/model/temporal.go` |
| Pipeline run model | `backend/shared/model/pipeline.go` |
| Propagator | `backend/shared/temporal/propagator.go` |
| Tests | `internal/workflow/emailToTransactionWorkflow_test.go` |
