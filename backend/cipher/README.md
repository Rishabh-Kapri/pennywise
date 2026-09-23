# cipher

Cipher is Pennywise's classification and AI service ("the brain"). It turns chaotic Indian bank/UPI transaction strings into clean payee + category predictions through a four-phase cascade, and hosts the budget agent runtime.

## Classification pipeline

1. **Extraction (Ollama SLM)** — the raw email text goes to a local model (`gemma4` via Ollama, JSON mode, temperature 0) that extracts merchant, amount, and account; normalized by shared cleaning utils.
2. **Payee rules (fast path)** — extracted merchant/UPI handle is matched against `payee_rules` (`EXACT` first, then `PATTERN`/ILIKE) via the shared repository.
3. **Vector search** — pgvector similarity against `transaction_embeddings` with configurable thresholds.
4. **LLM fallback** — cloud LLM (OpenAI/Anthropic/OpenRouter) classifies with reasoning when nothing else is confident.

Every prediction is recorded in `cipher_predictions` with its source (`RULE`/`VECTOR`/`LLM`/`MANUAL`/`UNCATEGORIZED`), confidences, and any later user correction. See [docs/cipher.md](../../docs/cipher.md) for the full architecture.

## Routes

- `POST /api/predict`, `POST /api/corrections`
- `POST /api/email/normalize`, `POST /api/email/extract`
- `POST /api/embeddings/transaction`
- `POST /api/workflows/parsed-to-transaction`, `POST /api/workflows/:workflowId/retry-predict`, `POST /api/workflows/:workflowId/retry-parse` (nudge a Temporal workflow parked at the predict/parse step)
- `/api/agent/runs` — agent run execution (dispatched from `go-pennywise-api`, which owns persistence)

Agent streaming deltas are published to the Redis stream `pubsub`; the Go API rebroadcasts them to budget-scoped websocket clients.

## Run

```bash
cp .env.example .env
make run                 # API server on PORT (default 5160)
make run-agent           # local agent runtime
make run-backfill        # backfill embeddings; BACKFILL_ARGS="-data path"
make run-embed-entities  # embed budget entities; requires BUDGET_ID
make test && make check
```

Requires a reachable Ollama endpoint (`OLLAMA_URL`); it is intentionally not part of the compose stack — run `ollama serve` on the host.

## Layout

- `cmd/api`, `cmd/backfill` — entry points
- `internal/service/prediction.go` — pipeline orchestration
- `internal/temporal` — `PredictionActivity` (on `CipherActivitiesTaskQueue`)
- `agent/` — agent runtime: `runtime/` (loop + streaming), `llm/providers/` (ollama/openai/anthropic/openrouter/lumo), `tools/`, `memory/`, `context/`

## Environment

`DATABASE_URL` (shares the main Postgres), `OLLAMA_URL`, `PENNYWISE_SERVICE_URL`, `REDIS_URL`, `INTERNAL_AUTH_TOKEN`, `AGENT_PROVIDER`, `OPENAI_API_KEY`/`ANTHROPIC_API_KEY`/`OPENROUTER_API_KEY`, `TEMPORAL_SERVER_HOST`/`PORT`, `PORT`.

### Lumo provider

Set `LUMO_BASE_URL=http://localhost:3003` (a trailing `/v1` is also accepted) to register `lumo`, then set `AGENT_PROVIDER=lumo` to select it. Use a host reachable from Cipher when running in Docker. `LUMO_API_KEY` is optional; set it to the server API key if authentication is enabled. The default model is `lumo-max`.

The provider uses the Responses API with `reasoning.effort: high` and forwards `response.reasoning_text.delta` to the agent chat. To display thinking text, set `server.reasoning.surfaceThinking: true` in [lumo-tamer](https://github.com/ZeroTricks/lumo-tamer) (under `server`, not at the YAML root). Enable `server.customTools.enabled: true` for agent tools; upstream describes custom tool support as experimental. Lumo can also be selected in `EMAIL_PIPELINE_PROVIDERS` (e.g. `lumo=lumo-max`) and `AGENT_TITLE_MODEL` (`lumo/lumo-max`).
