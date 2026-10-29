# AGENTS.md — cipher

Go service: email classification pipeline (Temporal activities + HTTP) and the chat agent runtime.

## Commands

```bash
make run            # API server (cmd/api), default PORT 5160
make run-agent      # local agent runtime (agent/runtime)
make run-backfill BACKFILL_ARGS="-data path"
make test           # go test ./...
make check          # fmt + vet + test
make vendor         # GOWORK=off go mod vendor (re-run after shared/ changes)
```

Docker and CI build with `-mod=vendor`; verify with `GOWORK=off go build -mod=vendor ./cmd/api`.

## Layout

- `cmd/api` — HTTP server, DI wiring, LLM provider registry, Temporal worker (only when `RAILWAY_ENVIRONMENT_NAME != local`).
- `cmd/backfill` — one-off backfill tool (payee rules, embeddings, extraction).
- `internal/service` — prediction pipeline (`prediction.go`), statement extraction, category descriptions, agent service.
- `internal/client` — Ollama, OpenRouter embeddings, MLP client. `jev.go` is fully commented out (no code).
- `internal/temporal` — `PredictionActivity`: `ParseEmail`, `PredictEmail` (plus legacy batch `ParseEmailData`, `Predict`).
- `internal/ports` — hexagonal port interfaces for the prediction flow (in progress, not wired yet). `internal/adapters/<kind>` — their implementations (`regex`: `CheckTransaction`).
- `internal/handler` — Gin handlers; `internal/config` — env parsing; `internal/llmusage` — per-call usage collector; `internal/progress` — step reporter for heartbeats.
- `agent/runtime` — tool-calling loop (`agent.go`). `agent/tools` — tools + `query.go` (shared filters, read-only tx). `agent/memory` — observations, token budget. `agent/context` — system prompts. `agent/llm` — client, registry, `providers/` (anthropic, openai, openrouter, lumo, ollama).
- `agent/handler` — SSE/Redis stream delivery to the Go API.

## Email pipeline

- Per email, one activity each: `ParseEmail` (LLM extraction), `PredictEmail` (payee rules → pgvector semantic search → LLM fallback). Data problems return `Skipped`; infrastructure errors return errors for Temporal retry.
- **Chat models are the budget owner's own** (`user_llm_providers`, set in the web app's Settings → AI). `internal/userllm.Resolver` is the one `llm.LLMResolver`: it reads the budget from ctx, decrypts keys with `LLM_CREDENTIALS_KEY`, and caches clients per budget until a row changes. There is **no server-key fallback**: no enabled provider returns `userllm.ErrNotConfigured` (`AI_PROVIDER_NOT_CONFIGURED`), which the email activities make non-retryable so the workflow parks the email. A requested provider the user hasn't configured (memory observer, `AGENT_TITLE_MODEL`, the chat's model picker) falls back to their primary provider and its model.
- LLM steps (`parse:extract`, `predict:summarize`, `predict:llm_fallback`, `statement:extract`, `category:describe`) go through `chatWithFallback`, which walks the user's providers in priority order; any error falls through to the next.
- User Ollama URLs are user input: their HTTP client (`userllm/safedial.go`) refuses non-public addresses at dial time. `ALLOW_PRIVATE_LLM_URLS=true` turns that off for single-user self-hosting only. Lumo is not a user provider, so it is currently unreachable.
- Embeddings use `EMAIL_EMBEDDING_PROVIDERS` (default `ollama=bge-m3`). Every target must serve the **same** model (bge-m3, 1024 dims; OpenRouter: `baai/bge-m3`), or stored pgvector rows become incomparable.
- LLM category resolution: exact name, then trigram fuzzy (`resolveCategoryFuzzy`: score ≥ 0.45 and lead ≥ 0.10 over runner-up). If nothing resolves, `CATEGORY_LOOKUP_FAILED` is returned as non-retryable, so the workflow parks the email on `retry-predict` immediately.
- Every LLM/embedding call is bounded by `CIPHER_LLM_CALL_TIMEOUT` (default 3m). `withHeartbeats` heartbeats at activity start and at each `progress.Report` step.
- Each call is recorded in `llmusage` (provider, model, tokens, duration, `Failed`) and returned in `llmCalls` on the activity result, including failed attempts.

## Agent invariants

- **Tool failures are results, not errors.** `toolErrorResult` sets `IsError` so every `tool_use` gets a `tool_result`. Out-of-budget calls get `refusedToolResults`; budget exhaustion takes a tools-disabled final turn (`finalAnswerTurn`).
- **Strict tool args.** Decode only via `decodeToolArgs` (`DisallowUnknownFields`). Unknown keys must error, not be ignored.
- **Filters live in one place.** Add filters to `entityFilters.apply` (`agent/tools/query.go`), not to individual tools. `get_spending_summary` applies the same scope to its total and grouped queries.
- **Tags** are `transactions.tag_ids UUID[]`. Join with `tg.id = ANY(t.tag_ids)`; any-of uses `&&`.
- **Isolation is in Postgres.** Read-only tools run in `withBudgetScopedTx` (read-only tx, `set_config('app.budget_id', …)`) so RLS applies. Set `AGENT_DB_URL` to a role granted `pennywise_agent_ro`; without it the tools run on the app role and isolation is inactive (a startup warning is logged).
- **Prompt caching.** `SystemPromptStatic` must stay byte-identical across requests (cache prefix). Varying content (date, memory, budget id) goes in `SystemPromptDynamic`. Keep tool registration order stable (`cmd/api/main.go`); the Anthropic provider caches the tool array and the static system block.
- **Context budget.** `messageTokens` = 8000. `enforceTokenBudget` first shrinks old tool-result bodies, then drops assistant/tool-call groups oldest-first. Never split a `tool_use`/`tool_result` pair.
- **Observation timestamps** come from transcript times; `repairObservationTimes` clamps anything the observer invented. Do not ask the model to produce times it was not given.

## Key env vars

| Var | Purpose |
|---|---|
| `LLM_CREDENTIALS_KEY` | base64 32-byte key that opens user provider keys; same value as the API |
| `ALLOW_PRIVATE_LLM_URLS` | let user Ollama URLs reach private addresses (self-hosting) |
| `EMAIL_EMBEDDING_PROVIDERS` | ordered embedding chain (same model across targets) |
| `CIPHER_LLM_CALL_TIMEOUT` | per-call timeout (default 3m) |
| `AGENT_TITLE_MODEL` | title model (`provider/model`), resolved against the user's providers |
| `AGENT_MAX_TURNS`, `AGENT_MAX_TOOL_CALLS` | run budgets (0 = built-in defaults) |
| `AGENT_TIMEZONE` | IANA zone for "today" (default process local) |
| `AGENT_DB_URL` | least-privilege read-only DB for agent SQL tools |
| `DATABASE_URL`, `REDIS_URL`, `OLLAMA_URL`, `PENNYWISE_SERVICE_URL`, `INTERNAL_AUTH_TOKEN` | infrastructure |
| `OPENROUTER_API_KEY` | server key for embeddings only (`EMAIL_EMBEDDING_PROVIDERS`) |
| `TEMPORAL_SERVER_HOST`, `TEMPORAL_SERVER_PORT` | Temporal (worker only when not `local`) |

## HTTP routes (cmd/api)

- Budget-scoped (`X-Budget-ID`): `/api/email/normalize`, `/api/email/extract`, `/api/predict`, `/api/embeddings/transaction`, `/api/corrections`, `/api/categories/describe`, `/api/statements/extract`, `/api/llm/providers/test` (one-off call with the owner's saved provider; records the result on the row), `/api/agent/runs` (GET/POST), `/api/agent/runs/cancel/:id` (GET).
- Unscoped: `/api/workflows/:workflowId/retry-parse`, `/retry-predict`, `/api/workflows/parsed-to-transaction`.

## Key files

| Concern | Path |
|---|---|
| Wiring, routes, worker | `cmd/api/main.go` |
| Pipeline fallback chain, fuzzy category | `internal/service/prediction.go` |
| Per-user LLM resolver, SSRF-safe client | `internal/userllm/` |
| Activities, error classification | `internal/temporal/activities.go` |
| Config / target parsing | `internal/config/config.go` |
| Agent loop, tool errors | `agent/runtime/agent.go` |
| Tools, filters, RLS tx | `agent/tools/query.go`, `agent/tools/tool.go`, `agent/tools/getSpendingSummary.go` |
| Token budget, observations | `agent/memory/memory.go` |
| System prompts | `agent/context/prompts.go` |
| Anthropic prompt caching | `agent/llm/providers/anthropic.go` |
