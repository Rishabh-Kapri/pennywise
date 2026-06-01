You can hit 2–3 s turn time and keep accuracy by running a two-speed agent: a fast router for easy turns and a bounded micro-planner that emits a tiny DAG for multi-tool work. Enforce JSON-schema I/O at the MCP edge. Run independent tools in parallel. Stream from the first token. Cache tool results and LLM prefixes. Gate risky calls with interrupts. This mirrors what “feels” like Claude.

# Architecture that works in 2024–2025

## Overview

Router → Micro-planner → Graph executor → MCP tools.

* **Router** (cheap model). Classifies `single_tool | multi_step | clarify`.
* **Micro-planner** (only on `multi_step`). Produces a small DAG: up to 4 steps, depth up to 2, with explicit dependencies.
* **Graph executor**. Runs ready nodes in parallel, persists state, supports retries and **interrupts** for clarifying questions or approvals. LangGraph gives graphs, streaming, and interrupts; LlamaIndex Workflows gives event-driven steps with `num_workers` concurrency. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/graph-api "Graph API overview - Docs by LangChain"))
* **MCP edge**. Define every tool with JSON Schema, validate before call, and surface missing required fields to the planner as `clarify`. Use the official spec to model tools, prompts, and security. ([Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18 "Specification"))

## Why this beats ReAct and full planners

* ReAct is greedy and local. It often fabricates arguments and stalls on multi-tool dependencies. Keep ReAct-like reasoning inside each node only. ([arXiv](https://arxiv.org/abs/2210.03629 "ReAct: Synergizing Reasoning and Acting in Language Models"))
* Full planners add passes and re-plans. Instead, plan small then execute a DAG in parallel. LLMCompiler shows lower wall time from parallel function calling with a planner plus executor. ([arXiv](https://arxiv.org/abs/2312.04511 "[2312.04511] An LLM Compiler for Parallel Function Calling"))

# Concrete open-source stack

* **Serving**: vLLM with Automatic Prefix Caching and structured outputs for tool calling. SGLang is an alternative with RadixAttention and cache-aware scheduling. Both support continuous batching and speculative options. ([VLLM Docs](https://docs.vllm.ai/en/latest/features/automatic_prefix_caching/ "Automatic Prefix Caching - vLLM"))
* **Models**: Qwen 3 or 2.5 for strong function calling, Mistral function calling, Llama with strict JSON outputs. Qwen-Agent demonstrates parallel multi-step, multi-turn calls. ([GitHub](https://github.com/QwenLM/Qwen-Agent "QwenLM/Qwen-Agent"))
* **Orchestration**: LangGraph for stateful graphs and interrupts, or LlamaIndex Workflows for event steps with `num_workers`. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/graph-api "Graph API overview - Docs by LangChain"))
* **Structured decoding**: vLLM structured outputs or libraries like Outlines and LM-Format-Enforcer to guarantee valid JSON for `plan` and `tool_args`. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
* **MCP servers**: follow the spec and copy a pragmatic folder layout. Milvus shows a sensible MCP server structure and a reference server. ([Milvus](https://milvus.io/ai-quick-reference/what-is-the-recommended-filefolder-structure-for-an-model-context-protocol-mcp-server-project "What is the recommended file/folder structure for an Model ..."))

# Latency plan to reach ~2–3 s P50

1. **Cut LLM passes**

   * Route first. Only invoke the planner when the router says `multi_step`.
   * Generate arguments for a parallel group in one constrained JSON, not one call per tool. Use vLLM structured outputs. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
2. **Exploit cache**

   * **Prefix/KV cache**. Keep system prompt and tool table stable to maximize APC hit rate. ([VLLM Docs](https://docs.vllm.ai/en/latest/features/automatic_prefix_caching/ "Automatic Prefix Caching - vLLM"))
   * **Tool I/O cache**. Memoize by `(tool, normalized_args)` with TTL and user scope.
3. **Run tools in parallel**

   * Execute independent nodes concurrently. LlamaIndex `num_workers` and LangGraph parallel branches cover this. ([LlamaIndex](https://developers.llamaindex.ai/python/framework/understanding/workflows/concurrent_execution/ "Concurrent execution of workflows"))
4. **Stream early**

   * Stream planner acknowledgment and node progress. LangGraph supports streamed interrupts and state updates. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))
5. **Speculative and batching**

   * Enable continuous batching. Evaluate speculative decoding on your model server if available. ([GitHub](https://github.com/sgl-project/sglang "SGLang is a fast serving framework for large language ..."))

# Clarifying questions that do not tank UX

* Add a **clarify gate** before any high-impact call. If required schema fields are unknown or the router flags ambiguity, trigger an **interrupt** that asks one concise question, then resume at the same graph node. LangGraph interrupts persist state until the user replies. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))

# Security for remote MCP tools

* Apply the MCP security guidance: validate tool inputs and outputs, rate-limit, add user confirmation for sensitive operations, and log calls. Pair with OWASP LLM Top-10 controls for prompt-injection and insecure output handling. ([Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18/server/tools "Tools"))

# Claude-like behavior: what to copy

* **Tight MCP integration and short plans**. Anthropic’s material shows MCP first-class support in the host. You can reproduce the feel by planning small, executing in parallel, and streaming. ([Model Context Protocol](https://modelcontextprotocol.io/ "Model Context Protocol"))
* **Constrained arguments**. Make the model emit tool calls that already conform to schema via guided decoding. That removes repair loops and mis-calls. vLLM and Outlines support this. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))

# Reference prompts and skeletons

## Router prompt

Return strict JSON. No prose.

```json
{"decision":"single_tool|multi_step|clarify",
 "tool":"<name|null>",
 "missing":["field_a","field_b"],
 "why":"<10 words>"}
```

Enforce with structured decoding on the server. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))

## Micro-planner prompt

Emit a tiny DAG or ask one question.

```json
{"steps":[
  {"id":"A","tool":"search","args":{"q":"<...>"},"after":[]},
  {"id":"B","tool":"lookup","args":{"id":"$A.id"},"after":["A"]}
],
"clarify":null}
```

If any required field is unknown, set `"clarify":"<single question>"`. The executor uses topological order to parallelize nodes with empty `after`. ([arXiv](https://arxiv.org/abs/2312.04511 "[2312.04511] An LLM Compiler for Parallel Function Calling"))

## Executor sketch

```python
# Pseudo-only. Comments include sources.
# Graph runtime: LangGraph (https://docs.langchain.com/.../graph-api)
# Workflows concurrency: https://developers.llamaindex.ai/.../concurrent_execution/
def execute(dag, mcp):
    ready = topo_sort_ready(dag)
    while ready:
        # launch independent nodes concurrently
        futs = [run_node(n, mcp) for n in ready]
        results = wait_all(futs)
        persist(results)       # checkpoint
        ready = unblock(dag, results)  # release dependents

def run_node(node, mcp):
    args = node.args
    validate_jsonschema(node.tool.schema, args)  # fail fast
    # For risky tools:
    interrupt({"confirm": {"tool": node.tool.name, "args": args}})  # LangGraph interrupt
    return mcp.call(node.tool.name, args)
```

# Practical knobs that move your P50

* Stable system prompt and stable tool registry text to maximize APC reuse. ([VLLM Docs](https://docs.vllm.ai/en/latest/features/automatic_prefix_caching/ "Automatic Prefix Caching - vLLM"))
* Single JSON for all args in a parallel batch. One LLM call, many tools. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
* Low temperature and small top-p for `plan` and `tool_args`. Normal settings for final prose.
* Per-tool timeouts and bounded retries. Retry only the failed subgraph.
* Add node-level cache and avoid recomputing deterministic steps.

# Evaluation and tracing

* Test real tool use, not only chat. GAIA evaluates assistant skills including tool use and web. WebArena and VisualWebArena stress long-horizon web tasks. ([proceedings.iclr.cc](https://proceedings.iclr.cc/paper_files/paper/2024/file/25ae35b5b1738d80f1f03a8713e405ec-Paper-Conference.pdf "A BENCHMARK FOR GENERAL AI ASSISTANTS"))
* Trace every node: spans, inputs, outputs, latency, cache hits. This makes slow steps obvious and debuggable.

# Current, high-signal references

* **MCP spec and build guides**: protocol, tools, prompts, auth, and security. ([Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18 "Specification"))
* **LangGraph**: Graph API and interrupts for HITL. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/graph-api "Graph API overview - Docs by LangChain"))
* **LlamaIndex Workflows**: concurrency with `num_workers`. ([LlamaIndex](https://developers.llamaindex.ai/python/framework/understanding/workflows/concurrent_execution/ "Concurrent execution of workflows"))
* **LLMCompiler**: plan once, parallel function execution via DAG. ([arXiv](https://arxiv.org/abs/2312.04511 "[2312.04511] An LLM Compiler for Parallel Function Calling"))
* **vLLM**: tool calling, structured outputs, Automatic Prefix Caching. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/tool_calling.html "Tool Calling - vLLM"))
* **SGLang**: RadixAttention and cache-aware scheduling. ([GitHub](https://github.com/sgl-project/sglang "SGLang is a fast serving framework for large language ..."))
* **HF Transformers tool-use**: portable tool interface across models. ([Hugging Face](https://huggingface.co/docs/transformers/en/chat_extras "Tool use"))
* **Outlines and LM-Format-Enforcer**: hard guarantees for JSON. ([Dottxt AI](https://dottxt-ai.github.io/outlines/ "Outlines"))
* **Qwen-Agent**: parallel multi-tool examples on OSS models. ([GitHub](https://github.com/QwenLM/Qwen-Agent "QwenLM/Qwen-Agent"))
* **Milvus MCP examples and structure**: folder layout and server patterns. ([Milvus](https://milvus.io/ai-quick-reference/what-is-the-recommended-filefolder-structure-for-an-model-context-protocol-mcp-server-project "What is the recommended file/folder structure for an Model ..."))

---

---

Use a two-speed design: a fast router for easy turns and a bounded micro-planner that emits a tiny DAG for multi-tool turns, executed by a graph runtime with interrupts, parallel branches, and strict JSON-schema I/O at the MCP edge. Serve the model behind an engine with prefix/KV caching and structured outputs for reliable tool calls. ([Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18 "Specification"))

# Overview

**Goal.** Keep 2–3 s turns without giving up correctness. Achieve this by separating fast routing from small, parallel plans and by constraining tool I/O.

**Roles.**

* **Router (cheap model).** Classifies each turn as `single_tool | multi_step | clarify`. If any required fields for the best tool are unknown, return `clarify`. This minimizes planning calls and enforces a clarify-first gate. Use structured outputs so the router must emit valid JSON. Engines like vLLM support structured outputs and named tool calling. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
* **Micro-planner (only when needed).** Produces a **small DAG** (≤4 steps, depth ≤2) with explicit dependencies. The plan is a compact JSON object, not a verbose chain. This mirrors compiler-style planning that decomposes a query into interdependent tasks for **parallel** execution. ([arXiv](https://arxiv.org/abs/2312.04511 "[2312.04511] An LLM Compiler for Parallel Function Calling"))
* **Graph executor.** Runs the DAG with:

  * **Parallel branches** and barriered joins to shorten the critical path.
  * **Interrupts** for human-in-the-loop clarifications or approvals, resuming exactly where paused.
  * **Streaming** of node progress and final tokens.
    LangGraph documents `interrupt()` for waiting on user input; LlamaIndex Workflows exposes `num_workers` for step concurrency. ([LangChain](https://langchain-ai.github.io/langgraph/how-tos/human_in_the_loop/wait-user-input/ "How to wait for user input using interrupt - GitHub Pages"))
* **MCP edge (typed tools).** Define every tool with JSON Schema on the server and validate locally before each call. MCP standardizes the host↔client↔server contract, so your chatbot can call remote tools consistently across vendors. Keep tool specs short and stable. ([Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18 "Specification"))
* **Serving layer.** Use a runtime with **Automatic Prefix Caching** so repeated system prompts and tool tables reuse KV blocks. Pair with **structured outputs** to guarantee JSON for `plan` and `tool_args`. Both cut retries and prefill time. ([VLLM Docs](https://docs.vllm.ai/en/latest/design/prefix_caching/ "Automatic Prefix Caching - vLLM"))

**Data flow.**

1. Router emits `{decision, tool?, missing[]}`.
2. If `clarify`: executor interrupts, asks one question, resumes on reply. ([LangChain](https://langchain-ai.github.io/langgraph/how-tos/human_in_the_loop/wait-user-input/ "How to wait for user input using interrupt - GitHub Pages"))
3. If `single_tool`: fill args → validate against schema → call tool → synthesize answer. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
4. If `multi_step`: micro-planner emits DAG → executor runs ready nodes in **parallel**, retries failed nodes, then synthesizes. ([arXiv](https://arxiv.org/pdf/2312.04511 "An LLM Compiler for Parallel Function Calling"))

# Why this beats ReAct and full planners

**ReAct’s limits in multi-tool work.** ReAct interleaves thoughts and actions step-by-step. It works on short horizons but is **myopic** for tool chains: it chooses the next tool greedily, often without a global view of dependencies, and can fabricate arguments inline. The original paper shows benefits on WebShop/ALFWorld and QA, but it does not address parallel dependency execution or schema-constrained I/O. Using ReAct *inside* a node is fine; using it as the global policy is brittle for MCP pipelines. ([arXiv](https://arxiv.org/abs/2210.03629 "ReAct: Synergizing Reasoning and Acting in Language Models"))

**Parallel DAGs cut wall-time without sacrificing accuracy.** Compiler-style planners build a dependency graph once, then execute **independent functions in parallel** and only synchronize when needed. Empirical results show this reduces latency versus sequential ReAct while preserving correctness, because the critical path becomes the longest dependency chain, not the sum of all steps. ([arXiv](https://arxiv.org/abs/2312.04511 "[2312.04511] An LLM Compiler for Parallel Function Calling"))

**Graphs match how complex reasoning actually branches.** Research on Graph-of-Thoughts motivates representing intermediate reasoning as a **graph** rather than a single chain. This structure enables concurrent sub-tasks and selective consolidation, aligning with a micro-planner + graph executor. ([arXiv](https://arxiv.org/abs/2308.09687 "Graph of Thoughts: Solving Elaborate Problems with Large Language Models"))

**Clarify-first gates are explicit, not emergent.** Methods like Self-Ask show that asking targeted sub-questions **before** answering improves multi-step accuracy. The graph runtime turns this into a first-class interrupt rather than hoping the model self-asks mid-trace. ([arXiv](https://arxiv.org/abs/2210.03350 "Measuring and Narrowing the Compositionality Gap in ..."))

**Runtime optimizations are built in.**

* **Prefix/KV caching** reuses shared prompt segments across turns to reduce prefill latency.
* **Structured outputs** remove JSON repair loops for tool arguments and plans.
  Together these cut both compute and failure modes that inflate latency in ReAct or long, full-dialog planners. ([VLLM Docs](https://docs.vllm.ai/en/latest/design/prefix_caching/ "Automatic Prefix Caching - vLLM"))

**MCP standardizes the tool plane.** With MCP, the host orchestrates remote tools through a uniform protocol, so your agent can keep plans **small and typed** and focus on execution, not bespoke adapters. This consistency is a key ingredient of the “fast and fluid” feel you saw in Claude Desktop. ([Model Context Protocol](https://modelcontextprotocol.io/specification/2025-06-18 "Specification"))

---

Use this concrete OSS stack:

* **Serving:** vLLM or SGLang for low latency, KV-reuse, and structured outputs. vLLM gives Automatic Prefix Caching, continuous batching, tool-calling, structured outputs, speculative decoding, and quantization. SGLang adds RadixAttention plus cache-aware scheduling. These features reduce prefill time and retries and keep turns near 2–3 s when plans stay small. ([VLLM Docs](https://docs.vllm.ai/ "vLLM Documentation"))

* **Orchestration:** LangGraph or LlamaIndex Workflows. Both run **DAGs with parallel branches** and support **interrupts** for clarifications. LangGraph exposes first-class interrupts, persistence/checkpointing, and streaming; Workflows gives per-step concurrency with `num_workers`. Use them to execute independent MCP tools concurrently and pause only when a required argument is missing. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))

* **Models:** Qwen, Mistral, or Llama with function/structured output support. Qwen-Agent shows **parallel, multi-step, multi-turn** tool calls. Mistral docs and cookbooks cover function calling end-to-end. Llama provides **JSON structured output** guidance. These three are the most reliable OSS options for tool use in 2024–2025. ([GitHub](https://github.com/QwenLM/Qwen-Agent "QwenLM/Qwen-Agent"))

* **Structured decoding (hard guarantees):** Use vLLM structured outputs where possible. If you need provider-agnostic enforcement, add **Outlines** or **LM-Format-Enforcer**. This prevents hallucinated tool arguments and invalid plan JSON. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))

* **MCP tool plane:** Follow the MCP spec and keep servers schema-first. For a concrete server layout, copy Milvus’ recommended MCP project structure and tutorial. This keeps tool catalogs short, typed, and cacheable. ([Model Context Protocol](https://modelcontextprotocol.io/ "Model Context Protocol"))

# How these pieces fit

## 1) Serving layer: fast, constrained, cache-friendly

* **KV-reuse:** vLLM **Automatic Prefix Caching** reuses shared prompt segments across turns. Keep the system prompt + tool registry stable. Continuous batching keeps GPUs saturated and lowers p50. Speculative decoding further cuts per-token latency when compatible. ([VLLM Docs](https://docs.vllm.ai/ "vLLM Documentation"))
* **Structured tool calls:** vLLM supports OpenAI-style tool calling and structured outputs so the model emits valid JSON for `plan` and `tool_args`. This removes repair loops. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/tool_calling.html "Tool Calling - vLLM"))
* **Quantization:** When memory-bound, enable INT8/FP8/AWQ/GPTQ or use the vLLM compressor toolkit. Throughput improves with minor quality tradeoffs if you pick the right scheme. ([VLLM Docs](https://docs.vllm.ai/en/latest/features/quantization/ "Quantization - vLLM"))
* **SGLang alternative:** **RadixAttention** stores many request histories in a radix tree to maximize prefix hits; cache-aware scheduling boosts reuse. It also supports speculative decoding and structured outputs. Useful if you expect heavy multi-turn traffic. ([GitHub](https://github.com/sgl-project/sglang "SGLang is a fast serving framework for large language ..."))

> Minimal server:

```bash
# vLLM OpenAI-compatible server
# docs: https://docs.vllm.ai/  (APC, batching, structured outputs)
vllm serve meta-llama/Llama-3.1-70B-Instruct \
  --port 8000 \
  --tensor-parallel-size 2 \
  --max-num-seqs 256 \
  --block-size 16
# Structured outputs + tool calling are driven by request params.  # docs: https://docs.vllm.ai/en/stable/features/structured_outputs.html ; https://docs.vllm.ai/en/stable/features/tool_calling.html
```

([VLLM Docs](https://docs.vllm.ai/ "vLLM Documentation"))

## 2) Orchestration: graph with parallel branches and interrupts

* **Why graphs:** You avoid serial ReAct loops. Ready nodes run in parallel and only **join** on dependencies. You pause the graph with an **interrupt** to ask for a missing required field, then resume exactly where you left off. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/graph-api "Graph API overview - Docs by LangChain"))
* **LangGraph:** Interrupts, streaming, persistence. Checkpoints enable long-running workflows, approvals, and fault tolerance. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))
* **LlamaIndex Workflows:** Decorate steps with `num_workers` to control concurrency; examples show fan-out/fan-in quickly. ([LlamaIndex](https://developers.llamaindex.ai/python/framework/understanding/workflows/concurrent_execution/ "Concurrent execution of workflows"))

> Graph skeleton:

```python
# Orchestrator: LangGraph (graphs, interrupts, streaming)
# docs: https://docs.langchain.com/oss/python/langgraph/graph-api
# interrupts: https://docs.langchain.com/oss/python/langgraph/interrupts
from langgraph.graph import StateGraph, END
from langgraph.checkpoint.sqlite import SqliteSaver  # persistence  # docs: https://docs.langchain.com/oss/python/langgraph/persistence

def router(state): ...
def plan(state): ...
def run_tool_A(state): ...
def run_tool_B(state): ...
def clarify(state):
    # Pause for required field, then resume
    from langgraph.func import interrupt
    return interrupt({"question": state["missing"]})  # waits persistently  # docs: interrupts

g = StateGraph(dict)
g.add_node("router", router)
g.add_node("plan", plan)
g.add_node("toolA", run_tool_A)
g.add_node("toolB", run_tool_B)
g.add_node("clarify", clarify)

g.add_edge("router", "plan")
g.add_conditional_edges("plan", lambda s: s["route"], {
    "clarify": "clarify",
    "single": "toolA",
    "multi": "toolA",  # will unlock toolB after A if dependent
})
# Parallelize toolA/toolB by unlocking when deps resolve
app = g.compile(checkpointer=SqliteSaver("agent.db"))
```

([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/graph-api "Graph API overview - Docs by LangChain"))

## 3) Models that behave well with tools

* **Qwen + Qwen-Agent:** Templates and parsers for function calling, with examples of **parallel, multi-step, multi-turn** tool use. Good default for OSS tool use. ([GitHub](https://github.com/QwenLM/Qwen-Agent "QwenLM/Qwen-Agent"))
* **Mistral:** Official docs and cookbook for function calling and agent integrations. Clean minimal prompts. ([docs.mistral.ai](https://docs.mistral.ai/capabilities/function_calling "Function calling | Mistral Docs"))
* **Llama:** Official **JSON structured output** makes plan/args predictable. ([llama.developer.meta.com](https://llama.developer.meta.com/docs/features/structured-output/ "JSON Structured Output"))

> Tool-use via HF Transformers:

```python
# HF Transformers "Tool use" works across Qwen, Mistral, Llama
# docs: https://huggingface.co/docs/transformers/en/chat_extras
from transformers import AutoTokenizer, AutoModelForCausalLM
# define tools=[], call chat template with tools, parse tool calls → MCP client
```

([Hugging Face](https://huggingface.co/docs/transformers/en/chat_extras "Tool use"))

## 4) Enforce structure to stop hallucinated tool inputs

* **vLLM structured outputs:** Native support for xgrammar/guidance backends. Use it for `router`, `plan`, and `tool_args` turns. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
* **Outlines / LM-Format-Enforcer:** Provider-agnostic constrained decoding by JSON Schema, regex, or CFG. Helpful when you swap models/servers. ([dottxt-ai.github.io](https://dottxt-ai.github.io/outlines/ "Outlines"))

> Example JSON schemas (router/plan):

```json
// router.schema.json
{"type":"object","properties":{
 "decision":{"enum":["single_tool","multi_step","clarify"]},
 "tool":{"type":["string","null"]},
 "missing":{"type":"array","items":{"type":"string"}},
 "why":{"type":"string","maxLength":32}},
 "required":["decision","missing"]}

// plan.schema.json
{"type":"object","properties":{
 "steps":{"type":"array","items":{
   "type":"object","properties":{
     "id":{"type":"string"},
     "tool":{"type":"string"},
     "args":{"type":"object"},
     "after":{"type":"array","items":{"type":"string"}}},
   "required":["id","tool","args","after"]}},
 "clarify":{"type":["string","null"]}},
 "required":["steps"]}
```

## 5) MCP layer: clean, typed, and testable

* **Spec + ecosystem:** MCP standardizes how your host talks to remote tools. Keep tool specs short with strict JSON schema. ([Model Context Protocol](https://modelcontextprotocol.io/ "Model Context Protocol"))
* **Server layout:** Copy Milvus’ suggested MCP server folder structure and tutorial to keep tools modular and testable. ([Milvus](https://milvus.io/ai-quick-reference/what-is-the-recommended-filefolder-structure-for-an-model-context-protocol-mcp-server-project "What is the recommended file/folder structure for an Model ..."))
* **Cross-stack adoption:** Official spec and repos are maintained under `modelcontextprotocol/*`. This keeps you aligned with hosts like Claude Desktop or other MCP-aware apps. ([GitHub](https://github.com/modelcontextprotocol/modelcontextprotocol "Specification and documentation for the Model Context ..."))

---

## Practical setup checklist

1. **Start vLLM (or SGLang) with caching on and streaming to client.** vLLM: APC + continuous batching; SGLang: RadixAttention + cache-aware scheduling. ([VLLM Docs](https://docs.vllm.ai/ "vLLM Documentation"))
2. **Constrain all non-prose turns** (`router`, `plan`, `tool_args`) with structured outputs. Prefer server-side enforcement. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))
3. **Graph executor** with **parallel branches** and **interrupts** for clarifications. Persist state for reliability. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))
4. **Pick a tool-savvy model** (Qwen/Mistral/Llama) and keep tool definitions minimal and typed. ([GitHub](https://github.com/QwenLM/Qwen-Agent "QwenLM/Qwen-Agent"))
5. **Cache tool I/O** by `(tool, normalized_args)` with TTL. Keep plans tiny to maximize KV reuse.
6. **Trace and test** with realistic multi-tool flows; adjust `num_workers` and timeouts to meet your p50.

---

## Known pitfalls and fixes

* **Invalid JSON/tool args:** Always enforce schemas; do not rely on “JSON-by-prompt.” Outlines/LMFE solve this. ([GitHub](https://github.com/dottxt-ai/outlines "dottxt-ai/outlines: Structured Outputs"))
* **Planner latency:** Plan **once**, execute DAG, and replan only failing subgraphs. Use interrupts for missing slots instead of re-thinking the whole plan. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))
* **Runtime quirks under heavy parallel tool calls:** Validate your stack; watch issues on SGLang and model repos for edge cases in parallel tool use. ([GitHub](https://github.com/sgl-project/sglang/issues/7117 "[Bug] The issue of parallel tool calls when deploying ..."))

---

## Small end-to-end wiring (request side)

```python
# OpenAI-compatible call against vLLM with structured outputs + tools
# vLLM tool calling: https://docs.vllm.ai/en/stable/features/tool_calling.html
# vLLM structured outputs: https://docs.vllm.ai/en/stable/features/structured_outputs.html
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8000/v1", api_key="sk-...")

router_schema = {...}  # see JSON above
msg = [{"role":"system","content":"You are a router..."}, {"role":"user","content": turn}]
resp = client.chat.completions.create(
  model="meta-llama/Llama-3.1-70B-Instruct",
  messages=msg,
  response_format={"type":"json_schema","json_schema":router_schema},  # structured outputs
)
router = json.loads(resp.choices[0].message.content)
# if router["decision"]=="multi_step": call planner with plan.schema.json; else call MCP tool directly
```

([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))

---

### Why this “concrete stack” matches your goals

* It keeps turns fast by **reducing model passes**, **reusing KV prefixes**, and **executing tools in parallel**.
* It keeps accuracy by **forcing JSON schemas** and **pausing** for clarifications via **interrupts**.
* It stays portable and OSS-friendly: **HF Transformers** for tool use, **vLLM/SGLang** for serving, **LangGraph/Workflows** for orchestration, and **MCP** for governed tools. ([Hugging Face](https://huggingface.co/docs/transformers/en/chat_extras "Tool use"))

---

Here is what to copy from Claude’s behavior, and how to harden remote MCP tools. I keep it concrete.

# Claude-like behavior: what to copy

## 1) Treat tools as first-class, typed contracts

* Define tools with clear JSON schemas and let the model emit *tool_use* blocks, then feed *tool_result* back. This tight loop is how Claude stays reliable under tool use. Implement the same contract in your stack. ([Claude Docs](https://docs.claude.com/en/api/messages "Messages"))
* Keep tool definitions short and specific. Anthropic’s guides emphasize crisp schemas and error handling, plus parallel tool execution when possible. Mirror that formatting and discipline. ([Claude Docs](https://docs.claude.com/ja/docs/agents-and-tools/tool-use/implement-tool-use "ツール使用の実装方法"))

**Replicate:** enforce schema-valid JSON on the *model side* (structured outputs) and on the *server side* (MCP tool input validation). Do not rely on “JSON-by-prompt” alone. ([Claude Docs](https://docs.claude.com/ja/docs/agents-and-tools/tool-use/implement-tool-use "ツール使用の実装方法"))

## 2) Plan small, execute in parallel

* Claude’s tool docs encourage parallel tool calls and concise definitions. You get latency wins by compiling a tiny DAG and running independent tools concurrently, then joining. This is the behavior users perceive as “fast and fluid.” ([Claude Docs](https://docs.claude.com/ja/docs/agents-and-tools/tool-use/implement-tool-use "ツール使用の実装方法"))

**Replicate:** in your micro-planner, return a compact JSON plan with `steps[]` and `after[]`, execute siblings in parallel, and only replan the failed subgraph.

## 3) Clarify before acting when inputs are missing

* Claude’s guidance: stronger models handle ambiguity and “ask to clarify when needed.” Encode this as a gate: if required fields are missing, ask one targeted question before any tool call, then resume. ([Claude Docs](https://docs.claude.com/ja/docs/agents-and-tools/tool-use/implement-tool-use "ツール使用の実装方法"))

**Replicate:** make “clarify” an explicit branch in your graph runtime rather than hoping the model self-asks mid-trace.

## 4) Stream progress; keep transports suited to the job

* Streaming output makes turns *feel* instant while tools run. On the wire, MCP supports stdio, Streamable HTTP, and SSE. Pick stdio for local dev tools, streamable HTTP or SSE for remote servers and live events. ([LangChain Docs](https://docs.langchain.com/oss/python/langchain/mcp "Model Context Protocol (MCP) - Docs by LangChain"))

**Replicate:** stream the planner ack, stream node starts/finishes, then stream the final synthesis.

## 5) Reduce token overhead instead of “thinking more”

* Anthropic shows an alternative to shoving everything through the context window: let agents **execute code** and pass results via MCP, which cuts tokens and scales to many tools. Keep plans small, move work into tools/code. ([Anthropic](https://www.anthropic.com/engineering/code-execution-with-mcp "Code execution with MCP: building more efficient AI agents"))

## 6) Lower perceived friction with one-click MCP packaging

* Claude Desktop’s **Desktop Extensions** install MCP servers with one click. Users see “it just works,” because the host handles the plumbing. Copy the idea: package your server with a clean manifest and minimal post-install steps. ([Anthropic](https://www.anthropic.com/engineering/desktop-extensions "One-click MCP server installation for Claude Desktop"))

## 7) Stay aligned with the evolving MCP spec

* MCP is versioned by date and aims for backward-compatible evolution. Use the official spec and schemas, not ad-hoc adapters. This keeps your tool catalog future-proof and portable across hosts. ([Model Context Protocol](https://modelcontextprotocol.io/specification/versioning "Versioning"))

## 8) Performance hygiene that matches Claude’s “feel”

* Cache shared prompt prefixes in your serving layer and keep tool catalogs stable to maximize reuse. Anthropic’s developer hub highlights prompt-caching to cut cost/latency; the same idea applies with your OSS server. ([Anthropic](https://www.anthropic.com/learn/build-with-claude "Anthropic Academy: Claude API Development Guide"))
* Keep your tool list per turn minimal. Shorter catalogs improve selection and reduce retries. Tie this to your router so only relevant tools are exposed each turn. ([Claude Docs](https://docs.claude.com/ja/docs/agents-and-tools/tool-use/implement-tool-use "ツール使用の実装方法"))

---

# Security for remote MCP tools

Use protocol-level guidance plus LLM-app security norms. Treat remote MCP as an attack surface, not just a transport.

## 1) Start with the official MCP security playbook

* Follow the **MCP Security Best Practices**. It documents concrete threats and mitigations: confused-deputy OAuth flows, “token passthrough” anti-pattern, session hijacking, and scope minimization. Apply these verbatim. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))
* Use the **MCP Authorization** spec when you expose HTTP transports. It defines how an MCP server advertises protected resource metadata and how clients obtain tokens. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/authorization "Authorization"))

**Non-negotiables from the spec:**

* Do **not** forward client tokens to downstream APIs (“token passthrough”). Issue and validate tokens explicitly for your MCP server. Keep audience separation. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))
* Implement **per-client consent** to block confused-deputy attacks when your MCP server proxies to third-party OAuth providers. Enforce exact redirect-URI matches and single-use state. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))
* Prefer **stdio** for local tools. If you must expose HTTP, require auth, consider UNIX domain sockets, and lock network scope. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))

## 2) Harden transports and infrastructure

* **Choose transports deliberately.** stdio for local, streamable HTTP or SSE for remote. Documented across multiple ecosystems to avoid misconfiguration traps. ([LangChain Docs](https://docs.langchain.com/oss/python/langchain/mcp "Model Context Protocol (MCP) - Docs by LangChain"))
* **SSE and load balancing.** Sticky routing is required. Route all `/mcp*` SSE traffic to a single replica. Otherwise streams break or cross-talk. Also disable proxy buffering and gzip on the SSE endpoint. Concrete guidance is available. ([n8n Docs](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-langchain.mcptrigger/ "MCP Server Trigger node documentation | n8n Docs"))
* **Reverse proxy settings.** For SSE, ensure HTTP/1.1, `proxy_buffering off`, and no chunked encoding at the proxy, or events stall. ([n8n Docs](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-langchain.mcptrigger/ "MCP Server Trigger node documentation | n8n Docs"))

## 3) Threat model like an LLM application, not just an API

* Apply **OWASP Top-10 for LLM Apps**: Prompt Injection (LLM01), Insecure Output Handling (LLM02), Model DoS (LLM04), and supply-chain risks. Build allow-lists, sanitize tool inputs, and validate tool outputs before re-prompting or executing anything. ([OWASP](https://owasp.org/www-project-top-10-for-large-language-model-applications/ "OWASP Top 10 for Large Language Model Applications"))
* MCP-specific **prompt-injection guidance** exists. Treat remote content and tool results as untrusted; isolate context; use content-policy checks and explicit allow-lists for dangerous tool classes. ([Microsoft Developer](https://developer.microsoft.com/blog/protecting-against-indirect-injection-attacks-mcp "Protecting against indirect prompt injection attacks in MCP"))

## 4) Identity, scopes, and consent

* Many platforms now expose reference implementations for **MCP server authorization**. Use an identity provider, register scopes for the **server itself**, and publish protected-resource metadata so clients discover the right auth endpoint. This avoids DIY auth drift. ([Microsoft Learn](https://learn.microsoft.com/en-us/azure/app-service/configure-authentication-mcp "Configure MCP server authorization - Azure App Service"))
* **Minimize scopes.** Only grant per-tool scopes that the plan needs. Rotate and revoke cleanly. The spec calls out over-broad scopes as a latent lateral-movement risk. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))

## 5) Secure one-click/server install flows

* One-click installation is powerful but dangerous. The security guide requires explicit **pre-execution consent dialogs** that show the exact command and demand approval before running on the user’s machine. Build this into your host. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))
* Ship signed packages and verify hashes before execution. Prefer stdio over open HTTP ports for local servers, per the guidance. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))

## 6) Operational controls you should enable

* **Per-tool allow-lists** and least-privilege credentials. Keep secrets out of prompts and tool outputs. Map each tool to a distinct credential context. ([OWASP](https://owasp.org/www-project-top-10-for-large-language-model-applications/ "OWASP Top 10 for Large Language Model Applications"))
* **Runtime policy**: block high-risk actions without user confirmation; throttle tool call rates to cap Model DoS; log every call with inputs, outputs, and identity. ([OWASP](https://owasp.org/www-project-top-10-for-large-language-model-applications/ "OWASP Top 10 for Large Language Model Applications"))
* **Transport observability**: monitor SSE/streamable HTTP health, detect broken subscriptions, and alert on session reuse anomalies consistent with hijack attempts. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/security_best_practices "Security Best Practices - Model Context Protocol"))

## 7) Documentation to keep handy

* MCP overview/spec and versioning: what “2025-06-18” means and how to track changes. ([Model Context Protocol](https://modelcontextprotocol.io/ "What is the Model Context Protocol (MCP)? - Model Context ..."))
* Claude tool-use and implementation guides, including parallel tool execution patterns. ([Claude Docs](https://docs.claude.com/en/api/messages "Messages"))
* Engineering posts on **code execution with MCP** to reduce tokens and scale to many tools. ([Anthropic](https://www.anthropic.com/engineering/code-execution-with-mcp "Code execution with MCP: building more efficient AI agents"))
* Infrastructure notes for MCP servers behind proxies or on multi-replica gateways. ([n8n Docs](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-langchain.mcptrigger/ "MCP Server Trigger node documentation | n8n Docs"))
* LLM-app security baselines (OWASP LLM Top-10). ([OWASP](https://owasp.org/www-project-top-10-for-large-language-model-applications/ "OWASP Top 10 for Large Language Model Applications"))

---

## Quick implementation checklist

**Copy from Claude**

* Strict tool schemas → enforce JSON on both sides. ([Claude Docs](https://docs.claude.com/en/api/messages "Messages"))
* Small plan → parallel tools → stream progress. ([Claude Docs](https://docs.claude.com/ja/docs/agents-and-tools/tool-use/implement-tool-use "ツール使用の実装方法"))
* Code execution or external tools instead of long context stuffing. ([Anthropic](https://www.anthropic.com/engineering/code-execution-with-mcp "Code execution with MCP: building more efficient AI agents"))
* Package MCP servers for one-click install where users live. ([Anthropic](https://www.anthropic.com/engineering/desktop-extensions "One-click MCP server installation for Claude Desktop"))

**Secure the surface**

* Adopt MCP Authorization; forbid token passthrough; implement per-client consent. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/authorization "Authorization"))
* Route `/mcp*` SSE to a single replica; disable buffering on the proxy. ([n8n Docs](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-langchain.mcptrigger/ "MCP Server Trigger node documentation | n8n Docs"))
* Apply OWASP LLM Top-10 controls; validate outputs before use. ([OWASP](https://owasp.org/www-project-top-10-for-large-language-model-applications/ "OWASP Top 10 for Large Language Model Applications"))

---

Add these options and safeguards to round out your MCP-based agent. Each item includes why it helps, how to do it, and concrete references.

# Architecture alternatives that actually move the needle

## 1) **Async function calling** to overlap “think→act”

* **Why**: You lose time waiting for the model to finish planning before tools start. Asynchronous calling overlaps generation with execution and can match or beat parallel sync calling.
* **How**: Stream tokens from the model, parse early tool calls, dispatch immediately, and keep generating. Resume with results as they arrive. This reduces critical-path latency for multi-tool turns.
* **Refs**: AsyncLM shows lower wall-clock via overlapped plan+exec and automatic parallelism without an explicit dependency graph. ([arXiv](https://arxiv.org/html/2412.07017v1 "Asynchronous LLM Function Calling"))

## 2) **Mixture-of-Agents (MoA) committee** for brittle turns only

* **Why**: A small committee can outvote a bad router or a weak planner on hard prompts. Use it selectively to avoid cost.
* **How**: Route “high-uncertainty” turns to a 2–3 agent committee (same base model, different seeds/system prompts). Aggregate with a simple rank-and-select or a verifier model.
* **Refs**: MoA improves complex reasoning and planning; open-source repo and paper detail layered agent voting. ([arXiv](https://arxiv.org/abs/2406.04692 "Mixture-of-Agents Enhances Large Language Model Capabilities"))

## 3) **Retrieval-Augmented Planning (RAP)** instead of pure “plan from scratch”

* **Why**: Plans degrade when tools or APIs have quirks. Retrieve prior successful plans or snippets from an execution log and adapt them.
* **How**: Index past DAGs, tool argument patterns, and error fixes; retrieve by user intent + tool set; let the planner “edit” a close match.
* **Refs**: Retrieval-augmented planners for agents on web/embodied tasks show better reliability vs. vanilla ReAct, and reduced hallucinated arguments. ([ACL Anthology](https://aclanthology.org/2024.findings-acl.802.pdf "RaDA: Retrieval-augmented Web Agent Planning with LLMs"))

## 4) **Structured-output–only invocation** (skip classic function calling)

* **Why**: A single structured JSON with `tool_batch[]` can be faster and easier to validate than function-calling tokens interleaved with prose.
* **How**: Constrain the model to emit one JSON that lists parallelizable tool calls and their args. Then your executor fires them concurrently and synthesizes results.
* **Refs**: vLLM structured outputs support JSON-schema enforcement; practitioners use structured outputs as a full replacement for function calling. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))

## 5) **Durable orchestration outside the LLM**

* **Why**: Long or fragile tasks should not block the chat loop.
* **How**: Push slow subgraphs to a durable orchestrator (Temporal/Dagster/Prefect). Chat stays snappy; long steps resume after failures, with retries and concurrency control.
* **Refs**: Temporal gives durable execution, retries, signals, and timers; Dagster/Prefect provide dynamic graphs, concurrency limits, and scheduling. ([Temporal](https://temporal.io/ "Temporal: Durable Execution Solutions"))

## 6) **Typed agent graphs (Pydantic-AI)**

* **Why**: Invalid plans/args waste tokens and time.
* **How**: Define planner and executor I/O as Pydantic models; validate on every hop; stream validated structured output.
* **Refs**: Pydantic-AI supports streamed structured outputs and graph composition with type hints. ([Pydantic AI](https://ai.pydantic.dev/ "Pydantic AI"))

## 7) **MCP ecosystem as glue code**

* **Why**: Many automations already exist.
* **How**: Expose existing workflows as MCP servers. n8n provides MCP Server/Client nodes; mind SSE routing and sticky sessions in clustered setups.
* **Refs**: n8n MCP nodes and known client/server connection issues; MCP HTTP/SSE transport guidance. ([n8n Docs](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-langchain.mcptrigger/ "MCP Server Trigger node documentation"))

# Latency and reliability tactics that stack with your current design

## A) **Exploit server-side optimizations**

* **SGLang**: RadixAttention for aggressive KV reuse, cache-aware load-balancing, speculative decoding, continuous batching. Good when many turns share tool catalogs and system prompts. ([GitHub](https://github.com/sgl-project/sglang "SGLang is a fast serving framework for large language ..."))
* **vLLM**: Tool calling + structured outputs + (as of mid-2025) low-overhead schema enforcement; combine with continuous batching. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/tool_calling.html "Tool Calling - vLLM"))

## B) **Minimize tool catalog per turn**

* **Why**: Smaller tool registry cuts token footprint and mis-selection.
* **How**: Router emits an allow-list of N relevant tools; only those are exposed to the planner/executor.

## C) **Argument canonicalization + tool I/O caching**

* **Why**: Many user requests map to the same normalized tool arguments.
* **How**: Canonicalize and cache `(tool, normalized_args)` with short TTL and user scope.

## D) **Async plan streaming**

* **Why**: Perceived speed matters.
* **How**: Stream a minimal “plan ack,” then per-node start/finish, then final synthesis. LangGraph’s `interrupt()` + checkpoints make clarify-and-resume natural. ([LangChain Docs](https://docs.langchain.com/oss/python/langgraph/interrupts "Interrupts - Docs by LangChain"))

# Safety and governance additions

## Guardrails at two layers

* **Structured validation**: enforce schemas with Guardrails or NeMo Guardrails, not just prompts. ([guardrails](https://www.guardrailsai.com/docs/how_to_guides/structured_data_with_guardrails "Generate structured data | Your Enterprise AI needs ..."))
* **Policy rails**: block tool classes without consent, redact PII in traces, and require user approval for high-risk actions. OWASP LLM Top-10 maps the main risks. ([OWASP](https://owasp.org/www-project-top-10-for-large-language-model-applications/ "OWASP Top 10 for Large Language Model Applications"))

## MCP-specific security

* **Follow MCP Authorization + Security Best Practices** when using HTTP/SSE transports; avoid token-passthrough and verify `Origin` headers to prevent DNS rebinding; prefer `stdio` for local servers. ([Model Context Protocol](https://modelcontextprotocol.io/specification/draft/basic/authorization "Authorization"))

# Observability, evals, and rollouts

* **Tracing**: Use LangSmith or Arize Phoenix to trace every node and tool call; both integrate with OpenTelemetry. Weave is a light option for agent runs. ([LangChain Docs](https://docs.langchain.com/langsmith/home "LangSmith docs - Docs by LangChain"))
* **Evals**: For agentic tasks, use GAIA for assistant skills and ST-WebAgentBench for safety-aware web actions. Track success, side-effects, and repetition, not just “task done.” ([proceedings.iclr.cc](https://proceedings.iclr.cc/paper_files/paper/2024/file/25ae35b5b1738d80f1f03a8713e405ec-Paper-Conference.pdf "A BENCHMARK FOR GENERAL AI ASSISTANTS"))
* **LLM-as-a-judge checks**: See AgentRewardBench for judging trajectories; add a small verifier pass only on uncertain turns. ([arXiv](https://arxiv.org/abs/2504.08942 "AgentRewardBench: Evaluating Automatic Evaluations of Web Agent Trajectories"))

# Quick “drop-ins” you can test this week

1. **Async function calling** on top of your current planner

   * Start streaming, parse first function call, dispatch, keep generating; merge results when futures resolve. Use your existing executor. ([arXiv](https://arxiv.org/html/2412.07017v1 "Asynchronous LLM Function Calling"))

2. **Structured-output–only batch**

   * Ask the model for:

   ```json
   {"tool_batch":[
     {"tool":"search","args":{"q":"..."}},
     {"tool":"lookup","args":{"id":"$search.id"}}
   ]}
   ```

   * Validate against a JSON Schema once. Fire siblings in parallel. vLLM structured outputs support this pattern. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/structured_outputs.html "Structured Outputs - vLLM"))

3. **RAP memory**

   * Log successful DAGs and failed-arg patches. At plan time, retrieve the top-k similar DAGs and let the planner edit, not invent. ([ACL Anthology](https://aclanthology.org/2024.findings-acl.802.pdf "RaDA: Retrieval-augmented Web Agent Planning with LLMs"))

4. **Durable offload**

   * Send long subgraphs to Temporal; return a streaming status message in chat and poll for completion to synthesize the final answer. ([Temporal](https://temporal.io/ "Temporal: Durable Execution Solutions"))

# Curated references for deeper dives

**Async & parallel orchestration**

* *An LLM Compiler for Parallel Function Calling* (paper + code). Parallel DAG planning and execution. ([arXiv](https://arxiv.org/pdf/2312.04511 "An LLM Compiler for Parallel Function Calling"))
* *Asynchronous LLM Function Calling (AsyncLM)*. Overlap generation with execution. ([arXiv](https://arxiv.org/html/2412.07017v1 "Asynchronous LLM Function Calling"))

**Ensembling / committees**

* *Mixture-of-Agents* (paper + repo). Layered agent voting surpassing single-model baselines. ([arXiv](https://arxiv.org/abs/2406.04692 "Mixture-of-Agents Enhances Large Language Model Capabilities"))

**Retrieval-Augmented Planning**

* *RaDA/ExRAP* style works for web/embodied agents; adapt plans using retrieved know-how. ([ACL Anthology](https://aclanthology.org/2024.findings-acl.802.pdf "RaDA: Retrieval-augmented Web Agent Planning with LLMs"))

**Serving and structure**

* vLLM tool calling + structured outputs; SGLang RadixAttention and cache-aware scheduling. ([VLLM Docs](https://docs.vllm.ai/en/stable/features/tool_calling.html "Tool Calling - vLLM"))

**Guardrails**

* Guardrails AI and NVIDIA NeMo Guardrails for typed outputs and runtime policies; OWASP LLM Top-10 2025. ([GitHub](https://github.com/guardrails-ai/guardrails "Adding guardrails to large language models."))

**Durable orchestration**

* Temporal (durable execution), Dagster (dynamic graphs), Prefect (work-pool concurrency). ([GitHub](https://github.com/temporalio/temporal "temporalio/temporal: Temporal service"))

**Observability and evals**

* LangSmith docs; Arize Phoenix OSS; W&B Weave resources; GAIA and ST-WebAgentBench. ([LangChain Docs](https://docs.langchain.com/langsmith/home "LangSmith docs - Docs by LangChain"))

---
