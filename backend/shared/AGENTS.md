# AGENTS.md — backend/shared

Shared Go module (`github.com/Rishabh-Kapri/pennywise/backend/shared`): repository ports and their Postgres adapter, models, HTTP transport, middleware, logging, Temporal propagator, utils. Imported by cipher, go-pennywise-api, go-gmail, workflows.

## Test

```bash
cd backend/shared && go test ./...   # or: make check (fmt + vet + test)
```

Postgres integration tests (`adapters/postgres/*_integration_test.go`) skip unless `PENNYWISE_TEST_DATABASE_URL` names a role that can `CREATE DATABASE`; each run creates a throwaway database, applies `migrations/sql`, and drops it. Use the `pgvector/pgvector:pg17` image (see `integration_test.go`).

## Layout

- `ports/repository/` — repository ports: one interface per aggregate, plus `DBTxn`, `BulkInserter`, `ErrNotFound`. No driver types.
- `adapters/postgres/` — the Postgres adapter for those ports (pgx + squirrel), one file per aggregate
- `factory/` — `ConnectDB(Config)`: picks the adapter from `DATABASE_DRIVER` (default `postgres`) and returns the ports. The only package outside `adapters/` that imports an adapter
- `migrations/sql/` — embedded goose migrations for Postgres; how to write one: `migrations/sql/README.md`. Run by `go-pennywise-api/cmd/migrations`. Other databases get a sibling (`migrations/mongodb/`)
- `model/` — domain structs and enums
- `middleware/` — gin middleware: request metadata, internal auth, budget scoping, header stripping
- `transport/` + `httpclient/` — protocol-agnostic client (`transport.Client`) and HTTP engine
- `ports/` — port interfaces (`ports/transport.go` is a stub)
- `temporal/` — `RequestMetadataPropagator` for workflow/activity headers
- `logger/` — structured logging (pretty handler for dev)
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
- Budget isolation: repos take `budgetId` explicitly; read-only agent queries rely on Postgres RLS via `app.budget_id`, not on Go code.
- Internal trust comes only from `utils.VerifiedInternalFromContext` (set by `middleware.InternalRequestAuth`). Never trust raw headers.
- Header names (`utils/context.go`): `X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, `X-Internal-Token`, `X-Internal-Service`, `X-Service-Name`, `X-Budget-ID`.
- Outbound: transport injects the canonical headers and `X-Internal-Token` from context; seed contexts with `utils.WithInternalAuthToken`.
- Gotcha: `verifyInternalRequest` falls back to the legacy `X-Internal-Service: true` header when the expected token is empty. Services must always set `INTERNAL_AUTH_TOKEN`.
- `StripInternalHeaders` removes internal headers from responses; keep it in each ingress chain.

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
