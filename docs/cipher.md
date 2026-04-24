# Cipher: Financial Categorization Architecture

This document outlines the comprehensive multi-tiered architecture designed to process, normalize, and categorize chaotic Indian bank and UPI transaction strings into clean, user-specific YNAB-style ledgers.

Indian banking infrastructure produces highly fragmented transaction data. A single business might appear via multiple payment gateways (e.g., `PYU*Swiggy`, `Razorpay*Swiggy`), direct UPI handles, or varying Email/SMS templates depending on the bank. This architecture details the technical flow, database schemas, and critical engineering pivots made to tame this chaos efficiently using Go, PostgreSQL, and local edge-AI models.

---

## 1. Service Orchestration & The Distributed Monolith

The system utilizes an event-driven, distributed monolith architecture to balance deployment flexibility with ultra-low latency data access.

- **Ingestion (`go-gmail`):** A lightweight service triggered by Google PubSub. It intercepts the raw email, strips the metadata, and acts as a pure, dumb data pipe, passing the raw string to the brain.
- **The Brain (`cipher`):** The core business logic service. It handles all LLM interactions, data extraction, and routing logic.
- **The Ledger (`api`):** The backend that manages the frontend API and state mutations.
- **The Shared Repository (`shared/db`):** Rather than strict, latency-heavy microservice communication over HTTP/gRPC, both `cipher` and `api` import a shared Go repository package. This allows `cipher` to execute sub-millisecond database reads for classification rules without network hops, while keeping SQL queries centralized.

---

## 2. The Four-Phase Classification Pipeline

To balance latency, compute cost, and AI accuracy, the system processes incoming transactions through a strict cascade.

### Phase 1: The Local AI Extractor (Ollama + Gemma)

Regex pipelines are fragile state machines that break upon minor bank template updates. Phase 1 intercepts the raw, chaotic string and passes it to a local small language model (SLM) (e.g., Gemma 4b via Ollama) running strictly in JSON-output mode with a temperature of `0`.

- **Input:** `Alert: You've spent USD 12.00 on your CC XX1111 at GITHUB INC... equivalent INR is Rs. 1024.50.`
- **Few-Shot Prompting:** The LLM uses schema rules to bypass grammatical debris, resolve Forex traps (identifying 1024.50 over 12.00), and autonomously truncate account strings (e.g., dropping "CC XX" to just "1111").
- **Output:** `{"merchant": "GITHUB INC", "amount": 1024.5, "account_card": "1111"}`

### Phase 2: The Fast Path (Hybrid SQL Rules)

The Go backend takes the Phase 1 extracted merchant and queries the `merchant_rules` table via the shared repository. This utilizes a hybrid SQL query prioritizing `EXACT` matches, falling back to `PATTERN` (`ILIKE`) matches.

- **Execution:** `SELECT category_id FROM merchant_rules WHERE budget_id = $1 AND ((match_type = 'EXACT' AND merchant = $2) OR (match_type = 'PATTERN' AND $2 ILIKE merchant)) ORDER BY match_type ASC LIMIT 1;`
- **Result:** If a match is found, the transaction is categorized instantly. _Latency: <5ms. Cloud AI is bypassed entirely._

### Phase 3: The Vector Memory (pgvector / bge-m3)

If Phase 2 fails to find a hard SQL rule, the system moves to semantic matching.

- **Embedding:** The cleaned text is embedded using the `bge-m3` model, generating a 1024-dimension vector.
- **Hybrid Search:** It performs a Nearest-Neighbor (K-NN) search against the `transaction_embeddings` table, scoped strictly via a `WHERE budget_id = $1` clause.
- **Heuristic Penalty:** To prevent false positives, the vector distance score is penalized if the transaction amounts differ wildly.
  Formula used:

```
ABS(amount - transaction_amount) / GREATEST(amount, transaction_amount)) * 0.15
```

### Phase 4: The Cloud LLM Bridge (Cold Start)

If the transaction is entirely novel, the system triggers the heavy AI fallback.

- **Normalization:** An LLM normalizes the chaotic text into a clean corporate brand and assigns a universal `global_mcc_tag`.
- **Translation:** The LLM maps this universal tag to the specific user's categories, saving the result back down the chain to build Phase 2 and Phase 3 memory for the future.

---

## 3. The Conversational RAG Engine (Transaction Search)

To allow users to chat with their financial data (e.g., _"How much did I spend on food last month?"_), the architecture bypasses heavy NLP frameworks (like [Rasa](https://fold.money/blog/leveraging-nlp-in-transaction-search)) in favor of a 3-Step Retrieval-Augmented Generation pipeline.

1. **Step 1: The Intent Router (Local LLM):** The user's natural language query is passed to an LLM with the current system date. The LLM translates relative timeframes and fuzzy text into a strict JSON search intent (e.g., `{"intent": "sum", "categories": ["Food"], "date_start": "2026-03-01"}`).
2. **Step 2: The Context Assembler (Go Repository):** The Go backend dynamically builds the PostgreSQL queries (using tools like `squirrel`) based on the JSON intent. It executes standard aggregations or `pgvector` semantic searches, retrieving the hard financial facts to prevent LLM hallucinations.
3. **Step 3: The Synthesizer (Local LLM):** The raw database rows are injected into a final prompt. The LLM acts purely as a synthesizer, formatting the hard database math into a friendly, conversational text response.

---

## 4. Core Database Schema

### The Global Layer (Cross-User Truth)

| Table                      | Purpose                                               | Key Columns                            |
| :------------------------- | :---------------------------------------------------- | :------------------------------------- |
| `global_merchants`         | The canonical identity of a business.                 | id (UUID), canonical_name, mcc_tag     |
| `global_merchant_mappings` | The dictionary linking chaotic strings to the entity. | mapping_trigger (PK), merchant_id (FK) |

### The Local Ledger Layer (Per User)

| Table            | Purpose                                      | Key Columns                                                              |
| :--------------- | :------------------------------------------- | :----------------------------------------------------------------------- |
| `merchant_rules` | Phase 2 Fast Path mapping rules.             | id, budget_id, merchant_pattern, category_id, match_type (EXACT/PATTERN) |
| `payees`         | The user's local alias engine and UI anchor. | id, budget_id, name, default_category_id                                 |
| `transactions`   | The actual financial ledger.                 | id, budget_id, amount, payee_id, category_id, original_statement_text    |

### The AI Memory Layer

| Table                    | Purpose                           | Key Columns                                               |
| :----------------------- | :-------------------------------- | :-------------------------------------------------------- |
| `transaction_embeddings` | Fuzzy similarity matching memory. | id, budget_id, embedding_text, embedding (vector), amount |

---

## 5. Key Learnings & Strategic Pivots

### Escaping the Regex Prison

Bank Email/SMS templates are unpredictable state machines. Attempting to extract entities via regex resulted in infinite edge cases. Shifting Phase 1 to a 4B parameter local LLM completely solved extraction logic. By utilizing Few-Shot prompting, the model natively understands how to ignore grammatical debris, correctly identify Forex billing amounts over base currency, and autonomously truncate messy account numbers (e.g., turning "A/C XXXXX1234" into "1234") without writing a single line of string manipulation code in Go.

### VRAM Cold Starts & Asynchronous Ingestion

Running local edge-AI (Ollama) introduced a 12-second hardware latency when loading the model into VRAM from a cold boot. This was mitigated by utilizing the hidden Ollama `keep_alive` parameter to retain the model in memory. Furthermore, leveraging PubSub for `go-gmail` ingestion naturally decoupled the event pipeline, ensuring background processing never blocks the user interface, even during inference spikes.

### Skipping Rasa for Search

Traditional financial search architectures rely on heavy NLP frameworks like Rasa + Duckling to parse intents and relative dates. By injecting the current system date into an LLM prompt, the model natively understands typo-forgiveness (e.g., "Zometo" -> "Zomato") and relative math ("Last month"), outputting perfect JSON search filters. This reduces a massive machine-learning architecture down to a single Go unmarshal endpoint.

### The "Fold" vs "YNAB" Architecture Clash

Standard categorization engines utilize a rigid, global MCC tag system. YNAB-style apps rely on custom user buckets. The solution was to maintain a massive internal `global_mcc_tag` ENUM for the system's objective understanding, but introduce an LLM Bridge during Phase 4 to seamlessly translate those global tags into the user's subjective categories.

### The Megamart Problem (Decoupling Payees)

Super-apps in India span multiple MCC categories (e.g., Swiggy Food vs. Instamart vs. Genie). By allowing a Payee's `default_category_id` to be explicitly `NULL`, the Phase 2 Fast Path can identify the consolidated payee ("Swiggy") while intentionally handing off the category detection to the Phase 3 Vector Search or Phase 4 LLM, allowing for dynamic categorization under a single UI Hub.
