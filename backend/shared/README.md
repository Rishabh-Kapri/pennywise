# shared

Common Go module imported by all Pennywise Go services (`go-pennywise-api`, `cipher`, `go-gmail`, `workflows`). Instead of strict HTTP microservice boundaries, services share this repository layer for sub-millisecond DB access with centralized SQL.

## Packages

| Package | Purpose |
|---------|---------|
| `db` | All repositories (budgets, transactions, categories, payees + rules, accounts, auth, cipher predictions, embeddings, agent runs, …). Methods accept a `pgx.Tx` so callers control atomicity. |
| `model` | Shared domain models and enums (prediction sources, transaction status, …) |
| `middleware` | `RequestMetadata` (ingress normalization), `InternalRequestAuth` (verifies `X-Internal-Token`, sets `VerifiedInternal`), `BudgetIdMiddleware` |
| `transport` / `httpclient` | Protocol-agnostic client/engine split for service-to-service calls; injects canonical `X-Correlation-ID`, `X-Caller-Service`, `X-Origin-Service`, and `X-Internal-Token` from context |
| `temporal` | `RequestMetadataPropagator` — carries `correlation_id` / `origin_service` across workflow/activity boundaries |
| `logger` | Structured logging with request metadata |
| `errors` | Typed error wrappers with error codes |
| `otelSDK` | OpenTelemetry setup (see [docs/observability.md](../../docs/observability.md)) |
| `utils` | Context helpers (`MustUserID`, `MustBudgetID`), `WithTx`, text cleaning for merchant/UPI strings |

## Conventions

- Internal service trust flows **only** through `VerifiedInternal` context set by the shared middleware — never raw headers.
- Outbound contexts are seeded with `INTERNAL_AUTH_TOKEN` so the transport layer can attach `X-Internal-Token`.
- Consumers vendor this module (`make vendor` in each service; the pre-commit hook re-vendors the Go API automatically when `shared/` changes).

```bash
make test   # go test ./...
make check  # fmt + vet + test
```
