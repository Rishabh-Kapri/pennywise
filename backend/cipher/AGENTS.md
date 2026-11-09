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

- `cmd/api` — HTTP server, DI wiring, LLM provider registry. It no longer connects to Temporal; `internal/temporal` and `internal/handler/workflow.go` are kept but not wired in.
- `cmd/backfill` — one-off backfill tool (payee rules, embeddings, extraction).
- `cmd/backfill-sources` — moves `transactions.raw_bank_text` into `transaction_sources` (statement lines also take amount, direction, date and account from their `statement_imports` row) and learns `counterparty_aliases`, all in one DB transaction (`-dry-run` rolls back; a re-run re-parses every existing source and updates it only if the parsed fields changed). Uses `db.Pool` directly. Doesn't embed aliases.
- `internal/narration` — local, rule-based parser for bank text (HDFC emails, statement narrations) → lookup key `CHANNEL | MERCHANT` + matching fields. No LLM: bank text stays in-process. After changing a rule, re-run the backfill to update existing rows.
- `internal/service` — prediction pipeline (`prediction.go`), statement extraction, category descriptions, agent service.
- `internal/client` — Ollama, OpenRouter embeddings, MLP client. `jev.go` is fully commented out (no code).
- `internal/temporal` — `PredictionActivity`: `ParseEmail`, `PredictEmail` (plus legacy batch `ParseEmailData`, `Predict`).
- `internal/ports` — hexagonal port interfaces for the prediction flow. `internal/adapters/<kind>` — their implementations (`regex`, `decisions`, `noop`: `CheckTransaction`; `unimplemented`: stand-ins; `MemoryLookup` always misses, the others return `NOT_IMPLEMENTED`). `internal/factory` selects them.
- `internal/worker` — job worker: one handler per message pipeline step (`message.received`, `.extract`, `.verify`, `.lookup`, `.predict`). Handlers call ports and queue the next job. Off without `JOB_DRIVER`. `factory.MessagePipeline` picks a step's adapter per budget and job from `pipeline.Attempts`; a step with fallback on runs a chain that moves to the next model when one fails. Check, extract, verify and predict are chosen this way; lookup always runs `adapters/alias`, an exact `CHANNEL | MERCHANT` match against `counterparty_aliases`; a miss goes on to predict.
- `internal/handler` — Gin handlers; `internal/config` — env parsing; `internal/llmusage` — per-call usage collector; `internal/progress` — step reporter for heartbeats.
- `agent/runtime` — tool-calling loop (`agent.go`). `agent/tools` — tools + `query.go` (shared filters, read-only tx). `agent/memory` — observations, token budget. `agent/context` — system prompts. `agent/llm` — client, registry, `providers/` (anthropic, openai, openrouter, lumo, ollama).
- `agent/handler` — SSE/Redis stream delivery to the Go API.

## Email pipeline

- Per email, one activity each: `ParseEmail` (LLM extraction), `PredictEmail` (payee rules → pgvector semantic search → LLM fallback). Data problems return `Skipped`; infrastructure errors return errors for Temporal retry.
- **Models are the budget owner's own**, set in the web app's Settings → AI: keys in `user_llm_providers` (one per provider) and models in `user_llm_models` (kind `chat` or `decision`, one default per kind; one key can back several models). `internal/userllm.Resolver` is the one `llm.LLMResolver`: it reads the budget from ctx, builds a chat client per enabled key (that key's first chat model, the default's key first), decrypts keys with `LLM_CREDENTIALS_KEY`, and caches per budget until a model or key changes. `Resolver.Model(ctx, kind, id)` returns a model of a kind (the requested one, else the default) with its key, for adapters such as Decisions. There is **no server-key fallback**: no usable chat model returns `userllm.ErrNotConfigured` (`AI_PROVIDER_NOT_CONFIGURED`), which the email activities make non-retryable so the workflow parks the email. A requested provider the user hasn't configured (memory observer, `AGENT_TITLE_MODEL`, the chat's model picker) falls back to their default chat model.
- LLM steps (`parse:extract`, `predict:summarize`, `predict:llm_fallback`, `statement:extract`, `category:describe`) go through `chatWithFallback`, which tries the default chat model's key first, then the user's other keys; any error falls through to the next.
- User Ollama URLs and decision-model endpoints are user input: their HTTP client (`transport.NewUserURLClient`) refuses non-public addresses at dial time. `ALLOW_PRIVATE_LLM_URLS=true` turns that off for single-user self-hosting only. Lumo is not a user provider, so it is currently unreachable.
- Jev has no adapter of its own: a `typesafe` key with a decision model at `https://api.typesafe.ai/v1/systemone` runs through `agent/decisions/providers/openrouter.go`, which speaks the same request. TypeSafe keys can't run chat or embedding models.
- Embeddings use the budget owner's default `embedding` model (Settings → AI, OpenRouter or Ollama keys only). It must return 1024 dimensions. There's no fallback to other models: stored pgvector rows are only comparable to vectors from the same model.
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
| `CIPHER_LLM_CALL_TIMEOUT` | per-call timeout (default 3m) |
| `AGENT_TITLE_MODEL` | title model (`provider/model`), resolved against the user's providers |
| `AGENT_MAX_TURNS`, `AGENT_MAX_TOOL_CALLS` | run budgets (0 = built-in defaults) |
| `AGENT_TIMEZONE` | IANA zone for "today" (default process local) |
| `AGENT_DB_URL` | least-privilege read-only DB for agent SQL tools |
| `DATABASE_URL`, `REDIS_URL`, `OLLAMA_URL`, `PENNYWISE_SERVICE_URL`, `INTERNAL_AUTH_TOKEN`, `SERVICE_TOKENS` | infrastructure |

## HTTP routes (cmd/api)

- Budget-scoped (`X-Budget-ID`): `/api/email/normalize`, `/api/email/extract`, `/api/predict`, `/api/embeddings/transaction`, `/api/corrections`, `/api/categories/describe`, `/api/statements/extract`, `/api/llm/models/test` (tiny call to a chat model, or one decision question at a decision model's endpoint, with the owner's saved key; with `modelId`, records the result on that model), `/api/agent/runs` (GET/POST), `/api/agent/runs/cancel/:id` (GET).

## Key files

| Concern | Path |
|---|---|
| Wiring, routes, worker | `cmd/api/main.go` |
| Pipeline fallback chain, fuzzy category | `internal/service/prediction.go` |
| Per-user LLM resolver | `internal/userllm/` |
| Per-model clients for pipeline steps | `internal/resolver/` |
| Adapter selection and chains | `internal/factory/` |
| Activities, error classification | `internal/temporal/activities.go` |
| Config / target parsing | `internal/config/config.go` |
| Agent loop, tool errors | `agent/runtime/agent.go` |
| Tools, filters, RLS tx | `agent/tools/query.go`, `agent/tools/tool.go`, `agent/tools/getSpendingSummary.go` |
| Token budget, observations | `agent/memory/memory.go` |
| System prompts | `agent/context/prompts.go` |
| Anthropic prompt caching | `agent/llm/providers/anthropic.go` |
