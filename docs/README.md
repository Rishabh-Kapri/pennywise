# docs

Design and architecture documents. The living, must-stay-accurate references are the root [README](../README.md), [AGENTS.md](../AGENTS.md), and the per-service READMEs; documents here go deeper or capture design history.

## Current architecture

- [api/](api/README.md) — importable API collection (Postman v2.1; Insomnia, Postman, Bruno) covering every route of the API, cipher and gmail
- [cipher.md](cipher.md) — the four-phase transaction classification pipeline (Ollama extraction → payee rules → pgvector → LLM fallback), Temporal integration, and schema decisions
- [job-queue.md](job-queue.md) — Postgres job queue replacing Temporal: port, adapters, history, queues and routing, worker runtime, the ingest flow on top, decisions, learnings and next steps
- [transport-architecture.md](transport-architecture.md) — shared inter-service transport layer, header propagation, internal auth
- [observability.md](observability.md) — OpenTelemetry, logging, metrics
- [agent-architecture.md](agent-architecture.md) — budget agent runtime (runs, streaming, tools, memory)

## Design explorations / history

- [prediction-flow.md](prediction-flow.md) — proposed hexagonal prediction flow: isTxn → extract → verify → memory or predict, each step a port with adapters
- [credit-card-statement-analysis-plan.md](credit-card-statement-analysis-plan.md) — proposed PDF statement extraction, charges and rewards analysis, review, and transaction matching
- [transaction-sources.md](transaction-sources.md) — one transaction, many sources: lookup key, reconciliation matching, backfill, findings and open questions

- [agent-architecture-future.md](agent-architecture-future.md) — future directions for the agent runtime
- [beyond-ReAct.md](beyond-ReAct.md) — notes on agent loop patterns beyond ReAct
- [mlp-implementation-plan.md](mlp-implementation-plan.md) — historical plan for the (now deprecated) python-mlp prediction service
- [PRD.md](PRD.md) — original product requirements
