# elephant.ai — Proactive AI Assistant

## Project identity

Proactive AI assistant across Lark, WeChat, CLI, and web. Persistent memory, autonomous ReAct execution, built-in skills, approval gates.

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
- `internal/infra/tools/builtin/` — File ops, shell, code exec, browser, media, search
- `internal/delivery/channels/` — Lark, WeChat integrations
- `internal/infra/observability/` — Traces, metrics, cost accounting
- `web/` — Next.js dashboard with SSE streaming

### Design preferences

- **Context engineering over prompt hacking** → When improving LLM output quality, modify context assembly logic (`internal/context/`) first. Only add prompt templates if context changes are verified insufficient.
- **Typed events over unstructured logs** → New observability must use typed event structs (`internal/agent/domain/events/`). Never add free-form log strings for state transitions.
- **Clean port/adapter boundaries over convenience shortcuts** → Cross-layer imports must go through port interfaces. Direct infra-to-domain imports are forbidden.
- **Multi-provider LLM support over vendor lock-in** → New LLM features must work across all providers in `internal/llm/`. Never use provider-specific APIs without a provider-agnostic adapter.
- **Skills and memory over one-shot answers** → When the assistant can learn from an interaction, persist it to memory. When a workflow is reusable, encode it as a skill.
- **Proactive context injection over user-driven retrieval** → Auto-inject relevant context before the user asks. Manual retrieval is a fallback, not the default.
- **Global best practices over local conventions** → Reference industry standards, academic research, and established open-source patterns when available.

### Proactive behavior constraints

When modifying code that governs proactive behavior (`internal/agent/`, skill triggers, context injection):
- Detect motivation state before executing proactive actions: low energy, overload, ambiguity, or clear readiness.
- Apply minimum-effective intervention: `clarify` → `plan` → reminder/schedule/task execution.
- Every proactive suggestion must remain user-overridable; never remove opt-out paths.
- Prefer progress visibility (artifacts/checkpoints) over high-frequency nudges.
- Use memory-guided personalization only when it materially improves relevance.

Safety constraints for proactive code paths:
- No manipulative framing (fear, guilt, urgency inflation). Applies to all LLM prompt construction.
- Any tool call that sends external messages or performs irreversible operations must pass through approval gates (`internal/agent/domain/`). No exceptions.
- Honor explicit stop signals immediately: disable reminders and proactive pushes.
- State uncertainty clearly; never fabricate confidence in LLM responses.

---

## Repo agent workflow & safety rules

### Conflict resolution (meta-rule)

When rules conflict, priority is: **safety > correctness > maintainability > speed**.
- If removing code would cause a runtime panic → keep it (safety).
- If two approaches are equally safe → pick the more correct one.
- If equally correct → pick the more maintainable one.
- "Avoid unnecessary defensive code" vs "cover edge cases" → if the invariant is guaranteed by the type system or caller contract, skip the guard; if it depends on external input or runtime state, add the check.

### 0 · About the user

* You are assisting **cklxx**. Address as cklxx first.
* Seasoned backend/database engineer; fluent in Rust, Go, Python.
* Values "Slow is Fast": reasoning quality, abstraction/architecture, long-term maintainability over short-term speed.
* Config files are YAML-only; avoid JSON config examples and assume `.yaml` paths.

### 1 · Planning & execution

#### 1.1 Decision priorities
1. Hard constraints and explicit rules.
2. Reversibility / order of operations.
3. Missing info only if it changes correctness.
4. User preferences within constraints.

#### 1.2 Process rules

**Planning**
* Plan for complex tasks (options + trade-offs); otherwise implement directly.
* Every plan → file under `docs/plans/`, updated as work progresses.
* Before each task, review engineering practices under `docs/guides/`; if missing, search and add them.
* Start with the most systematic view of the project, then propose a reasonable plan.

**Worktree workflow** (single source — all other references point here)
1. `git worktree add -b <branch> ../<dir> main`
2. `cp .env ../<dir>/`
3. Develop in the worktree.
4. `git checkout main && git merge --ff-only <branch>`
5. `git worktree remove ../<dir>` (optionally delete the branch).

**Testing & quality**
* Use TDD when touching logic; cover edge cases.
* Run full lint + tests before delivery.
* After lint/test pass, execute code review (`skills/code-review/SKILL.md`) on the diff. Fix P0/P1 before commit; create follow-up for P2.

**Commits**
* Always commit after completing changes. Split one solution into incremental batches — one solution, multiple commits.

**Service restart**
* After code changes, restart only the changed service: `alex dev restart backend` or `alex dev restart web`. Avoid `./dev.sh down && ./dev.sh`.

**Coding standards**
* Avoid unnecessary defensive code; trust invariants when guaranteed by types or caller contracts.
* Never write compatibility logic; refactor from first principles, redesign cleanly.
* If intelligent automation can reliably take over a solution, do not introduce a complex workflow orchestration scheme.
* Act as a strong reasoning and planning assistant — aim to get it right the first time, avoid shallow answers and needless clarification.
* Provide periodic summaries; abstract/refactor when appropriate for long-term maintainability.

**Compounding engineering**
* Record successful paths and failed experiences — this is the absolute core.
* Error/win entries: `docs/error-experience/entries/` and `docs/good-experience/entries/`.
* Summaries: `.../summary/entries/`.
* Index files (`docs/error-experience.md`, `docs/error-experience/summary.md`) are index-only — never put content there.
* Log important incidents in error-experience entries; record execution plans and progress in planning docs.

#### 1.3 Safety & tooling
* Warn before destructive actions; avoid history rewrites unless explicitly requested.
* Prefer local registry sources for Rust deps.
* Keep responses focused on actionable outputs (changes + validation + limitations).
* Other agent assistants may make changes; only commit your own code, fix conflicts, never roll back others' code.
* Keep `agent/ports` free of memory/RAG deps to avoid import cycles.

---

## Codex worker protocol (Brain + Hands)

Claude Code (Opus) is the **planning brain**; Codex CLI is the **execution hands**. Use this protocol autonomously for implementation tasks — no user trigger required.

### When to use Codex

- Multi-file implementation tasks (3+ files)
- Repetitive mechanical changes (rename, migrate, add boilerplate)
- Tasks where you already have a clear plan and need fast parallel execution
- Running test suites or build verification across the codebase

Do NOT delegate to Codex: architectural decisions, code review judgment, ambiguous requirements. Those stay with you.

### Codex invocation patterns

**Explore (read-only):**
```
codex exec --dangerously-bypass-approvals-and-sandbox --ephemeral -o /tmp/codex-explore-{n}.md -C <dir> "[READ-ONLY] <precise question>"
```

**Execute (write):**
```
codex exec --dangerously-bypass-approvals-and-sandbox -o /tmp/codex-exec-{id}.md -C <dir> "<self-contained prompt with all context>"
```

### Four-phase workflow

1. **EXPLORE** — Codex read-only probes + your own Read/Grep/Glob. Max 5 Codex explore calls. Parallel when independent.
2. **PLAN** — You design the architecture. Decompose into atomic tasks (T1, T2...) with deps, files, verification commands. Present to user.
3. **EXECUTE** — Dispatch tasks to Codex. Every prompt self-contained (zero cross-call memory). End each with verification command. Parallel when no deps.
4. **REVIEW** — Codex runs tests. You review `git diff`. Fix issues via targeted re-dispatch. Summarize when clean.

### Prompt rules for Codex calls

- Every prompt contains ALL needed context (paths, signatures, constraints, code snippets)
- Never reference "previous tasks" or pass conversation history
- End every execution prompt with: "After changes, run: `<verify command>`"
- Always use `-o` flag; always read the output file afterward

### Error handling

- Max 2 retries per task. Retry must include: original task + error output + your diagnosis.
- After 2 failures → escalate to user with attempts, errors, root cause, suggested fix.

### Constraints

- Architecture decisions are yours — Codex only executes your designs
- Default working directory: project root. Use `-C` for subtree scope.
- No interactive flags — Codex runs headless
- Instruct Codex to follow project's existing code style and patterns

---

## Memory loading guidance (first run + progressive disclosure)

### Memory sources
Use: error entries + summaries, good entries + summaries, memory-related plans under `docs/plans/`, and `docs/memory/long-term.md`.
Treat these as graph nodes backed by:
- `docs/memory/index.yaml`
- `docs/memory/edges.yaml`
- `docs/memory/tags.yaml`

### First-run memory load (mandatory)
On the first run in a repo session:
1. Read the latest 3–5 items from **each** of the four folders above.
2. Build a unified memory list and rank items by:
   - **Recency**: newer dates score higher.
   - **Frequency**: topics that repeat across entries score higher.
   - **Relevance**: lexical overlap with the current task and current files wins.
3. Keep only the top 8–12 items as the **active memory set**.
4. Expand 1-hop graph neighbors from `edges.yaml` for active nodes (max 6, relevance-ranked).
5. Store the remaining items as **cold memory** (not loaded unless requested).

### Progressive disclosure (on-demand)
Only expand memory beyond the active set when:
- The task touches a known failure/success pattern but lacks specifics.
- Tests fail with a known error signature.
- The user explicitly requests historical context or a postmortem.
- When authoring new entries, include `## Metadata` with `links` to enable graph edges (see `docs/memory/networked/README.md`).

### Retrieval rules
- Use summaries first; only open full entries if summaries are insufficient.
- Prefer the most recent item when multiple entries discuss the same topic.
- If two items are equally relevant, pick the one with higher recurrence across entries.
- For Lark tasks, retrieval order is: `memory_search -> memory_get -> memory_related -> lark_chat_history`.
- `memory_related` traverses only `related` edges (bidirectional). `see_also`/`supersedes`/`derived_from` remain directed cross-references.

### Long-term memory doc rules
- `docs/memory/long-term.md` stores only durable, long-lived lessons.
- Always update the `Updated:` timestamp to hour precision (`YYYY-MM-DD HH:00`).
- On the **first memory load each day**, re-rank memories (recency/frequency/relevance), refresh the active set, and update the long-term doc if needed.
- After editing memory docs, regenerate graph artifacts with `go run ./scripts/memory/backfill_networked.go`.

## Code review — mandatory before every commit

- **Trigger**: After lint + tests pass, before any commit or merge.
- **Entry point**: `python3 skills/code-review/run.py '{"action":"review"}'` (full workflow in `skills/code-review/SKILL.md`).
- **Blocking rule**: P0/P1 findings must be fixed before commit. P2 creates a follow-up task. P3 is optional.

## Additional agent rules

- Prefer using subagents for parallelizable tasks to improve execution speed.
- Understand the full context of changes before reviewing; respect architectural decisions over personal preferences.
