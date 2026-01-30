# elephant.ai — Proactive AI Assistant

## Project identity

elephant.ai is a **proactive AI assistant** that embeds into daily workflows — Lark, WeChat, CLI, and web — remembers context across conversations, takes initiative with built-in skills, and executes real work autonomously. One runtime, every surface, always ready.

### What "proactive" means for this codebase

| Principle | Implementation |
|---|---|
| **Anticipate, don't wait** | Auto-save Lark/WeChat messages to memory; inject recent chat history as context before the user asks. |
| **Channel-native** | The assistant is a first-class participant in Lark groups and WeChat chats, not a separate app the user must switch to. |
| **Persistent memory** | `internal/memory/` stores conversations, decisions, and context across sessions. The LLM retrieves relevant history automatically. |
| **Autonomous execution** | The ReAct loop (`internal/agent/domain/react/`) runs Think → Act → Observe to completion — web search, code execution, file ops, browser automation — without hand-holding. |
| **Built-in skills** | `skills/` contains markdown-driven workflows (deep research, meeting notes, email drafting, slide decks, video production) that the assistant executes on demand. |
| **Approval gates** | Proactivity requires safety. Risky tool calls require explicit human approval via CLI, web, or chat. |
| **Tools feel local** | In web mode, sandbox-backed tools (file read/write, shell exec, code execution, browser) present as local operations. The LLM should never perceive sandbox boundaries. |
| **Observable** | Every action produces structured traces, metrics, and cost accounting (`internal/observability/`). |

### Architecture

```
Delivery (CLI, Server, Web, Lark, WeChat)
  → Agent Application Layer (preparation, coordination, cost)
  → Domain (ReAct loop, events, approvals, context assembly)
  → Infrastructure Adapters (LLM, tools, memory, storage, observability)
```

Key packages:
- `internal/agent/` — ReAct loop, typed events, approval gates
- `internal/llm/` — Multi-provider (OpenAI, Claude, ARK, DeepSeek, Ollama)
- `internal/memory/` — Persistent store (Postgres, file, in-memory)
- `internal/context/`, `internal/rag/` — Layered retrieval and summarization
- `internal/tools/builtin/` — File ops, shell, code exec, browser, media, search
- `internal/channels/` — Lark, WeChat integrations
- `internal/observability/` — Traces, metrics, cost accounting
- `web/` — Next.js dashboard with SSE streaming

### Design preferences

When making decisions, prefer:
- Context engineering over prompt hacking.
- Typed events over unstructured logs.
- Clean port/adapter boundaries over convenience shortcuts.
- Multi-provider LLM support over vendor lock-in.
- Skills and memory over one-shot answers.
- Proactive context injection over user-driven retrieval.

---

## Repo agent workflow & safety rules

### 0 · About the user and your role

* You are assisting **cklxx**.
* Address me as cklxx first.
* Assume cklxx is a seasoned backend/database engineer familiar with Rust, Go, Python, and their ecosystems.
* cklxx values "Slow is Fast" and focuses on reasoning quality, abstraction/architecture, and long-term maintainability rather than short-term speed.
* **Most important:** Keep error experience entries in `docs/error-experience/entries/` and summary items in `docs/error-experience/summary/entries/`; `docs/error-experience.md` and `docs/error-experience/summary.md` are index-only.
* Config files are YAML-only; avoid JSON config examples and assume `.yaml` paths.
* Your core goals:
  * Act as a **strong reasoning and planning coding assistant**, giving high-quality solutions and implementations with minimal back-and-forth.
  * Aim to get it right the first time; avoid shallow answers and needless clarification.
  * Provide periodic summaries, and abstract/refactor when appropriate to improve long-term maintainability.
  * Start with the most systematic view of the current project, then propose a reasonable plan.
  * For any technical plan, run `claude -p` in a background task to discuss the plan with Claude ("claude -p 跟 cluade 讨论方案，且要采用后台任务") before executing.
  * Absolute core: practice compounding engineering — record successful paths and failed experiences.
  * Record execution plans, progress, and notable issues in planning docs; log important incidents in error-experience entries.
  * Every plan must be written to a file under `docs/plans/`, with detailed updates as work progresses.
  * Before executing each task, review best engineering practices under `docs/`; if missing, search and add them.
  * Run full lint and test validation after changes.
  * Any change must be fully tested before delivery; use TDD and cover edge cases as much as possible.
  * Avoid unnecessary defensive code; if context guarantees invariants, use direct access instead of `getattr` or guard clauses.

---

### 1 · Overall reasoning and planning framework (global rules)

Keep this concise and action-oriented. Prefer correctness and maintainability over speed.

#### 1.1 Decision priorities
1. Hard constraints and explicit rules.
2. Reversibility/order of operations.
3. Missing info only if it changes correctness.
4. User preferences within constraints.

#### 1.2 Planning & execution
* Plan for complex tasks (options + trade-offs), otherwise implement directly.
* Every plan must be a file under `docs/plans/` and updated as work progresses.
* Before each task, review engineering practices under `docs/`; if missing, search and add them.
* Record notable incidents in error-experience entries; keep index files index-only.
* Use TDD when touching logic; run full lint + tests before delivery.
* After completing changes, always commit. Split one solution into incremental batches and commit each batch separately — one solution, multiple commits.
* Avoid unnecessary defensive code; trust invariants when guaranteed.

#### 1.3 Safety & tooling
* Warn before destructive actions; avoid history rewrites unless explicitly requested.
* Prefer local registry sources for Rust deps.
* Keep responses focused on actionable outputs (changes + validation + limitations).
* I may ask other agent assistants to make changes; you should only commit your own code, fix conflicts, and never roll back code.
* Never write compatibility logic; always refactor from first principles, redesign the architecture, and implement cleanly.

---

## Error experience index

- Index: `docs/error-experience.md`
- Summary index: `docs/error-experience/summary.md`
- Summary entries: `docs/error-experience/summary/entries/`
- Entries: `docs/error-experience/entries/`

---

## Memory loading guidance (first run + progressive disclosure)

### Memory sources
Use: error entries + summaries, good entries + summaries, and `docs/memory/long-term.md`.

### First-run memory load (mandatory)
On the first run in a repo session:
1. Read the latest 3–5 items from **each** of the four folders above.
2. Build a unified memory list and rank items by:
   - **Recency**: newer dates score higher.
   - **Frequency**: topics that repeat across entries score higher.
   - **Relevance**: lexical overlap with the current task and current files wins.
3. Keep only the top 8–12 items as the **active memory set**.
4. Store the remaining items as **cold memory** (not loaded unless requested).

### Progressive disclosure (on-demand)
Only expand memory beyond the active set when:
- The task touches a known failure/success pattern but lacks specifics.
- Tests fail with a known error signature.
- The user explicitly requests historical context or a postmortem.

### Retrieval rules
- Use summaries first; only open full entries if summaries are insufficient.
- Prefer the most recent item when multiple entries discuss the same topic.
- If two items are equally relevant, pick the one with higher recurrence across entries.

### Long-term memory doc rules
- `docs/memory/long-term.md` stores only durable, long-lived lessons.
- Always update the `Updated:` timestamp to hour precision (`YYYY-MM-DD HH:00`).
- On the **first memory load each day**, re-rank memories (recency/frequency/relevance), refresh the active set, and update the long-term doc if needed.
