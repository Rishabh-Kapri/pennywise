# AGENTS.md — cipher

Go service: email classification pipeline (Temporal activities + HTTP) and the chat agent runtime.

## Commands

```bash
make run            # API server (cmd/api), default PORT 5160
make run-backfill BACKFILL_ARGS="-data path"
make test           # go test ./...
make check          # fmt + vet + test
make vendor         # GOWORK=off go mod vendor (re-run after shared/ changes)
```

Docker and CI build with `-mod=vendor`; verify with `GOWORK=off go build -mod=vendor ./cmd/api`.

## Layout

- `cmd/api` — HTTP server, DI wiring, LLM provider registry. It no longer connects to Temporal; `internal/temporal` and `internal/handler/workflow.go` are kept but not wired in.
- `cmd/backfill` — one-off backfill tool (payee rules, embeddings, extraction). The only user of `OLLAMA_URL`: it has its own Ollama client; the server only runs users' models.
- `cmd/backfill-sources` — moves `transactions.raw_bank_text` into `transaction_sources` (statement lines also take amount, direction, date and account from their `statement_imports` row) and learns `counterparty_aliases`, all in one DB transaction (`-dry-run` rolls back; a re-run re-parses every existing source and updates it only if the parsed fields changed). It parses statement lines only: alert emails are skipped and their sources left alone. Uses `db.Pool` directly. Doesn't embed aliases.
- `internal/service` — the old email flow's prediction (`prediction.go`, prompts in `prompts.go`), statement extraction, category descriptions, agent service. `MatchKeysFor` builds payee-rule and embedding keys for both lookups and what's stored (also `cmd/backfill`).
- `internal/client` — the user embedder (Ollama or OpenRouter, 1024 dims). Callers bound each call.
- `internal/resolver` — builds the client for one saved model from its key (chat, embedding, decisions) and caches it per key. The only place that decrypts a key or picks a provider's client.
- `internal/categorymatch` — the one way a model's category name resolves to the budget's: exact, then trigram (score ≥ 0.45, lead ≥ 0.10 over the runner-up).
- `internal/emailtext` — email HTML to text. `Plain` feeds models, dedupe hashes and lookup keys (changing it changes stored keys); `Display` keeps line breaks for people.
- `internal/temporal` — `PredictionActivity`: `ParseEmail`, `PredictEmail` (plus legacy batch `ParseEmailData`, `Predict`).
- `internal/ports` — port interfaces for the message pipeline, one per step. `internal/adapters/<kind>` — their implementations: `regex` (check), `decisions` (check, verify), `llm` (check, extract, predict; its prompts are in `prompts.go`), `alias` (lookup).
- `internal/factory` — `MessagePipeline` picks a step's adapter per budget and job from `pipeline.Attempts`. Each step's adapters are one table in `selector.go`, keyed by the adapter ID in `shared/pipeline/pipelines/message.json`; startup fails if the file names a step or adapter the table lacks (create is go-pennywise-api's). A step with fallback on runs a chain that moves to the next model when one fails.
- `internal/worker` — job worker: one handler per message pipeline step (`message.received`, `.extract`, `.verify`, `.lookup`, `.predict`). Handlers get the step's adapter from `factory.MessagePipeline`, call it and queue the next job. Off without `JOB_DRIVER`. Lookup runs `adapters/alias`, an exact `CHANNEL | MERCHANT` match against `counterparty_aliases`; a miss goes on to predict. Nothing in the pipeline parses email with rules: extract reads the details, and verify (one decisions call) checks them and picks channel, VPA, ref and a missing account. Decision models can only choose, so verify offers candidates found by shape in the text (`adapters/decisions/candidates.go`, no bank wording). A UPI alias is keyed on its VPA, then tried by name for older aliases (`narration.LookupMerchant`). A lookup hit's confidence is 1 for a MANUAL alias and 0.8 for a learned one; an alias whose category is Uncategorized is a miss. Extract keeps a message's clock as UTC; `messageSource` reads it in `narration.BankZone`. Keep the source row's `merchant` that same key.
- Review: details that fail verify always park as `needs_review` through `park()`; low-confidence check and verify answers do only with `MESSAGE_REVIEW_PARK=true`, and are otherwise logged as "would park for review". An adapter with no score (regex) leaves `Confidence` nil, which never parks. `reviewThreshold` is in `worker/handlers.go`. The report carries `review.resume`, the next job with a fresh `RunID`; without that the rest of the run shares idempotency keys with the parked one and its final report is dropped. Predict never parks: a low-confidence payee is created unapproved and handled in Prediction review.
- `internal/handler` — Gin handlers; `internal/config` — env parsing (only `main`s call `config.Load`; pass values in); `internal/llmusage` — per-call usage collector; `internal/progress` — step reporter for heartbeats.
- `agent/runtime` — tool-calling loop (`agent.go`). `PublishChatEvent` is the one writer of chat events (to `model.AgentEventsRedisStream`); run metadata stores tool calls as `[]RunToolCall`. `agent/tools` — tools + `query.go` (shared filters, read-only tx). `agent/memory` — observations, token budget. `agent/context` — system prompts. `agent/llm` — client, registry, `providers/` (anthropic, openai, openrouter, lumo, ollama). `agent/decisions` — the Decisions client interface, `Probe`, and `providers/openrouter.go` (the one Decisions HTTP client).
- `agent/handler` — SSE/Redis stream delivery to the Go API.

## Email pipeline

- Per email, one activity each: `ParseEmail` (LLM extraction), `PredictEmail` (payee rules → pgvector semantic search → LLM fallback). Data problems return `Skipped`; infrastructure errors return errors for Temporal retry.
- **Models are the budget owner's own**, set in the web app's Settings → AI: keys in `user_llm_providers` (one per provider) and models in `user_llm_models` (kind `chat` or `decision`, one default per kind; one key can back several models). `internal/userllm.Resolver` is the one `llm.LLMResolver`: it reads the budget from ctx, builds a chat client per enabled key (that key's first chat model, the default's key first) through `internal/resolver`, which decrypts keys with `LLM_CREDENTIALS_KEY`, and caches per budget until a model or key changes. There is **no server-key fallback**: no usable chat model returns `userllm.ErrNotConfigured` (`AI_PROVIDER_NOT_CONFIGURED`), which the email activities make non-retryable so the workflow parks the email. A requested provider the user hasn't configured (memory observer, `AGENT_TITLE_MODEL`, the chat's model picker) falls back to their default chat model.
- LLM steps (`parse:extract`, `predict:summarize`, `predict:llm_fallback`, `statement:extract`, `category:describe`) go through `chatWithFallback`, which tries the default chat model's key first, then the user's other keys; any error falls through to the next.
- User Ollama URLs and decision-model endpoints are user input: their HTTP client (`transport.NewUserURLClient`) refuses non-public addresses at dial time. `ALLOW_PRIVATE_LLM_URLS=true` turns that off for single-user self-hosting only. Lumo is not a user provider, so it is currently unreachable.
- Jev has no adapter of its own: a `typesafe` key with a decision model at `https://api.typesafe.ai/v1/systemone` runs through `agent/decisions/providers/openrouter.go`, which speaks the same request. TypeSafe keys can't run chat or embedding models. Settings → AI's model test probes a decision model through that same client (`decisions.Probe`).
- Embeddings use the budget owner's default `embedding` model (Settings → AI, OpenRouter or Ollama keys only). It must return 1024 dimensions. There's no fallback to other models: stored pgvector rows are only comparable to vectors from the same model.
- LLM category resolution goes through `categorymatch.Match`. In the old flow, no match returns `CATEGORY_LOOKUP_FAILED` as non-retryable, so the workflow parks the email on `retry-predict` immediately; in the job pipeline it ends the message as `no_prediction`.
- Every LLM/embedding call is bounded by `CIPHER_LLM_CALL_TIMEOUT` (default 3m): `main` passes it to the prediction service and the pipeline factory, which bound their calls. Decision calls use the adapter's own 15s. `withHeartbeats` heartbeats at activity start and at each `progress.Report` step.
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
| `DATABASE_URL`, `REDIS_URL`, `PENNYWISE_SERVICE_URL`, `INTERNAL_AUTH_TOKEN`, `SERVICE_TOKENS` | infrastructure |
| `OLLAMA_URL` | `cmd/backfill`'s own Ollama; the server never reads it |

## HTTP routes (cmd/api)

- Budget-scoped (`X-Budget-ID`): `/api/email/extract` and `/api/predict` (go-gmail only), `/api/embeddings/transaction`, `/api/categories/describe`, `/api/statements/extract`, `/api/llm/models/test` (tiny call to a chat model, or one decision question at a decision model's endpoint, with the owner's saved key; with `modelId`, records the result on that model), `POST /api/agent/runs`. Cipher has no get or cancel route for runs: the API's `CancelRun` calls `POST /api/agent/runs/:id/cancel`, which 404s.

## Key files

| Concern | Path |
|---|---|
| Wiring, routes, worker | `cmd/api/main.go` |
| Old flow: provider fallback chain, match keys | `internal/service/prediction.go` |
| Category name matching | `internal/categorymatch/` |
| Per-user LLM resolver | `internal/userllm/` |
| Per-model clients (chat, embedding, decisions) | `internal/resolver/` |
| Adapter tables, selection and chains | `internal/factory/` |
| Activities, error classification | `internal/temporal/activities.go` |
| Config / target parsing | `internal/config/config.go` |
| Agent loop, tool errors | `agent/runtime/agent.go` |
| Tools, filters, RLS tx | `agent/tools/query.go`, `agent/tools/tool.go`, `agent/tools/getSpendingSummary.go` |
| Token budget, observations | `agent/memory/memory.go` |
| System prompts | `agent/context/prompts.go` |
| Anthropic prompt caching | `agent/llm/providers/anthropic.go` |
