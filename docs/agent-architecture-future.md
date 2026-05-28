# Pennywise — Agent Architecture: Future Vision

This document describes the planned evolution of the Cipher agent beyond the current ReAct loop. It covers the full target architecture: intent routing, micro-planning, parallel DAG execution, local summarisation, durable orchestration via Temporal, and MCP exposure. Nothing here is implemented yet unless explicitly noted.

---

## Current State (What Is Built)

The live agent path is a single-threaded ReAct loop:

```
User message
     ↓
Intent Classification (cloud LLM — Anthropic/OpenAI, Haiku/GPT-4o-mini)
     ↓
Payee term resolution (local pgvector via bge-m3)
     ↓
System context injection (category + account names, budget_id)
     ↓
ReAct loop (cloud LLM + tools)
  ├── get_schema
  ├── execute_sql
  └── get_today
     ↓
Raw SQL result returned to cloud LLM for narration
     ↓
Response streamed to client via Redis pubsub → WebSocket
```

**What works well:**
- Intent classification is cheap and fast (Haiku/GPT-4o-mini, no raw data sent).
- Vector payee resolution keeps bank strings off the wire.
- ReAct loop handles multi-step dependent queries naturally.
- Provider-neutral model + adapter pattern gives LLM agnosticism.

**What breaks down:**
- Raw DB rows (amounts, balances) flow back to the cloud LLM for narration — violates the privacy constraint.
- All tool calls are sequential; unrelated sub-queries cannot run in parallel.
- No per-step model configuration — the same LLM does reasoning, SQL generation, and narration.
- No durable execution — a server restart mid-run drops the agent state entirely.
- No way for external clients or tools to reach the agent runtime (no MCP surface).

---

## Target Architecture Overview

```
User message
     ↓
┌─────────────────────────────────┐
│  Intent Router                  │  cloud LLM (cheap, fast)
│  - intent type                  │  sends: query + group names only
│  - date range                   │  never sees: amounts, IDs, balances
│  - payee terms                  │
│  - category groups              │
└────────────┬────────────────────┘
             │
             ├─── DateRange unresolved ──→ CLARIFY GATE
             │                             return question to user
             │                             no DB work done
             │ DateRange resolved
             ↓
┌─────────────────────────────────┐
│  Local Entity Resolution        │  pgvector (bge-m3, fully local)
│  - ResolvePayee()               │  maps raw bank strings → UUIDs
│  - category vector search       │  confidence threshold gates
└────────────┬────────────────────┘
             │ resolved UUIDs
             ↓
┌─────────────────────────────────┐
│  Observational Memory Check     │  pgvector agent_memory table
│  key: (budget_id, date_range)   │  long-lived, not TTL-based
│  HIT  → load cached context     │
│  MISS → run scoped DB query     │
│         write result to memory  │
└────────────┬────────────────────┘
             │ scoped context:
             │ category names + payee names
             │ + summed spend per category
             │ (for the resolved date window only)
             ↓
┌─────────────────────────────────┐
│  Micro-Planner                  │  cloud LLM (structured output)
│  - receives: intent, resolved   │  sends: scoped context only
│    entity names, scoped context │  never sees: raw rows or balances
│  - emits: ExecutionPlan (DAG)   │
│    OR: answers directly from    │
│    spend totals in context      │
└────────────┬────────────────────┘
             │ ExecutionPlan (or direct answer)
             ↓
┌─────────────────────────────────┐
│  Parallel DAG Executor          │  pure Go, no LLM
│  - skipped if planner answered  │
│    directly from context        │
│  - topological sort             │
│  - runs independent steps in    │
│    parallel goroutines          │
│  - passes outputs of step N     │
│    as inputs to dependents      │
│  - enforces max concurrency,    │
│    per-step timeout, row cap    │
└────────────┬────────────────────┘
             │ StepResults (raw DB rows)
             ↓
┌─────────────────────────────────┐
│  Local Summariser               │  Ollama (fully local, e.g. gemma3)
│  - receives raw rows            │  raw financial data never leaves box
│  - narrates into plain English  │
│  - strips thinking blocks       │
└────────────┬────────────────────┘
             │ natural language answer
             ↓
Redis pubsub stream → WebSocket → client
```

---

## Component Detail

### 1. Intent Router

**What it does:** Translates the user query into a structured `IntentResult`. This is already implemented.

**Privacy contract:** Only the query text, current date, and category group names (user-created labels, not PII) are sent to the cloud. No IDs, amounts, balances, or transaction strings leave the machine at this step.

**Output:**
```go
type IntentResult struct {
    Intent         string    // e.g. "spending_query", "balance_check", "what_if"
    DateRange      DateRange // start/end dates resolved from relative language; zero value = not resolved
    CategoryGroups []string  // matched group names from the budget
    PayeeTerms     []string  // raw payee strings from the query
    Confidence     float32
}
```

**Model:** Dedicated cheap cloud LLM (currently `claude-haiku-4-5` / `gpt-5.4`). Never the main reasoning model.

**Clarify gate — missing date range:**

If `IntentResult.DateRange` is unresolved (the query is a blanket statement like "what is my food spending?"), the agent stops immediately and returns a clarifying question to the user before doing any DB work:

```
"What time period are you asking about? (e.g. last month, April, this year)"
```

No context is loaded, no SQL is run, no cloud LLM call beyond the cheap router. The conversation resumes from this point once the user supplies a time frame, which is then passed back through the router to resolve a concrete `DateRange`.

This is an explicit interrupt, not an LLM-guessed default. Silently assuming "current month" produces misleading answers for queries that span multiple months or are genuinely all-time.

---

### 2. Local Entity Resolution

**What it does:** Maps raw payee terms from the query to internal UUIDs using pgvector similarity search. Already implemented.

**Privacy contract:** Fully local. No payee strings, names, or IDs reach the cloud.

**Thresholds:**
- `score >= 0.60` → accept, add UUID to resolved set.
- `score < 0.60` → collect as ambiguous candidate, surface clarification to user before continuing.

**Planned extension:** Category entity resolution using the same embedding table, scoped by `entity_type = 'category'`.

---

### 2a. Scoped Context Load (replaces full category/payee dump)

**The problem with the current approach:**

The current `ContextBuilder.GetBudgetContext()` loads *all* categories and *all* accounts for the budget and injects them into every system prompt. This has two costs:

- Token cost: a large budget with 40+ categories and 10 accounts sends hundreds of tokens of context that are irrelevant to the question.
- Signal-to-noise: the LLM reasons over the full set when the answer is in a small subset.

**The new approach — zero context by default, scoped on demand:**

The agent starts with no category or payee context. After the intent router resolves a `DateRange`, a targeted DB query loads only what is relevant to that window:

```sql
SELECT
    c.name                    AS category_name,
    SUM(t.amount)             AS total_spend
FROM transactions t
JOIN categories c ON t.category_id = c.id
WHERE t.budget_id = $1
  AND t.date >= $2
  AND t.date <= $3
  AND t.deleted = false
GROUP BY c.name
ORDER BY total_spend ASC
```

A similar query retrieves distinct payee names that appear in the window. The result — category names + payee names + summed spend per category — is injected into the system context instead of the full budget dump.

**Why summed spend is included:**

With spend totals already in context, the LLM can answer the majority of spending questions ("how much did I spend on food in April?") directly from context without calling `execute_sql` at all. `execute_sql` is reserved for novel queries the context doesn't cover — breakdowns by payee within a category, comparisons across windows, trend queries, etc.

**Privacy contract:** Only category names, payee display names, and spend totals reach the cloud LLM. No account numbers, transaction IDs, or raw transaction strings are sent. Spend totals are user-created aggregates — they are not raw bank data.

**Blanket queries (no date range):**

If the date range is not resolved, this step is skipped entirely. The clarify gate in step 1 returns a question to the user first. No DB queries run until a concrete window is available.

---

### 2b. Scoped Context Memory Cache (budget/date-range)

**What it is:**

The result of the scoped context load — the set of `(category_name, payee_name, spend_total)` tuples for a given budget and time window — is stored as a long-lived memory entry in pgvector (reusing the `entity_embeddings` table or a dedicated `agent_memory` table).

This is separate from conversation observational memory. This cache remembers budget context for a resolved time window; conversation observational memory compresses long chat history so the agent does not replay every past message and tool result.

This is not a TTL cache. It is durable observational memory. Categories and payees don't change often; the memory stays valid until the user explicitly tells the agent the data has changed ("I added a new category, use fresh data").

**How it works:**

Before running the scoped context DB query, the agent checks the memory store:

```
memory key: (budget_id, date_range_start, date_range_end)
memory value: {categories: [{name, spend_total}], payees: [name, ...]}
```

If a memory entry exists for this `(budget_id, date_range)`, it is used directly and the DB query is skipped entirely. If not, the DB query runs and the result is written to memory before being injected into context.

**Why pgvector:**

pgvector is already in the stack for entity embeddings. A dedicated `agent_memory` table stores the serialised context blob alongside a vector embedding of the query that produced it. This enables fuzzy retrieval — a follow-up question with a slightly different date phrasing ("April" vs "last month" for the same resolved window) can still hit the same memory entry via semantic similarity rather than requiring an exact key match.

**Memory invalidation:**

No automatic TTL. The user can trigger invalidation explicitly ("refresh your memory" or "use updated categories"). A manual invalidation endpoint can also flush memory for a budget when transactions are bulk-imported or categories are restructured.

**What this replaces in the pipeline:**

Once memory is warm, the flow for a returning user asking about a previously-seen time window is:

```
Intent Router → date range resolved
     ↓
Memory check → HIT → inject cached context
     ↓
Micro-Planner (gets rich scoped context, not full budget dump)
     ↓
DAG Executor → may skip execute_sql entirely if spend totals answer the question
     ↓
Local Summariser
```

The scoped DB query and the `execute_sql` tool call are both skipped on a memory hit for common questions.

---

### 2c. Conversation Observational Memory (long chat history)

**Problem it solves:**

Today the Go API sends all previous `conversation_messages` and all previous run metadata to Cipher. Cipher then reconstructs past assistant messages, tool calls, and tool results in `agentRunToChatRequest`. This preserves history, but token usage grows linearly with every turn. Tool results are the worst offender because old raw result payloads are replayed even when only their conclusion matters.

Conversation observational memory keeps the active prompt bounded:

```
system prompt
working memory
active observations / reflection
recent raw messages and recent tool exchanges
current user message
```

`working_memory` remains explicit memory written by tools when the user states lasting preferences or mappings. Conversation observational memory is automatic history compression for continuity.

**Mastra findings:**

Mastra's Observational Memory is not only a post-run summarizer. It hooks into the main agentic lifecycle with input and output processors:

- `processInputStep` runs before each actor model step. It loads memory context, activates buffered observations when needed, prunes observed raw messages, and injects observation system messages.
- `processOutputResult` runs after the actor model output. It finalizes the turn and triggers background buffering/reflection work.
- Observer and Reflector are separate runners from the actor agent. They perform one-shot LLM calls, not full tool-enabled agent loops.
- Async buffering starts Observer calls before the hard history threshold. `bufferTokens` defines the interval, `messageTokens` defines when buffered chunks are activated, and `blockAfter` is a safety fallback that forces synchronous observation if buffering cannot keep up.
- Reflection follows the same shape: start reflection in the background as observations grow, then activate the reflection when observation context crosses its threshold.
- Mastra also tracks `currentTask` and `suggestedResponse` separately from the observations so the actor can resume smoothly after old raw messages are pruned.

Reference: https://mastra.ai/docs/memory/observational-memory#async-buffering

**Pennywise shape:**

`backend/cipher/agent/memory/memory.go` should own the conversation memory lifecycle:

```go
type Memory interface {
    GetWorkingMemory(ctx context.Context, budgetID uuid.UUID) string
    PrepareContext(ctx context.Context, req MemoryPrepareRequest) (*MemoryContext, error)
    Start(ctx context.Context)
}
```

`PrepareContext` is the input-side hook. It runs before the real chat agent calls the actor model. It should:

1. Load the conversation memory record.
2. Activate buffered observations if thresholds, idle time, or model/provider changes require it.
3. Load active observations/reflection plus `currentTask` and `suggestedResponse`.
4. Load only raw messages after the activated boundary, then keep a small recent window.
5. Return bounded messages and memory context for the actor prompt.

`Start` is the output-side worker. It listens for persisted-run events and performs async buffering/reflection. It should not listen to the UI stream or raw text deltas; memory should run after the assistant message content and run metadata have been persisted.

**Runtime hook point:**

`backend/cipher/agent/runtime/agent.go` is the right boundary for both sides of the hook because `Agent.Run` already owns tool injection, system prompt injection, the bounded tool loop, final assistant output, normalized message parts, and run metadata.

Memory should be enabled per run:

```go
agent.WithMemoryEnabled(true)  // real chat runs
agent.WithMemoryEnabled(false) // title, observer, reflector, classification
```

Before the LLM step, `Agent.Run` calls `memory.PrepareContext` when memory is enabled. After the run, the existing deferred persistence block patches run metadata and conversation message content. Only after both patches succeed should it publish a persisted-run event:

```go
type AgentRunPersistedEvent struct {
    ConversationID uuid.UUID
    RunID          uuid.UUID
    MessageID      uuid.UUID
    BudgetID       uuid.UUID
    UserID         uuid.UUID
    Model          string // provider/model
}
```

The memory worker consumes this event and runs:

```
load OM record
load unobserved messages and tool metadata
count pending message tokens

if pending tokens crossed next buffer boundary:
    run Observer in background
    store buffered observation chunk

if pending tokens >= messageTokens:
    activate buffered chunks
    advance activatedThroughSequence
    stop replaying covered raw messages

if active observation tokens crossed reflection buffer threshold:
    run Reflector in background

if active observation tokens >= observationTokens:
    activate reflection / create a new generation
```

The event bus can be in-process for v1. Missing an event is acceptable because memory is catch-up based: the next persisted run can inspect the conversation sequence and process any unobserved range.

**One-shot LLM runner:**

Title generation, Observer, and Reflector should not use the full `Agent.Run` loop. They should use a smaller one-shot runner that resolves the provider/model and calls `Chat` once:

```go
type Runner interface {
    Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error)
}
```

Use the full `runtime.Agent` only for real chat runs that can execute tools, stream to the UI, patch run/message metadata, and publish memory events. Use the one-shot runner for internal text generation:

- title generation
- observer extraction
- reflector compaction
- future classification/summarization calls

This prevents internal calls from accidentally executing tools, patching user-visible messages, publishing chat stream events, or recursively triggering memory.

**Suggested defaults:**

```
messageTokens:      30_000
bufferTokens:       0.2 * messageTokens  // about 6,000
blockAfter:         1.2 * messageTokens  // about 36,000
observationTokens:  40_000
reflectionBuffer:   0.5 * observationTokens
recentRawMessages:  8 conversation messages
```

Observer prompts may see raw historical tool outputs because those outputs contain facts the assistant may not have repeated. The active actor prompt should not replay old raw tool outputs after they are observed. It should receive active observations/reflection plus only recent raw tool exchanges needed for provider sequencing.

**Storage model:**

Use a dedicated conversation memory table instead of placing this in `conversations.metadata`. The memory record needs cursors, buffering flags, chunk status, token counts, reflection generations, and covered sequence ranges. A JSON metadata blob would make those updates harder to reason about.

At minimum, store:

- conversation id, budget id, user id
- active observations
- buffered observation chunks
- active reflection / generation count
- `activatedThroughSequence`
- `lastBufferedSequence` / `lastBufferedAt`
- pending message token count
- observation token count
- current task, suggested response, optional generated thread title

**Failure behavior:**

Memory is non-critical for v1. If `PrepareContext` fails, log and fall back to the recent raw messages available on the request. If buffering/reflection fails, log and leave the cursor unchanged so a later persisted-run event can retry.

---

### 3. Micro-Planner

**What it does:** Takes the `IntentResult` + resolved entity names + a compact schema summary and produces an `ExecutionPlan`: an ordered list of named steps, each with a tool name, arguments template, and declared dependencies on other steps.

**Why a separate planner instead of letting the ReAct loop plan?**

The ReAct loop plans and executes sequentially — each tool call waits for the previous one to complete before the LLM decides the next step. For queries that require multiple independent sub-queries (e.g., "compare food spend in March vs April and also show my savings rate"), the ReAct loop runs them one after another. The micro-planner emits the full plan upfront so the executor can run independent steps in parallel.

**Privacy contract:** The planner receives entity *names* (category names, payee display names) and the schema — not amounts, balances, or raw transaction data. It emits a plan (SQL queries + tool args) but never sees execution results.

**Output:**
```go
type ExecutionStep struct {
    ID           string            // unique step name, e.g. "food_april"
    Tool         string            // e.g. "execute_sql"
    Args         map[string]any    // tool arguments, may reference prior step outputs via "$steps.food_march.total"
    DependsOn    []string          // step IDs that must complete before this step runs
}

type ExecutionPlan struct {
    Steps []ExecutionStep
}
```

**Model:** Same cloud LLM as the main agent (Claude Sonnet / GPT-4o), structured JSON output mode.

---

### 4. Parallel DAG Executor

**What it does:** Pure Go. No LLM calls. Executes the plan:

1. Topological sort of steps by `DependsOn`.
2. Steps with no unmet dependencies are dispatched as goroutines immediately.
3. When a step completes, its output is stored in a shared result map.
4. Waiting steps are unblocked as their dependencies resolve.
5. Template references like `$steps.food_march.total` in step args are resolved from the result map before execution.

**Limits enforced by the executor:**
- Max parallel goroutines per plan (e.g., 5).
- Per-step timeout (e.g., 5s — already enforced in `execute_sql`).
- Global plan timeout (e.g., 30s).
- Row cap per SQL result (500 rows) before truncation.

**Error handling:**
- A failed step marks dependent steps as `skipped`.
- Non-dependent steps continue executing.
- The summariser receives partial results with step status annotations.

---

### 5. Local Summariser

**What it does:** Receives the raw step results (DB rows, tool outputs) and narrates them into a plain English response using a local Ollama model.

**Privacy contract:** Raw financial data (amounts, balances, transaction rows) never leaves the machine. The cloud LLM only ever saw the plan — not the data the plan retrieved.

**Model:** Local Ollama (e.g., `gemma3`, `qwen3:8b`). Thinking blocks must be stripped before the response is returned to the client (last non-empty line of model output is the answer).

**Intercept point in current code:** After the final `execute_sql` tool result in `Run()`, before the cloud LLM narration step. The local summariser replaces the cloud narration call.

**Prompt shape:**
```
You are a personal finance assistant. Below are query results for the user's question.
Narrate the results in plain English. Be concise. Do not mention SQL, IDs, or internal fields.

User question: {query}

Results:
{step_results_as_json}
```

---

### 6. Durable Orchestration via Temporal

**Why:** The current agent runs in a goroutine tied to the HTTP request context. If the server restarts mid-run, the agent state is lost. For long-running plans (multi-step, external tool calls, retries), durability is required.

**Target structure:**

```
AgentWorkflow (Temporal workflow)
  ├── IntentActivity       → calls cloud LLM for intent classification
  ├── EntityResolutionActivity → local pgvector
  ├── PlanActivity         → calls cloud LLM for execution plan
  ├── ExecuteStepActivity  → one activity per DAG step (parallelised via goroutines inside the activity, or via child workflows)
  └── SummariseActivity    → local Ollama narration
```

Each activity is individually retryable. Temporal handles timeouts, retry backoff, and state persistence. The agent run `status` in the `agent_runs` table is updated by the workflow as it progresses.

**This is the correct long-term execution model.** The Temporal worker in cipher (`CipherActivitiesTaskQueue`) already exists for the prediction pipeline — the agent workflow would register on the same queue or a dedicated `CipherAgentTaskQueue`.

**Not needed for v1.** The bounded goroutine + Redis pubsub path is sufficient while running for a single user. Add Temporal when multi-user scale or restart resilience becomes a requirement.

---

### 7. MCP Surface (Model Context Protocol)

**What it does:** Exposes Cipher as an MCP server, allowing any MCP-compatible client (Claude Desktop, Cursor, Codex, OpenCode, etc.) to query the user's financial data through natural language, without installing anything beyond a config snippet.

**Why deferred:** MCP adds an external surface that must be auth-gated and budget-scoped. Internal agent correctness comes first.

#### Tool surface: keep it minimal, delegate the rest

Railway's production MCP server shipped with 7 tools and is actively working to reduce that number. Their reasoning applies directly here:

> "Every tool definition you expose lives in the client's prompt — every turn, before the user's actual work. A 7-tool surface is cheap. A 25-tool surface is not. Larger lists measurably hurt selection quality."

The tools that belong at the top level are only cleanly bounded, no-reasoning-needed operations. Everything that requires multi-step reasoning belongs inside a single delegation tool.

**Pennywise MCP tool surface (target):**

| Tool | What it does | Reasoning required |
|------|--------------|--------------------|
| `pennywise-agent` | Delegates a natural language question to the full Cipher agent | Yes — all of it, server-side |
| `get-schema` | Returns the DB schema so the client LLM can understand the data model | No |
| `get-today` | Returns today's date | No |

`execute_sql` is **not** exposed at the MCP surface. Raw SQL generation from an untrusted external client is a larger attack surface than is justified. The `pennywise-agent` tool handles queries that would require SQL — it runs the full internal pipeline (classify → resolve → plan → execute → summarise) and returns only the narrated answer.

#### The delegation tool pattern

`pennywise-agent` takes a single natural language string. Cipher does the multi-step work internally. The MCP client pays one context-cheap round-trip and never sees intermediate tool calls, SQL, raw rows, or internal IDs.

```
MCP client: "how much did I spend on food last month?"
     ↓  one tool call: pennywise-agent(query="...")
Cipher internal:
  classify → resolve payees → plan SQL → execute → local Ollama narrate
     ↓  one response: "You spent ₹8,420 on food in April across..."
MCP client receives narrated answer
```

This keeps orchestration complexity server-side. Every improvement to the internal agent pipeline — better planning, faster execution, smarter summarisation — is automatically available to every MCP client without any tool-surface breaking changes or schema churn. Usage telemetry on what `pennywise-agent` handles internally also reveals which capabilities (if any) might eventually earn promotion to a top-level tool.

#### Implementation: route handler inside cipher, not a separate service

Railway explicitly tried a separate MCP service first and abandoned it because they kept rebuilding what the main service already had — auth, sessions, permissions, encryption, dataloaders. Their final architecture is a route handler inside the main backend.

Apply the same decision here: the MCP handler is a route inside `cipher/cmd/api/main.go`, not a new binary. It reuses:

- The existing `InternalRequestAuth` + `BudgetIdMiddleware` middleware chain for auth and budget scoping.
- The existing `ToolRegistry` for the two simple tools (`get-schema`, `get-today`).
- The existing `agentService.CreateRun()` path for `pennywise-agent` delegation.

Context propagation follows the same pattern Railway used with `AsyncLocalStorage` — in Go this is `context.Context` already threaded through every handler and tool call via shared utils (`MustBudgetID`, `MustUserID`). No new context system needed.

#### Auth

The MCP endpoint accepts the same `Authorization: Bearer <jwt>` token issued by the Go API. No second auth system. The existing JWT validation middleware already handles this. Budget scoping is enforced by `BudgetIdMiddleware` — the `X-Budget-ID` header is required, same as every other budget-scoped route.

For remote/public exposure (e.g. connecting Claude Desktop to a hosted instance), the `.well-known/oauth-authorization-server` discovery document points back to the Go API's existing auth endpoints — no parallel OAuth state.

#### Transport

Use **Streamable HTTP** (the current MCP spec default) for the hosted endpoint. `stdio` remains available for local dev via a thin CLI wrapper over the same HTTP handler. Do not expose SSE on a multi-replica setup without sticky routing — SSE streams break across replicas without session affinity.

---

## Per-Step Model Configuration

The current code hardcodes the classify LLM separately but does not allow configuring provider/model per pipeline step without code changes. The target design:

```go
type StepName string

const (
    StepClassify   StepName = "classify"
    StepPlan       StepName = "plan"
    StepSummarise  StepName = "summarise"
    StepReasonFallback StepName = "reason_fallback" // used only if planner is skipped
)

type StepConfig struct {
    Provider string // "anthropic", "openai", "ollama"
    Model    string
}

type PipelineConfig struct {
    Steps map[StepName]StepConfig
}
```

The `agentService` holds one `llm.LLM` per step name, constructed from `PipelineConfig` at startup. Steps share clients where the config is identical.

**Default configuration:**

| Step | Provider | Model | Rationale |
|------|----------|-------|-----------|
| `classify` | Anthropic | `claude-haiku-4-5` | Cheap, fast, structured JSON |
| `plan` | Anthropic | `claude-sonnet-4-6` | Needs reliable SQL + DAG output |
| `summarise` | Ollama | `gemma3` | Local only — sees raw financial data |
| `reason_fallback` | Anthropic | `claude-sonnet-4-6` | ReAct fallback when planner is skipped |

---

## Privacy Boundary Summary

| Data | Classify | Scoped Context Load | Plan | Execute | Summarise |
|------|----------|---------------------|------|---------|-----------|
| User query text | ✅ sent | — | ✅ sent | — | ✅ local only |
| Category group names | ✅ sent | — | — | — | — |
| Category/payee display names | — | ✅ local DB query | ✅ sent | — | — |
| Summed spend per category (window) | — | ✅ local DB query | ✅ sent | — | — |
| Entity UUIDs | — | — | ✅ sent | ✅ used in SQL | — |
| Raw SQL queries | — | — | ✅ generated | ✅ executed locally | — |
| Individual transaction amounts | ❌ never | ❌ never | ❌ never | ❌ never leaves box | ✅ local only |
| Account balances | ❌ never | ❌ never | ❌ never | ❌ never leaves box | ✅ local only |
| Raw DB rows | ❌ never | ❌ never | ❌ never | ❌ never leaves box | ✅ local only |

**Note on summed spend totals:** Category-level spend aggregates (e.g. "Food: ₹8,420 in April") are sent to the cloud LLM in the micro-planner step. These are user-defined category labels and aggregated numbers — not individual transactions, not bank payee strings, not account numbers. This is the minimum signal the planner needs to reason about whether it can answer directly or needs to run additional queries.

---

## Migration Path from Current ReAct Loop

The ReAct loop is not removed — it becomes the `reason_fallback` step used when the micro-planner is disabled or produces an empty plan. This keeps the system working during migration.

**Phase 1 (next):**
- Add 500-row cap to `execute_sql`.
- Move agent wiring from `runtime.go` (dev harness) into `agentService.NewAgentService()`.
- Add clarify gate: if `IntentResult.DateRange` is unresolved, return a clarifying question immediately without running any DB queries.
- Replace `GetBudgetContext()` (full category/payee dump) with scoped context load: distinct category names + payee names + summed spend per category for the resolved date window only.
- Intercept the final `execute_sql` result in `Run()` and route raw rows to local Ollama for narration instead of back to the cloud LLM.

**Phase 2:**
- Implement `StepConfig` / `PipelineConfig` and wire per-step LLM clients in `agentService`.
- Implement the micro-planner: structured output prompt, `ExecutionPlan` parsing, fallback to ReAct on parse failure.
- Micro-planner receives scoped context (spend totals) and can answer directly without emitting any DAG steps for simple spending queries.

**Phase 3:**
- Implement the parallel DAG executor in Go. Plug it in after the planner.
- Keep the local Ollama summariser from Phase 1 as the final step.

**Phase 4:**
- Implement scoped context memory: `agent_memory` table in pgvector, keyed by `(budget_id, date_range)`. Store the scoped context result (category + payee + spend totals) as a long-lived memory entry.
- Before running the scoped DB query, check memory. On a hit, inject cached context and skip the DB query.
- Expose a manual memory invalidation path (user prompt: "use fresh data" / bulk import trigger).

**Phase 5:**
- Implement agent run persistence (`agent_runs`, `conversation_messages`).
- Implement conversation observational memory for long chat history: memory records/chunks, `PrepareContext`, async buffering, activation, and reflection.
- Add runtime memory hooks guarded by `WithMemoryEnabled`, and publish persisted-run events only after message content and run metadata patches succeed.
- Move title/observer/reflector calls onto a one-shot LLM runner instead of the full tool-enabled `Agent.Run` loop.
- Move execution into Temporal activities for durability.

**Phase 6:**
- Add MCP route handler inside `cipher/cmd/api/main.go` (not a new service).
- Expose three tools: `pennywise-agent` (delegation), `get-schema`, `get-today`.
- Auth via existing JWT middleware + `BudgetIdMiddleware`. No new auth system.
- `pennywise-agent` calls `agentService.CreateRun()` and returns the narrated answer.
- Do not expose `execute_sql` at the MCP surface.

---

## What Does Not Change

- The `LLM` interface and provider adapters (`AnthropicLLM`, `OpenAILLM`, `OllamaLLM`).
- The `ToolRegistry` and individual tool implementations.
- The `ContextBuilder` and `VectorResolver`.
- The Redis pubsub → WebSocket fanout path for streaming deltas to the client.
- The `ObservedLLM` wrapper for tracing.
- The `agent_runs` API surface in the Go API (`POST /api/agent/runs`, `GET /api/agent/runs/:id`, `POST /api/agent/runs/:id/cancel`).
