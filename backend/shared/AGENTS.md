# AGENTS.md — backend/shared

Shared Go module (`github.com/Rishabh-Kapri/pennywise/backend/shared`): repository ports and their Postgres adapter, the queue port and its Redis adapter, models, HTTP transport, middleware, logging, Temporal propagator, utils. Imported by cipher, go-pennywise-api, go-gmail, workflows.

## Test

```bash
cd backend/shared && go test ./...   # or: make check (fmt + vet + test)
```

Postgres integration tests (`adapters/postgres/*_integration_test.go`) skip unless `PENNYWISE_TEST_DATABASE_URL` names a role that can `CREATE DATABASE`; each run creates a throwaway database, applies `migrations/sql`, and drops it. Use the `pgvector/pgvector:pg17` image (see `integration_test.go`).

Redis queue integration tests (`adapters/redis/*_integration_test.go`) skip unless `PENNYWISE_TEST_REDIS_URL` is set (Redis 7+; each test uses its own key prefix).

## Layout

- `ports/repository/` — repository ports: one interface per aggregate, plus `DBTxn`, `BulkInserter`, `ErrNotFound`. No driver types. `MailConnectionRepository` (table `mail_connections`) holds ingest's connected mailboxes: `budget_ids` is an array without a foreign key, so readers must skip budgets that no longer exist
- `adapters/postgres/` — the Postgres adapter for those ports (pgx + squirrel), one file per aggregate
- `ports/queue/` — work-queue port (`Publisher`, `Consumer`, `Queue`), its options, and `Permanent` for errors that skip retries. Semantics (at-least-once, competing consumers, retry → dead-letter) are in the package doc
- `ports/job/` — job port (`Broker`, `JobStore`) with the Postgres adapter in `adapters/postgres/job.go` and an in-memory one in `adapters/memory/`. `factory.ConnectJob` wraps the broker in `job.ObservedBroker`, which records each state change as a `job_events` row through the store's `JobLog` (Postgres, at `JOB_STORE_URL`). Every job is on a named queue. There is no routing: `Enqueue` needs `job.WithQueue` and refuses a job without one; to send a type to several queues, enqueue once per queue (dedupe is per `(queue, type, idempotency_key)`). `ClaimJob` takes a list of queues. Queue names and job types are constants in `model/jobs.go`; the message pipeline's payloads are in `model/messagePipeline.go`. Handlers use the helpers in `jobrunner/handler.go`. `JobStore.ListTrees` and `Overview` take a `JobScope`; anything serving a user must use them rather than `ListJobs`, which lists every budget. Broker adapters write no history. Jobs carry `job.Metadata` (correlation ID, origin service: the same two fields the Temporal propagator carries), captured at enqueue by `job.MetadataFromContext`; restore it with `Metadata.Context` before running a job. Never put the user, budget or internal-verified flag in it: a stored value must not grant trust. With `JOB_DRIVER=memory` the history is printed to the console (`memory.NewMemoryJobLogAdapter`) and there is no `JobStore`
- `adapters/redis/` — Redis Streams adapter for the queue port; key layout and Lua scripts documented in `queue.go`
- `factory/` — `ConnectDB(Config)`: picks the adapter from `DATABASE_DRIVER` (default `postgres`) and returns the ports. `ConnectQueue(ctx, QueueConfig)` does the same for the queue from `QUEUE_DRIVER` (default `redis`) and `REDIS_URL`. The only package outside `adapters/` that imports an adapter
- `migrations/sql/` — embedded goose migrations for Postgres; how to write one: `migrations/sql/README.md`. Run by `go-pennywise-api/cmd/migrations`. Other databases get a sibling (`migrations/mongodb/`)
- `model/` — domain structs and enums
- `pipeline/` — the system pipelines (messages today; statements, PDFs, background agents later): one file per pipeline in `pipelines/<id>.json`, embedded and validated at `Load` against `pipeline.schema.json` (point an editor at it with `$schema`), plus Go checks the schema can't express (unique IDs, default adapter in its list). `Resolve`/`ValidateOverride` apply a budget's `budget_pipeline_steps` row. The API serves them at `GET /api/pipelines`; the service that runs a pipeline must check at startup that it knows every step. Pipeline, step and adapter IDs are stored in overrides: never rename one, and bump a pipeline's `version` when its steps change. A model adapter declares `modelKind` (`chat` or `decision`); its step uses the override's model if it's still usable and of that kind, else the owner's default of the kind (`DefaultOf`). A step's `fallback` (file default, budget override) says whether a model adapter tries the owner's other models of the kind when its model fails; `Attempts` turns that into the ordered list a runner chains over (the step's model first, then the kind's others)
- `ports/auth/` — auth port: `Credentials` in, `Principal` out. `Authenticate` returns `(nil, nil)` for "not my credential". Adapters in `adapters/auth/` (API key, HS256 JWT with `token_version` revocation) depend only on repository ports, so they work with any database adapter. `factory.NewAuthenticators` builds them; an empty `JWT_SECRET` disables JWT auth instead of accepting unsigned-key tokens
- `middleware/` — gin middleware: request metadata, internal auth, user auth (`Authenticate`: 401 on a rejected credential, 503 when the check itself fails), API key scopes (`RequireScopes`), budget scoping, header stripping
- `transport/` + `httpclient/` — protocol-agnostic client (`transport.Client`) and HTTP engine. Call URLs a user typed in only through `transport.NewUserURLClient`, which refuses private addresses unless `ALLOW_PRIVATE_LLM_URLS` is set
- `ports/` — port interfaces (`ports/transport.go` is a stub)
- `temporal/` — `RequestMetadataPropagator` for workflow/activity headers
- `logger/` — structured logging: `Setup` installs a readable console format locally (`RAILWAY_ENVIRONMENT_NAME` unset/`local`; `NO_COLOR` drops colors) and JSON elsewhere. Log through `logger.Logger(ctx)` so the source line and request context are right
- `errors/` — typed errors and error codes
- `utils/` — context helpers (`MustUserID`, `MustBudgetID`, request metadata), pagination, text cleaning
- `otelSDK/` — OpenTelemetry setup

## Re-vendoring (required after ANY change here)

`backend/go.work` builds local services against `shared` source; Docker and CI build each service with `-mod=vendor`, so local builds prove nothing about images. From `backend/`:

```bash
for d in cipher go-pennywise-api go-gmail workflows; do (cd $d && GOWORK=off go mod vendor); done
(cd cipher && GOWORK=off go build -mod=vendor ./cmd/api)
(cd go-pennywise-api && GOWORK=off go build -mod=vendor ./cmd/api)
(cd go-gmail && GOWORK=off go build -mod=vendor -o /tmp/gmail ./cmd)
(cd workflows && GOWORK=off go build -mod=vendor ./cmd/worker)
```

## Conventions

- Queries use squirrel with Postgres placeholders: `sq.StatementBuilder.PlaceholderFormat(sq.Dollar)`.
- Services never import `adapters/` or pgx. They take `repository.*` ports; `main` gets them from `factory.ConnectDB`.
- Atomicity: wrap the work in `DBTxn.Do(ctx, func(ctx) error {...})`. The transaction travels in that ctx; every repository call made with it joins, and a nested `Do` joins too. Never start goroutines with that ctx.
- Not found: check `errors.Is(err, repository.ErrNotFound)`, never `== pgx.ErrNoRows` (adapter errors are wrapped). In the adapter, use `r.Executor(ctx)` (maps no-rows) and `errNoRows` for explicit misses.
- A repository method that issues several statements wraps them in `r.InTx(ctx, ...)` (see `payeeRepo.Merge`).
- `transactions.status` may be NULL on legacy rows: always read through `scannedStatus` (`adapters/postgres/transaction.go`), which maps NULL/empty to `MANUAL`. Do not scan it into a plain string.
- Status columns with rules (`transactions.status`, `inbound_messages.status`) change only through a guarded repo method (`TransitionStatus`, `Transition`, `RecordReport`) that writes from the status the caller read and reports whether it moved. The allowed moves live on the model type (`CanMoveTo`); check them in the service first.
- Budget isolation: repos take `budgetId` explicitly; read-only agent queries rely on Postgres RLS via `app.budget_id`, not on Go code.
- Internal trust comes only from `utils.VerifiedInternalFromContext` (set by `middleware.InternalRequestAuth`). Never trust raw headers.
- Header names (`utils/context.go`): `X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, `X-Internal-Token`, `X-Internal-Service`, `X-Service-Name`, `X-Budget-ID`.
- Outbound: transport injects the canonical headers and `X-Internal-Token` from context; seed contexts with `utils.WithInternalAuthToken`.
- Service auth: each service sends its own `INTERNAL_AUTH_TOKEN`; receivers verify it against `SERVICE_TOKENS` (`name:token,...`) and take the caller's name from the matching token, not from `X-Caller-Service`. With `SERVICE_TOKENS` unset, the receiver's own `INTERNAL_AUTH_TOKEN` is the only accepted token (the old shared-token setup, callers indistinguishable). A service principal never carries a user.
- `StripInternalHeaders` removes internal headers from responses; keep it in each ingress chain.
- Queue handlers must be idempotent: delivery is at least once. Dedupe on `Message.ID` (stable across retries), not on the Redis entry ID. Return `queue.Permanent(err)` for failures a retry can't fix.
- Queue topics are work queues (one consumer group, settled entries are `XDEL`'d). For fan-out to several services, publish to one topic per consumer; don't attach a second group to a topic.

## Key files

| Purpose | Path |
|---------|------|
| Transaction repo + `scannedStatus` | `adapters/postgres/transaction.go` |
| Connect / pool | `adapters/postgres/db.go` |
| Base repo, `Executor(ctx)`, `InTx` | `adapters/postgres/repository.go` |
| `DBTxn`, tx-in-ctx, not-found mapping | `adapters/postgres/tx.go` |
| Driver selection, `Repositories` | `factory/repositories.go` |
| Context helpers, header consts, `RequestMetadata` | `utils/context.go` |
| Internal auth middleware | `middleware/internalRequestAuth.go` |
| Budget scoping middleware | `middleware/budget.go` |
| Outbound client | `transport/client.go`, `httpclient/transport.go` |
| Temporal metadata propagation | `temporal/propagator.go` |
| Queue port | `ports/queue/queue.go` |
| Redis queue adapter + scripts | `adapters/redis/queue.go`, `adapters/redis/scripts/` |
