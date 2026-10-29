# go-gmail

Gmail Pub/Sub listener and Gmail activity worker. Starts `EmailToTransactionWorkflow` for each new Gmail history ID; it does not classify or parse emails itself.

## Commands
- `make run` / `make build` (binary in `./bin`) / `make test` / `make check` (fmt + vet + test)
- `make vendor` re-vendors shared modules (`GOWORK=off go mod vendor`). Docker builds with `-mod=vendor`, so re-vendor after any `backend/shared` change.

## Layout
- `cmd/main.go`: HTTP API (`/api/watch`, `/api/temporal`, `/api/test/setup-watch`), Temporal activity worker on `gmail-activities`
- `pkg/pubsub/`: Pub/Sub subscriber; the only place that starts `EmailToTransactionWorkflow` in production
- `pkg/temporal/`: activities `FetchEmailData` (Gmail history fetch) and `GmailWatchCall`
- `pkg/gmail/`, `pkg/auth/`: Gmail API and OAuth token refresh
- `pkg/client/`: REST clients for go-pennywise-api and cipher (shared transport, internal headers)
- `pkg/config/`: env loading (`.env` via godotenv)
- `pkg/parser/`: DEPRECATED regex parser (`email.go`); only constructed, not on the active path. Do not extend it; cipher's `ParseEmail` replaces it.
- Dead code, do not build on it: `pkg/runner/` (not imported), `pkg/prediction/` (legacy python-mlp), `pkg/database/` (stub), `pkg/storage/`, `authInit/` (separate module)

## Invariants
- Workflow ID is `email-to-txn-<email>-<historyId>` (set in `pkg/pubsub`), with `ALLOW_DUPLICATE_FAILED_ONLY` reuse and `USE_EXISTING` conflict policies. Redelivered pushes dedupe at Temporal; the in-memory maps are only a fast path. Keep these policies: a completed run must not rerun.
- The `/api/temporal` HTTP handler starts the workflow WITHOUT an ID, so it has no dedupe. Use it for manual testing only.
- `/api/backfill` (onboarding, called by the API) lists recent bank alerts with a Gmail search (`backfillQuery`, capped by days and count) and starts `ParsedEmailToTransactionWorkflow` directly with `backfill-<budget>-<unix>` as the ID. It has no history id and no parent, so the child records its own pipeline run (trigger `manual`). Transactions dedupe on `(budget_id, dedupe_hash)`, so overlapping a push is safe.
- `FetchEmailData` is the only Gmail read in the pipeline. It returns raw bodies plus `From`/`Subject`/`Snippet` headers. Parsing happens in cipher (`ParseEmail`, one email per activity).
- Activities restamp `gmail-pubsub` as the service name (`utils.WithServiceName`) before calling downstream HTTP.
- Production runs two activity workers on `gmail-activities` in one process: `cmd/main.go` and `pubsub.PullMessages`, both registering `GmailActivities`. Keep registrations identical in both.
- The worker and the subscriber start only when `RAILWAY_ENVIRONMENT_NAME` is not `local` (default `local`). Locally, only the HTTP routes run.
- Gmail watch is refreshed by `RefreshGmailWatchWorkflow` (workflows service) calling `GmailWatchCall` here.

## Env
- Gmail/GCP: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_ANDROID_CLIENT_ID`, `GOOGLE_APPLICATION_CREDENTIALS_JSON`, `PROJECT_ID`, `SUB_NAME`
- Services: `PENNYWISE_SERVICE_URL`, `CIPHER_SERVICE_URL`, `INTERNAL_AUTH_TOKEN`, `DATABASE_URL`
- Temporal: `TEMPORAL_SERVER_HOST`, `TEMPORAL_SERVER_PORT`
- `PORT` (default `5170`), `RAILWAY_ENVIRONMENT_NAME`

## Key files
| Purpose | Path |
|---|---|
| Workflow starter + ID/reuse policy | `pkg/pubsub/pubsub.go` |
| Raw email fetch activity | `pkg/temporal/fetchAndParseEmail.go` |
| Worker + HTTP routes | `cmd/main.go` |
| Temporal propagator (shared) | `backend/shared/temporal/propagator.go` |
| Workflow definitions | `backend/workflows/internal/workflow/emailToTransactionWorkflow.go` |
