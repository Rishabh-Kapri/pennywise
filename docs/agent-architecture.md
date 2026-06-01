# Pennywise — AI Agent Architecture

## What We're Building

Not a query engine. A **Financial Copilot** — an intelligent personal assistant that answers natural language questions, proactively surfaces insights, detects spending patterns, flags anomalies, and helps users understand their finances without being explicitly asked.

---

## Why Simple Patterns Don't Work

### Text-to-SQL Alone

Converts natural language to SQL, runs it, returns results. Covers factual queries like "how much did I spend on food this month?" but is fundamentally **stateless and reactive** — it has no concept of "problematic", no memory of user goals, and surfaces nothing unless asked.

### ReAct Alone

Designed for task completion — reason, act, observe, repeat until done. Good for single-session tool-augmented Q&A. Breaks for a financial copilot because:

- No model for proactive insight generation
- Stateless across sessions
- No concept of background scheduled analysis

### Small Local Models (8B)

Unreliable SQL generation, inconsistent tool use, fails on multi-step reasoning. Not viable today. Revisit in 6–12 months.

---

## The Right Architecture: Memory-Enriched Tool-Augmented Agent

Three layers working together:

```
┌─────────────────────────────────────────────────┐
│  Conversational Agent  (Cloud LLM)               │
│                                                  │
│  Context window contains:                        │
│  - Last 90 days transactions (structured)        │
│  - User budgets + goals                          │
│  - Pre-computed insights from Analysis Agent     │
│  - Conversation history                          │
│                                                  │
│  Tools available:                                │
│  - execute_sql()  ← for novel queries            │
│  - get_today()    ← prevent date hallucination   │
└────────────────────┬────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────┐
│  Analysis Agent  (Go cron job, runs nightly)     │
│                                                  │
│  Pure Go + SQL, no LLM:                          │
│  - Recurring pattern detection                   │
│  - Anomaly scoring per merchant                  │
│  - Budget breach warnings                        │
│  - Trend detection across months                 │
│                                                  │
│  Writes → insights table                         │
└────────────────────┬────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────┐
│  PostgreSQL                                      │
│  transactions, budgets, goals, insights          │
└─────────────────────────────────────────────────┘
```

---

## The Insights Table

The Analysis Agent's primary output. Pre-computed, typed, dismissable.

```sql
insights
--------
id, user_id, type, title, body, severity, generated_at, expires_at, dismissed_at

-- Example rows:
-- type: "pattern"  → "You spend 2.4x more on weekends"
-- type: "anomaly"  → "Electricity bill spiked 40% this month"
-- type: "budget"   → "Food at 87% of limit with 12 days left"
-- type: "trend"    → "Swiggy spend up 3 months in a row"
```

This is what makes "unexpected" and "problematic" meaningful — relative to the user's own baseline, not global averages.

---

## The Three Rules

**1. Analysis Agent is pure Go, never LLM**
Recurring detection, anomaly scoring, trend analysis — deterministic, cheap, fast. Runs nightly, writes to insights table. The LLM cannot do this reliably in real-time.

**2. LLM always gets full context, not summaries**
Real transactions + pre-computed insights in every request. This is what makes natural conversation possible. Summaries kill the reasoning quality.

**3. Tool use only for novel queries**
If the answer is in the context window, the LLM reasons directly. If fresh data is needed, `execute_sql()`. No pre-baked specialist tools for every possible calculation.

---

## What Handles What

| Question                                      | Handled By                                   |
| --------------------------------------------- | -------------------------------------------- |
| "How much on food this month?"                | LLM reasons over context directly            |
| "Why did I save less in March?"               | LLM reasons over transactions + insights     |
| "Did any bills spike?"                        | LLM surfaces pre-computed anomaly insight    |
| "Spend on travel excluding Uber"              | LLM + single execute_sql() call              |
| "You spend 2x more on weekends"               | Analysis Agent detected it, LLM narrates it  |
| "If I cut Swiggy 50%, how long to save ₹50k?" | LLM chains 2–3 tool calls (ReAct inner loop) |

---

## Conversational Agent — Inner Patterns

Three modes, chosen dynamically by the LLM:

**Direct Tool Call** — one SQL query, one answer. Covers ~60% of questions.

**ReAct loop** — chain 2–3 dependent tool calls when results inform the next query. Covers hypotheticals and multi-step reasoning.

**Direct reasoning** — no tool calls, answer from context window. Covers anything already in the last 90 days of data.

You do not pre-classify queries. The LLM decides which mode to use based on what it needs.

---

## Tools Required (Minimal Surface)

```
execute_sql(query)       ← read-only, covers ~80% of analytical questions
get_financial_snapshot() ← current balances, this month summary (fast path)
get_schema()             ← table/column docs so LLM writes correct SQL
get_today()              ← prevent date arithmetic hallucinations
```

No specialist tools for `calculate_average`, `detect_recurring`, etc. The Analysis Agent handles those pre-computations. The LLM uses SQL for everything else.

---

## Privacy

- Use Anthropic's **zero data retention API tier** — data not stored after request, not used for training
- Be transparent in UI: "Your financial data is processed with zero retention"
- Get explicit consent at onboarding (required for DPDP compliance anyway)

Raw transactions never need to be masked or summarized when using ZDR with a reputable provider. The architectural privacy approach (summaries only) sacrifices the core product value for a risk that is manageable contractually.

---

## Agent Framework Direction

Do not use LangChain/LangGraph/MCP for the core runtime. Build a small Go framework with stable internal types and provider adapters.

The framework owns these concepts:

```
ChatRequest
ChatResponse
Message
ContentBlock
ToolDefinition
ToolCall
ToolResult
ToolChoice
```

Each provider adapter converts between the framework model and the provider wire format:

```
model.ChatRequest   → Anthropic request / OpenAI request / Ollama request
provider response   → model.ChatResponse
```

The runtime should never import Anthropic/OpenAI SDK types directly.

---

## LLM Agnosticism

Use a thin Go interface:

```go
type LLM interface {
	Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error)
}
```

Implement once per provider: `AnthropicLLM`, `OpenAILLM`, `OllamaLLM`. Your agent only talks to the interface.

Provider differences are isolated inside adapters:

| Concept       | Anthropic                             | OpenAI                   |
| ------------- | ------------------------------------- | ------------------------ |
| System prompt | top-level `system` field              | `system` message         |
| Tool schema   | `input_schema`                        | `parameters`             |
| Tool request  | `tool_use` content block              | tool/function call       |
| Tool result   | user message with `tool_result` block | tool output message/item |

Use plain HTTP behind the adapter for now. SDKs are optional later for streaming or provider-specific features.

---

## Observability First

Every LLM implementation should be wrapped by an observable decorator instead of putting tracing logic into each provider adapter.

```
Agent Runtime
      ↓
ObservedLLM
      ↓
AnthropicLLM / OpenAILLM / OllamaLLM
```

The wrapper starts one span per LLM call:

```
llm.chat
```

Minimal span attributes:

```
gen_ai.system
gen_ai.request.model
gen_ai.request.max_tokens
gen_ai.request.message_count
gen_ai.request.tool_count
gen_ai.response.model
gen_ai.response.finish_reason
gen_ai.usage.input_tokens
gen_ai.usage.output_tokens
gen_ai.usage.total_tokens
gen_ai.response.tool_call_count
```

For initial development, capture prompt and completion because Langfuse reads these fields as input/output:

```
gen_ai.prompt
gen_ai.completion
```

These must be gated before production because prompts can contain financial data.

---

## Transport Boundary

The shared transport client owns header policy. Individual transports only encode headers into their protocol.

```
transport.Client
  - default headers
  - per-request headers
  - context header propagation policy
  - final merged header map

httpclient.Transport
  - URL construction
  - JSON encoding
  - HTTP header application
  - request execution
```

External clients, such as Anthropic/OpenAI, must disable internal context header propagation. Internal service clients opt in when they need `X-Correlation-ID`, `X-Budget-ID`, `X-Internal-Token`, etc.

Do not pass booleans like `needInternalHeaders` through request methods. Set propagation once when constructing the client.

---

## Agent Run Lifecycle

The UI does not directly "run the agent". The UI creates an agent run, and the backend owns execution, tool access, auth, cancellation, tracing, and persistence.

Recommended v1 flow:

```
User opens agent message box
      ↓
UI sends message to API
      ↓
API creates agent_run record
      ↓
API starts server-side goroutine/job
      ↓
runtime.Agent.Run executes bounded LLM/tool loop
      ↓
Result is persisted
      ↓
UI polls or subscribes for status/final answer
```

Initial API shape:

```
POST /api/agent/runs       → creates run, returns run_id
GET  /api/agent/runs/:id   → returns queued/running/completed/failed status
POST /api/agent/runs/:id/cancel
```

SSE/WebSocket streaming can come later:

```
GET /api/agent/runs/:id/events
```

Minimum persisted state:

```sql
agent_runs
----------
id, user_id, budget_id, conversation_id,
status, user_message, final_message, error,
trace_id, started_at, completed_at
```

Later, add step-level persistence:

```sql
agent_run_steps
---------------
id, run_id, type, payload, created_at

-- type examples:
-- run.started
-- llm.started
-- llm.completed
-- tool.started
-- tool.completed
-- run.completed
-- run.failed
```

Important execution rules:

- Do not use the HTTP request context inside a detached goroutine after returning from the handler; create a run context with timeout and seeded request metadata.
- Every run must have max duration, max turns, and max tool-call limits.
- Persist failed/cancelled states instead of only logging failures.
- Store or propagate the OTel trace ID so UI/debug pages can link the run to Langfuse traces.
- Limit concurrent active runs per user and globally to avoid unbounded goroutine growth.

For v1, simple polling is enough. The UI can submit a message, receive `run_id`, poll every second, and render the final assistant answer when the run completes.

---

## Build Sequence

**Phase 1 — Framework Core + Observability**
Define provider-neutral `ChatRequest`, `ChatResponse`, messages, content blocks, tools, tool calls, and tool results. Implement `LLM`, `AnthropicLLM`, and `ObservedLLM`. Emit Langfuse-compatible `gen_ai.prompt` and `gen_ai.completion` during development.

**Phase 2 — Tool Mapping + Runtime Loop**
Map framework tools to provider tool schemas and map provider tool calls back into framework `ToolCall`s. Build a bounded loop with max iterations, max tool calls, and clear stop conditions.

**Phase 3 — Conversational Finance Tools**
Wire up `execute_sql`, `get_schema`, `get_today`, and `get_financial_snapshot`. Enforce SQL safety: read-only, budget-scoped, single statement, allowed tables, timeout, and row limit.

**Phase 4 — Context Builder**
Pass selected budget, budgets/goals, recent insights, conversation history, and raw structured transactions within a deterministic context budget. Prefer raw structured data over summaries, but do not allow unbounded prompts.

**Phase 5 — Analysis Agent**
Build the Go cron job. Start with anomaly detection and budget breach warnings. Write to the insights table. Wire insights into the conversational agent's context.

**Phase 6 — Recurring + Trend Detection**
Add pattern detection to the Analysis Agent. These require fuzzy merchant matching and interval clustering — pure SQL gets messy, Go logic is cleaner.

**Phase 7 — Privacy + Multi-user**
ZDR tier, consent flow, per-user data isolation. Not needed while building for yourself.

# Pennywise: Agentic Financial Assistant - Master Architecture

## 1. Core Architecture (The Foundation)

Pennywise is engineered to be a multi-tenant, privacy-first personal finance system that transforms messy banking data into structured, actionable insights.

### Pillar 1: The Nervous System (Ingestion & Ledger)

- **Asynchronous Ingestion:** Gmail webhooks trigger **Temporal workers** for reliable retries (24-hour window).
- **The HTML Filter:** A strict `html2text` parser ensures the extraction engine only sees raw data.
- **The Dedupe Shield:** SHA-256 hashing prevents the "Double SMS" bug.
- **Double-Entry Bookkeeping:** Internal bank transfers create mirrored transactions ($+\text{amount}$ and $-\text{amount}$) to ensure a zero net impact on the budget.
- **The "Ghost" Matcher:** Auto-generates the destination side of a transfer to solve the "Echo Problem" of delayed notifications.
- **Soft Deletes:** `deleted: false` flag preserves vector memory and audit logs for the LLM.

### Pillar 2: The 4-Tier Agentic Brain (Routing)

1.  **Phase 1: Lexical Router:** SQL `ILIKE` matches against user-defined `merchant_rules` ($Cost: \$0$).
2.  **Phase 2: Semantic Memory:** `pgvector` with `bge-m3` embeddings finds nearest neighbors in transaction history ($Cost: \$0$).
3.  **Phase 3: LLM Fallback:** GPT-4o (or local 8B) for complex, high-distance queries.
4.  **Phase 4: Category RAG:** Local vector search retrieves the Top 5 most relevant categories to inject into the LLM prompt, preventing context bloat.

### Pillar 3: The Vector Space Optimizer (V3 Sanitizer)

A Go-based middleware that scrubs raw text before embedding to prevent "Vector Pollution":

- Strips payment gateways (Razorpay, CCavenue).
- Strips Indian banking jargon (VPA, SO, WO).
- **P2P Bypass:** Detects human names and prevents them from entering vector space (forcing Rule/LLM logic).
- **Canonical Mapping:** Maps legal entities (e.g., BUNDL TECHNOLOGIES) to consumer brands (SWIGGY).

---

## 2. The Agentic Layer (Phase 5)

The evolution from **Reactive AI** (categorizing) to **Agentic AI** (analyzing/advising).

### Agent Personas

- **The Analyst Agent:** Answers natural language queries like _"How much did I spend on food, excluding Uber?"_ using **Function Calling**.
- **The Insight Engine:** Background Temporal workflows that run asynchronously to identify "Lifestyle Creep" or weekend spending patterns.
- **The Extraction Agent:** A vision-model-based agent that parses tabular data from messy PDF bank statements.

---

## 3. Engineering Patterns & Frameworks

### Core Agentic Patterns

| Pattern              | Mechanism                                                         | Best Use Case                                  |
| :------------------- | :---------------------------------------------------------------- | :--------------------------------------------- |
| **Direct Routing**   | LLM acts as a semantic parser to trigger a specific backend tool. | Predictable Q&A (e.g., "What is my balance?"). |
| **Reflection**       | The agent critiques its own output/SQL before execution.          | High-accuracy data extraction and coding.      |
| **ReAct**            | A loop of Reasoning + Acting (Thought -> Action -> Observation).  | Exploratory tasks (Learning phase only).       |
| **Plan-and-Execute** | A Planner creates a DAG; a Worker executes steps sequentially.    | Multi-step audits or long-running reports.     |

### The "Framework" Landscape

- **Mastra (TypeScript):** Clean, explicit control flows (steps/then) without the bloat of LangChain.
- **LangGraph (Python/JS):** Deterministic state machines for complex enterprise routing.
- **Eino (Golang):** ByteDance’s high-performance graph-based framework for Go.
- **Model Context Protocol (MCP):** Useful for exposing tools to external clients, but not needed for v1 provider portability.

---

## 4. Implementation Strategy (The Go Way)

To maintain control, the system will use a **custom dispatcher** rather than a heavy framework.

1.  **Provider-neutral model:** Internal request/response/message/tool types live in the agent model package.
2.  **Provider adapters:** Anthropic/OpenAI/Ollama convert between internal types and provider wire formats.
3.  **Observed LLM wrapper:** Tracing and Langfuse input/output capture sit around the `LLM` interface, not inside each provider.
4.  **Tool Registry:** Tools expose provider-neutral JSON schemas that adapters translate to provider-specific schema fields.
5.  **Dispatcher loop:** A bounded Go loop calls the LLM, executes tool calls, appends tool results, and stops on final answer or safety limits.
6.  **Safety Net:** Static validation is mandatory for privileged tools like SQL. Reflection is optional and should be used only for complex or risky tool calls.

---

## 5. Hardware & Privacy

- **Local Setup:** Optimized for an **RTX 3060** (12GB/8GB VRAM).
- **Models:** Utilizing 4-bit or 8-bit quantized **Llama 3.1 (8B)** or **Qwen 2.5 (7B)** via **Ollama**.
- **Sovereignty:** By using **Function Calling** (LLM provides parameters) rather than **Text-to-SQL** (LLM writes raw queries), the system remains secure and interpretable.
