# go-gmail

Gmail ingestion service: watches a Gmail inbox via Google Cloud Pub/Sub for bank transaction emails and starts a Temporal `EmailToTransactionWorkflow`. The service's `FetchEmailData` activity fetches raw email bodies from the Gmail API; extraction of transaction details (amount, merchant, account) happens downstream in Cipher's `ParseEmailData` activity via a local LLM (Ollama), followed by classification and transaction creation in `go-pennywise-api`.

```
Gmail Push (Pub/Sub) → pkg/pubsub → Temporal EmailToTransactionWorkflow
                                        │
                        go-gmail FetchEmailData (raw email bodies)
                                        │
                     cipher ParseEmailData (LLM extraction via Ollama)
                                        │
                     cipher PredictionActivity → go-pennywise-api
```

## Run

```bash
make run    # go run ./cmd
make build  # binary into ./bin/go-gmail
make test   # parser + client tests
make check  # fmt + vet + test
```

Listens on port `5170`. Entry point is `cmd/main.go` (HTTP server + Pub/Sub listener + Gmail watch refresh).

## Layout

- `cmd/main.go` — entry point
- `pkg/pubsub` — Pub/Sub subscription handling; starts the Temporal workflow
- `pkg/temporal` — Temporal activities (`FetchEmailData`, Gmail watch refresh) + workflow client
- `pkg/parser/email.go` — deprecated regex email parsing, replaced by Cipher's local-LLM extraction (only used by the legacy `pkg/runner` path)
- `pkg/runner` — legacy non-Temporal pipeline (parse + predict + create inline); superseded by the workflow path
- `pkg/gmail`, `pkg/auth` — Gmail API access, OAuth2 token refresh
- `authInit/` — separate Go module: Cloud Function for the one-time OAuth consent flow

## Environment

`.env`: Gmail/Pub/Sub credentials, `PENNYWISE_SERVICE_URL`, `CIPHER_SERVICE_URL`, Temporal host/port, `INTERNAL_AUTH_TOKEN`. Outbound calls to other Pennywise services carry the shared correlation/caller/origin headers plus `X-Internal-Token`.

See `AGENTS.md` in this directory for a deeper architectural walkthrough.
