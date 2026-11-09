# ingest

Receives messages that might be bank transactions (email, SMS, direct API calls), normalises them and queues them for cipher. It does not classify or parse anything itself.

Status: mailboxes connect from the web app and sync into the job queue. The Pub/Sub receiver is still commented out in `main`.

## Commands
- `make run` / `make build` (binary in `./bin`) / `make test` / `make check` (fmt + vet + test)
- `make vendor` re-vendors shared (`GOWORK=off go mod vendor`). Docker builds with `-mod=vendor`, so re-vendor after any `backend/shared` change.

## Layout
- `cmd/api/main.go`: wiring only (config, DB, job queue, auth, routes, graceful shutdown on SIGINT/SIGTERM via `signal.NotifyContext`)
- `internal/config/`: env loading (`.env` via godotenv)
- `internal/handler/`: HTTP → `ports.InboundMessage`. `api.go` is `POST /api/ingest/messages` (budget from `X-Budget-ID`, API keys need `write`), `POST /api/ingest/sms`, same auth, called directly by the Android app with the user's JWT (so `JWT_SECRET` must match the API's, and the ID hashes sender + body + `receivedAt` so live and inbox copies dedupe), and `POST /api/ingest/test`, same auth, which queues a `mailbox.changed` job for a mailbox connected to that budget, as a push would. `GET`/`POST /api/ingest/connections` list a budget's mailboxes and connect one from a consent code (web client only; go-pennywise-api forwards them for the web app, so its token must be in `SERVICE_TOKENS`). `PATCH /api/ingest/connections/:id` pauses or resumes one and `DELETE` removes it from the budget. `GET`/`POST /api/ingest/connections/:id/backfill` list the last 14 days of mail, no bodies and at most 500, and queue the chosen ones through `Accept`
- `internal/service/`: `IngestService.Accept` sanitizes emails and queues one `model.JobTypeMessageReceived` job per budget. Queue messages only through `Accept`: it owns the dedupe key
- `internal/factory/`: builds adapters from config. `NewMailProviders` returns a registry keyed by `Connection.Provider`; the only package outside `internal/adapters` that imports an adapter
- `internal/pubsub/`: Gmail push receiver (moved from go-gmail). Each push → `mailbox.changed` job keyed `gmail:account:historyId`, then ack; malformed pushes are acked, enqueue failures nacked. Nothing handles `mailbox.changed` yet
- `internal/worker/`: job worker (inbound adapter). `mailbox.changed` runs `MailboxService.Sync`, `mailbox.backfill` runs `Backfill`, `mailbox.watch.start` runs `RenewWatch`; each queued message is a child of the job, so the Jobs screen shows one run
- `internal/sanitize/`: email HTML to visible text. `Accept` runs it on every email, so all paths into the pipeline agree
- `cmd/seal/`: seals a refresh token (stdin) into `mail_connections.credentials` hex, for migrating connections by hand
- `internal/ports/`: `InboundMessage` (the common message shape), `Connection`, `MailProvider`
- `internal/adapters/gmail/`: `MailProvider` for Gmail, moved from go-gmail. `Connection.Credentials` is sealed JSON (`SealCredentials`, `utils.SecretBox`); the cursor is a Gmail history ID

## Rules
- `main` builds adapters only through `shared/factory`; it never imports `shared/adapters`.
- `/api/ingest/*` requires a user (API key or JWT, `middleware.Authenticate`) or a verified service (`SERVICE_TOKENS`). A service principal never carries a user.
- Mail connections live in shared: `model.MailConnection`, `repository.MailConnectionRepository`, from `factory.ConnectDB` (`database.MailConnection`). Accounts are stored lowercase; `budget_ids` has no foreign key (arrays can't, budgets are soft-deleted), so check a budget still exists before queuing for it.
- Accepted messages are enqueued as `message.received` jobs (every source: email, SMS, API) (payload `model.ReceivedMessage`, in shared so cipher can read it), one per budget. Dedupe is the job idempotency key `budget:source:provider:externalId`, not memory. Every message needs an `ExternalID`; the API derives one from content + `receivedAt` when the client sends none.
- Pausing is per mailbox, so it covers every budget the mailbox feeds. It stops the watch, and resuming starts a new one from now: mail from the pause isn't fetched. `paused_at` is apart from `status` because sync rewrites `status`.
- Each mailbox has one `job_schedules` row, `mailbox.watch:<connection id>`, that renews its watch daily at 03:00 UTC. Connecting or resuming turns it on (a failure fails the request); pausing or removing the last budget turns it off (best effort, because `RenewWatch` skips those anyway). `RunScheduler` runs in ingest's `main`. Renewal keeps the stored cursor so mail since the last sync isn't skipped.
- Removing a mailbox's last budget soft-deletes the row and wipes its credentials. Google's grant stays until the user revokes it: revoking would end every grant for this client, login included.
- `FetchSince` returns no cursor on any error: keep the old one so nothing is skipped. `MAIL_CURSOR_EXPIRED` means call `Watch` for a fresh cursor (and `Search` to fill the gap); `MAIL_AUTH_FAILED` means the user must reconnect.

## Env
- `PORT` (default `5180`), `RAILWAY_ENVIRONMENT_NAME`
- `DATABASE_URL`, `DATABASE_DRIVER`
- `JWT_SECRET` (unset = API keys only), `INTERNAL_AUTH_TOKEN` (sent), `SERVICE_TOKENS` (accepted callers)
- `JOB_DRIVER` (`postgres` | `memory` | unset = none), `JOB_BROKER_URL`, `JOB_STORE_URL`
- `MAIL_PROVIDERS` (`gmail`; unset = none), `MAIL_CREDENTIALS_KEY` (base64 32 bytes; required when any provider is set). Gmail: `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET` and/or `GOOGLE_ANDROID_CLIENT_ID`, `PROJECT_ID`, `PUBSUB_TOPIC`. Missing settings fail startup
- `SUB_NAME` (+ `PROJECT_ID`, `GOOGLE_APPLICATION_CREDENTIALS_JSON`) turns on the Gmail push receiver; unset = off
