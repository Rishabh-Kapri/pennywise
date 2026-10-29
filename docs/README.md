# docs

Design and architecture documents. The living, must-stay-accurate references are the root [README](../README.md), [AGENTS.md](../AGENTS.md), and the per-service READMEs; documents here go deeper or capture design history.

## Current architecture

- [cipher.md](cipher.md) — the four-phase transaction classification pipeline (Ollama extraction → payee rules → pgvector → LLM fallback), Temporal integration, and schema decisions
- [transport-architecture.md](transport-architecture.md) — shared inter-service transport layer, header propagation, internal auth
- [observability.md](observability.md) — OpenTelemetry, logging, metrics
- [agent-architecture.md](agent-architecture.md) — budget agent runtime (runs, streaming, tools, memory)

## Design explorations / history

- [prediction-flow.md](prediction-flow.md) — proposed hexagonal prediction flow: isTxn → extract → verify → memory or predict, each step a port with adapters
- [credit-card-statement-analysis-plan.md](credit-card-statement-analysis-plan.md) — proposed PDF statement extraction, charges and rewards analysis, review, and transaction matching

- [agent-architecture-future.md](agent-architecture-future.md) — future directions for the agent runtime
- [beyond-ReAct.md](beyond-ReAct.md) — notes on agent loop patterns beyond ReAct
- [mlp-implementation-plan.md](mlp-implementation-plan.md) — historical plan for the (now deprecated) python-mlp prediction service
- [PRD.md](PRD.md) — original product requirements
