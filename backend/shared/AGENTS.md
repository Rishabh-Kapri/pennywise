# AGENTS.md — backend/shared

Shared Go module (`github.com/Rishabh-Kapri/pennywise/backend/shared`): repositories, models, HTTP transport, middleware, logging, Temporal propagator, utils. Imported by cipher, go-pennywise-api, go-gmail, workflows.

## Test

```bash
cd backend/shared && go test ./...   # or: make check (fmt + vet + test)
```

## Layout

- `db/` — repositories (pgx + squirrel), one file per aggregate
- `sql/` — embedded goose migrations (`sql/migrations`); how to write one: `sql/README.md`. Run by `go-pennywise-api/cmd/migrations`
- `model/` — domain structs and enums
- `middleware/` — gin middleware: request metadata, internal auth, budget scoping, header stripping
- `transport/` + `httpclient/` — protocol-agnostic client (`transport.Client`) and HTTP engine
- `ports/` — transport port interface
- `temporal/` — `RequestMetadataPropagator` for workflow/activity headers
- `logger/` — structured logging (pretty handler for dev)
- `errors/` — typed errors and error codes
- `utils/` — context helpers (`MustUserID`, `MustBudgetID`, request metadata), `WithTx`, pagination, text cleaning
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
- Methods needing atomicity take `pgx.Tx` and carry a `Tx` suffix (e.g. `GetByIdTx`). Use `utils.WithTx` to open one.
- `transactions.status` may be NULL on legacy rows: always read through `scannedStatus` (`db/transaction.go`), which maps NULL/empty to `MANUAL`. Do not scan it into a plain string.
- Budget isolation: repos take `budgetId` explicitly; read-only agent queries rely on Postgres RLS via `app.budget_id`, not on Go code.
- Internal trust comes only from `utils.VerifiedInternalFromContext` (set by `middleware.InternalRequestAuth`). Never trust raw headers.
- Header names (`utils/context.go`): `X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, `X-Internal-Token`, `X-Internal-Service`, `X-Service-Name`, `X-Budget-ID`.
- Outbound: transport injects the canonical headers and `X-Internal-Token` from context; seed contexts with `utils.WithInternalAuthToken`.
- Gotcha: `verifyInternalRequest` falls back to the legacy `X-Internal-Service: true` header when the expected token is empty. Services must always set `INTERNAL_AUTH_TOKEN`.
- `StripInternalHeaders` removes internal headers from responses; keep it in each ingress chain.

## Key files

| Purpose | Path |
|---------|------|
| Transaction repo + `scannedStatus` | `db/transaction.go` |
| Connect / pool | `db/db.go` |
| Base repo, `Executor(tx)` | `db/repository.go` |
| Context helpers, header consts, `RequestMetadata` | `utils/context.go` |
| `WithTx` | `utils/pgx.go` |
| Internal auth middleware | `middleware/internalRequestAuth.go` |
| Budget scoping middleware | `middleware/budget.go` |
| Outbound client | `transport/client.go`, `httpclient/transport.go` |
| Temporal metadata propagation | `temporal/propagator.go` |
