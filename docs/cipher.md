# Cipher: Financial Categorization Architecture

This document outlines the comprehensive multi-tiered architecture designed to process, normalize, and categorize chaotic Indian bank and UPI transaction strings into clean, user-specific YNAB-style ledgers.

Indian banking infrastructure produces highly fragmented transaction data. A single business might appear via multiple payment gateways (e.g., `PYU*Swiggy`, `Razorpay*Swiggy`), direct UPI handles, or varying Email/SMS templates depending on the bank. This architecture details the technical flow, database schemas, and critical engineering pivots made to tame this chaos efficiently using Go, PostgreSQL, and local edge-AI models.

---

## 1. Service Orchestration & The Distributed Monolith

The system utilizes an event-driven, distributed monolith architecture to balance deployment flexibility with ultra-low latency data access.

- **Ingestion (`go-gmail`):** A lightweight service triggered by Google PubSub. It intercepts the raw Gmail event and starts a Temporal `EmailToTransactionWorkflow`. The workflow invokes cipher's `PredictionActivity` for the full classification pipeline. The old python-mlp prediction path is deprecated and no longer used.
- **The Brain (`cipher`):** The core business logic service. It handles all LLM interactions, data extraction, and routing logic. Exposes `POST /api/predict`, `POST /api/corrections`, and `POST /api/workflows/:workflowId/retry-predict`.
- **The Ledger (`api`):** The backend that manages the frontend API and state mutations.
- **The Shared Repository (`shared/db`):** Rather than strict, latency-heavy microservice communication over HTTP/gRPC, both `cipher` and `api` import a shared Go repository package. This allows `cipher` to execute sub-millisecond database reads for classification rules without network hops, while keeping SQL queries centralized.

### Temporal Integration

The `EmailToTransactionWorkflow` (on `PennywiseTaskQueue`) is the primary ingestion path. `go-gmail` starts the workflow, which invokes cipher's `PredictionActivity` (on `CipherActivitiesTaskQueue`). The activity processes a batch of parsed emails and returns a `CipherPredictionResult` slice. A `RetryPredict` signal endpoint (`POST /api/workflows/:workflowId/retry-predict`) allows nudging a parked workflow when Ollama was unavailable at processing time.

---

## 2. The Four-Phase Classification Pipeline

To balance latency, compute cost, and AI accuracy, the system processes incoming transactions through a strict cascade.

### Phase 1: The Local AI Extractor (Ollama / gemma4)

Regex pipelines are fragile state machines that break upon minor bank template updates. Phase 1 intercepts the raw, chaotic string and passes it to a local small language model (SLM) (`gemma4` via Ollama) running strictly in JSON-output mode with a temperature of `0`.

- **Input:** `Alert: You've spent USD 12.00 on your CC XX1111 at GITHUB INC... equivalent INR is Rs. 1024.50.`
- **Few-Shot Prompting:** The LLM uses schema rules to bypass grammatical debris, resolve Forex traps (identifying 1024.50 over 12.00), and autonomously truncate account strings (e.g., dropping "CC XX" to just "1111").
- **Output:** `{"merchant": "GITHUB INC", "amount": 1024.5, "account_card": "HDFC 1111"}`

After extraction, `utils.CleanAccountString` normalises the account suffix, and `utils.CleanUPIText` / `utils.CleanMerchantString` produce a canonical merchant name and optional UPI handle used as the match string for Phase 2.

### Phase 2: The Fast Path (Payee Rules)

The Go backend takes the extracted merchant/UPI handle and queries the `payee_rules` table via the shared repository. The query prioritises `EXACT` matches, falling back to `PATTERN` (`ILIKE`) matches.

```sql
SELECT id, budget_id, payee_id, category_id, match_string, match_type, created_at, updated_at
FROM payee_rules
WHERE budget_id = $1
  AND deleted = FALSE
  AND (
    (match_type = 'EXACT' AND match_string = $2)
    OR
    (match_type = 'PATTERN' AND $2 ILIKE match_string)
  )
ORDER BY match_type ASC
LIMIT 1;
```

- **Result:** If a match is found, the transaction is categorised instantly with 100% confidence. _Latency: <5ms. Vector search and LLM are bypassed entirely._

### Phase 3: The Vector Memory (pgvector / bge-m3)

If Phase 2 finds no rule, the system moves to semantic matching.

- **Embedding:** The cleaned text (`"debit SWIGGY"` / `"credit SALARY"`) is embedded using the `bge-m3` model via Ollama, generating a 1024-dimension vector.
- **Hybrid Search:** A combined-score query orders results by vector distance plus a weighted amount penalty, scoped via `WHERE budget_id = $1`.
- **Heuristic Penalty:** To prevent false positives when amounts differ wildly, the ordering formula is:

```sql
(embedding <=> $1)
+ (ABS(ABS(amount) - ABS($amount)) / NULLIF(GREATEST(ABS(amount), ABS($amount)), 0) * 0.15)
```

- **Thresholds:** A similarity of `1 - vector_distance ≥ 0.80` is required for a match. A softer `0.70` threshold applies when the stored amount is exactly equal to the incoming amount.

### Phase 4: The Local LLM Fallback (Ollama)

If the transaction is entirely novel, the system triggers the LLM fallback — also via Ollama (not a cloud API).

- **Prompt (`prompts.go`):** A detailed `promptV1` includes hardcoded payee normalisation rules, keyword-to-category mappings for Indian merchants (Swiggy, Zomato, BESCOM, Indigo, etc.), and a YNAB-style category list specific to the budget.
- **Output:** A JSON object with `payee`, `category`, `confidence`, and `reasoning` fields.
- **Result:** The response is matched against existing payees/categories in the DB. The transaction is created and the embedding is stored for future Phase 3 hits.

---

## 3. The Conversational RAG Engine (Transaction Search) — _Planned, Not Yet Implemented_

To allow users to chat with their financial data (e.g., _"How much did I spend on food last month?"_), the architecture bypasses heavy NLP frameworks (like [Rasa](https://fold.money/blog/leveraging-nlp-in-transaction-search)) in favor of a 3-Step Retrieval-Augmented Generation pipeline.

> **Status:** This section describes the planned design. It has not been implemented yet.

1. **Step 1: The Intent Router (Local LLM):** The user's natural language query is passed to an LLM with the current system date. The LLM translates relative timeframes and fuzzy text into a strict JSON search intent (e.g., `{"intent": "sum", "categories": ["Food"], "date_start": "2026-03-01"}`).
2. **Step 2: The Context Assembler (Go Repository):** The Go backend dynamically builds the PostgreSQL queries (using tools like `squirrel`) based on the JSON intent. It executes standard aggregations or `pgvector` semantic searches, retrieving the hard financial facts to prevent LLM hallucinations.
3. **Step 3: The Synthesizer (Local LLM):** The raw database rows are injected into a final prompt. The LLM acts purely as a synthesizer, formatting the hard database math into a friendly, conversational text response.

---

## 4. Core Database Schema

### The Local Ledger Layer (Per User / Budget)

| Table            | Purpose                                      | Key Columns                                                              |
| :--------------- | :------------------------------------------- | :----------------------------------------------------------------------- |
| `payee_rules`    | Phase 2 Fast Path mapping rules.             | id, budget_id, payee_id, category_id, match_string, match_type (EXACT/PATTERN), deleted |
| `payees`         | The user's local alias engine and UI anchor. | id, budget_id, name, default_category_id                                 |
| `transactions`   | The actual financial ledger.                 | id, budget_id, amount, payee_id, category_id, original_statement_text    |

### The AI Memory Layer

| Table                    | Purpose                           | Key Columns                                                               |
| :----------------------- | :-------------------------------- | :------------------------------------------------------------------------ |
| `transaction_embeddings` | Fuzzy similarity matching memory. | id, budget_id, embedding_text, embedding (vector), payee_id, category_id, amount, source |

The `source` column in `transaction_embeddings` tracks whether an entry was created from an automatic prediction (`prediction`) or a manual user correction (`user_corrected`). On upsert, the stored amount is updated as a rolling average: `(NEW.amount + existing.amount) / 2`.

---

## 5. Key Learnings & Strategic Pivots

### Escaping the Regex Prison

Bank Email/SMS templates are unpredictable state machines. Attempting to extract entities via regex resulted in infinite edge cases. Shifting Phase 1 to a 4B parameter local LLM completely solved extraction logic. By utilizing Few-Shot prompting, the model natively understands how to ignore grammatical debris, correctly identify Forex billing amounts over base currency, and autonomously truncate messy account numbers (e.g., turning "A/C XXXXX1234" into "1234") without writing a single line of string manipulation code in Go.

### VRAM Cold Starts & Asynchronous Ingestion

Running local edge-AI (Ollama) introduced a 12-second hardware latency when loading the model into VRAM from a cold boot. This was mitigated by utilizing the hidden Ollama `keep_alive` parameter to retain the model in memory. Furthermore, leveraging PubSub for `go-gmail` ingestion naturally decoupled the event pipeline, ensuring background processing never blocks the user interface, even during inference spikes.

### Skipping Rasa for Search

Traditional financial search architectures rely on heavy NLP frameworks like Rasa + Duckling to parse intents and relative dates. By injecting the current system date into an LLM prompt, the model natively understands typo-forgiveness (e.g., "Zometo" -> "Zomato") and relative math ("Last month"), outputting perfect JSON search filters. This reduces a massive machine-learning architecture down to a single Go unmarshal endpoint.

### The Megamart Problem (Decoupling Payees)

Super-apps in India span multiple MCC categories (e.g., Swiggy Food vs. Instamart vs. Genie). By allowing a Payee's `default_category_id` to be explicitly `NULL`, the Phase 2 Fast Path can identify the consolidated payee ("Swiggy") while intentionally handing off the category detection to the Phase 3 Vector Search or Phase 4 LLM, allowing for dynamic categorization under a single UI Hub.
