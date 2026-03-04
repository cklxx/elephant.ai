# elephant.ai — Proactive AI Assistant

## STOP — Read this first

* You are assisting **ckl**. Every conversation opening, first greet "ckl".
* Before ANY code change, run the **pre-work checklist** (§0.5). No exceptions.
* Conflict priority: **safety > correctness > maintainability > speed**.

---

## Coding standards (enforce on every change)

* Avoid unnecessary defensive code; trust invariants guaranteed by types or caller contracts.
* Never write compatibility logic; refactor from first principles, redesign cleanly.
* If intelligent automation can reliably handle it, don't introduce complex orchestration.
* Aim to get it right the first time — no shallow answers or needless clarification.
* Delete dead code completely. Never leave commented-out code, `_unused` prefixes, or `// deprecated` markers.

**DO / DON'T quick reference:**

| DO | DON'T |
|---|---|
| Delete unused code outright | Add `// deprecated`, `_unused` prefix, or re-export stubs |
| Return error directly when caller contract guarantees safety | Wrap every internal call in `if err != nil` defensively |
| Redesign the interface when requirements change | Add adapter shims or backward-compat wrappers |
| Write a plain `for` loop for 3 similar operations | Extract a premature generic helper |
| Fix root cause of a test failure | Add `t.Skip()`, retry loops, or `//nolint` |
| One struct/interface per responsibility | God-struct with 10+ fields "for convenience" |
| Use existing project patterns (check neighboring files) | Invent a new pattern without surveying the codebase |
| Modify only files relevant to the task | Touch unrelated files for "cleanup" or "while I'm here" |

---

## §0 · About the user

* Seasoned backend/database engineer; fluent in Rust, Go, Python.
* Values "Slow is Fast": reasoning quality, abstraction/architecture, long-term maintainability over short-term speed.
* Config files are YAML-only; avoid JSON config examples and assume `.yaml` paths.

## §0.5 · Pre-work checklist (mandatory before any code change on main)

> **Scope**: Only when working directly on the `main` branch. Skip in worktrees — all changes there are your own.

1. `git diff --stat` — see what files are currently modified.
2. `git log --oneline -10` — review recent commit history.
3. Identify changes that **should not have been made**: unrelated files touched, intentional logic reverted, needed code deleted, style regressions.
4. If suspicious changes exist → flag to ckl before proceeding. Never silently overwrite or build on top of incorrect diffs.
5. Applies to every new task, including mid-conversation topic switches.

---

## §1 · Planning & execution

### 1.1 Decision priorities
1. Hard constraints and explicit rules.
2. Reversibility / order of operations.
3. Missing info only if it changes correctness.
4. User preferences within constraints.

### 1.2 Process rules

**Planning**
* Plan for complex tasks (options + trade-offs); otherwise implement directly.
* Every plan → file under `docs/plans/`, updated as work progresses.
* Before each task, review engineering practices under `docs/guides/`; if missing, search and add them.

**Worktree workflow** (single source — all other references point here)
1. `git worktree add -b <branch> ../<dir> main`
2. `cp .env ../<dir>/`
3. Develop in the worktree.
4. `git checkout main && git merge --ff-only <branch>`
5. `git worktree remove ../<dir>` (optionally delete the branch).

**Testing & quality**
* Use TDD when touching logic; cover edge cases.
* Run full lint + tests before delivery.
* After lint/test pass, execute code review (`skills/code-review/SKILL.md`) on the diff. Fix P0/P1 before commit; follow-up for P2.

**Commits**
* Always commit after completing changes. One solution → multiple incremental commits.

**Service restart**
* Restart only the changed service: `alex dev restart backend` or `alex dev restart web`. Never `./dev.sh down && ./dev.sh`.

**Compounding engineering**
* Record successful paths and failed experiences — absolute core.
* Error/win entries: `docs/error-experience/entries/` and `docs/good-experience/entries/`.
* Summaries: `.../summary/entries/`.
* Index files (`docs/error-experience.md`, `docs/error-experience/summary.md`) are index-only — never put content there.
* Incident postmortems live under `docs/postmortems/` (`incidents/`, `templates/`, `checklists/`).
* For any high-impact regression, cross-channel leakage, or prompt/context safety incident: create an incident file in `docs/postmortems/incidents/` within one working day using `templates/incident-postmortem-template.md`.
* Every postmortem must include exact-date timeline, technical+process root cause, and prevention actions with owner/due-date/validation evidence.
* Closing an incident requires completing `docs/postmortems/checklists/incident-prevention-checklist.md` and linking matching error/good experience entries.

### 1.3 Safety & tooling
* Warn before destructive actions; avoid history rewrites unless explicitly requested.
* Prefer local registry sources for Rust deps.
* Keep responses focused on actionable outputs (changes + validation + limitations).
* Other agents may make changes; only commit your own code, fix conflicts, never roll back others' code.
* Keep `agent/ports` free of memory/RAG deps to avoid import cycles.

---

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

- **Context engineering over prompt hacking** → Modify context assembly (`internal/context/`) first. Prompt templates only if context changes are verified insufficient.
- **Typed events over unstructured logs** → Use typed event structs (`internal/agent/domain/events/`). No free-form log strings for state transitions.
- **Clean port/adapter boundaries** → Cross-layer imports go through port interfaces. Direct infra-to-domain imports forbidden.
- **Multi-provider LLM support** → New LLM features must work across all providers in `internal/llm/`. No provider-specific APIs without adapter.
- **Skills and memory over one-shot answers** → Persist learnings to memory. Encode reusable workflows as skills.
- **Proactive context injection** → Auto-inject relevant context before user asks. Manual retrieval is fallback.
- **Global best practices over local conventions** → Reference industry standards and established open-source patterns.

### Proactive behavior constraints

When modifying proactive behavior code (`internal/agent/`, skill triggers, context injection):
- Detect motivation state before proactive actions: low energy, overload, ambiguity, or clear readiness.
- Minimum-effective intervention: `clarify` → `plan` → reminder/schedule/task execution.
- Every proactive suggestion must remain user-overridable; never remove opt-out paths.
- Prefer progress visibility (artifacts/checkpoints) over high-frequency nudges.

Safety constraints:
- No manipulative framing (fear, guilt, urgency). Applies to all LLM prompt construction.
- External messages or irreversible operations must pass approval gates (`internal/agent/domain/`). No exceptions.
- Honor stop signals immediately: disable reminders and proactive pushes.
- State uncertainty clearly; never fabricate confidence.

---

## Codex worker protocol (Brain + Hands)

Claude Code (Opus) = **planning brain**; Codex CLI = **execution hands**. Use autonomously for implementation — no user trigger required.

### When to use Codex

- Multi-file implementation (3+ files)
- Repetitive mechanical changes (rename, migrate, boilerplate)
- Clear plan needing fast parallel execution
- Test suites or build verification

Do NOT delegate: architectural decisions, code review judgment, ambiguous requirements.

### Invocation patterns

**Explore (read-only):**
```
codex exec --dangerously-bypass-approvals-and-sandbox --ephemeral -o /tmp/codex-explore-{n}.md -C <dir> "[READ-ONLY] <precise question>"
```

**Execute (write):**
```
codex exec --dangerously-bypass-approvals-and-sandbox -o /tmp/codex-exec-{id}.md -C <dir> "<self-contained prompt with all context>"
```

### Four-phase workflow

1. **EXPLORE** — Codex read-only probes + your own Read/Grep/Glob. Max 5 explore calls. Parallel when independent.
2. **PLAN** — You design architecture. Decompose into atomic tasks with deps, files, verify commands. Present to user.
3. **EXECUTE** — Dispatch to Codex. Every prompt self-contained (zero cross-call memory). End with verify command. Parallel when no deps.
4. **REVIEW** — Codex runs tests. You review `git diff`. Fix via targeted re-dispatch. Summarize when clean.

### Prompt rules

- Every prompt contains ALL needed context (paths, signatures, constraints, code snippets)
- Never reference "previous tasks" or pass conversation history
- End every prompt with: "After changes, run: `<verify command>`"
- Always use `-o` flag; always read the output file afterward

### Error handling

- Max 2 retries per task. Retry includes: original task + error output + your diagnosis.
- After 2 failures → escalate to user with attempts, errors, root cause, suggested fix.

### Constraints

- Architecture decisions are yours — Codex only executes your designs
- Default directory: project root. Use `-C` for subtree scope.
- No interactive flags — Codex runs headless
- Instruct Codex to follow project's existing code style

---

## Memory loading guidance

### Always-load set (~8 KB, every conversation start)
1. `docs/memory/long-term.md` — stable cross-session rules.
2. `docs/guides/engineering-practices.md` — coding conventions.
3. Latest 3 **error summaries** from `docs/error-experience/summary/entries/` (by filename date DESC).
4. Latest 3 **good summaries** from `docs/good-experience/summary/entries/` (by filename date DESC).

No ranking algorithm needed — filenames are date-sorted; just read the most recent.

### On-demand loading (load when the task needs it)

| Source | Trigger |
|--------|---------|
| Full error/good entry | Summary lacks detail for the current task |
| `docs/memory/index.yaml` + `edges.yaml` | Need to search history by topic or find related entries |
| `docs/memory/tags.yaml` | Need tag-based filtering |
| `docs/postmortems/incidents/` | Task touches a component with a known incident |
| `docs/plans/` | Entering planning phase; need prior design references |
| 1-hop graph expansion | Already found a relevant entry; need its neighbors |

### Retrieval rules
- Summaries first; expand to full entry only when summary is insufficient.
- Prefer most recent item when multiple entries cover the same topic.
- Lark tasks: `memory_search → memory_get → memory_related → lark_chat_history`.

### Authoring rules
- `docs/memory/long-term.md` = durable, long-lived lessons only. Update `Updated:` timestamp to hour precision (`YYYY-MM-DD HH:00`).
- New entries: include `## Metadata` with `links` for graph edges.
- After editing memory docs: `go run ./scripts/memory/backfill_networked.go`.

---

## Code review — mandatory before every commit

- **Trigger**: After lint + tests pass, before any commit or merge.
- **Entry point**: `python3 skills/code-review/run.py '{"action":"review"}'`.
- **Blocking rule**: P0/P1 must be fixed. P2 creates follow-up. P3 optional.

## §2 · Workflow preferences

* **Always use worktrees for code changes.** Never modify code directly on `main`. Use `EnterWorktree` (or `git worktree add`) to create an isolated branch, develop there, then merge back via `git merge --ff-only`.

---

## Additional agent rules

- Prefer run_tasks for parallelizable tasks.
- Understand full context of changes before reviewing; respect architectural decisions over personal preferences.
- **Self-correction rule:** Upon receiving ANY correction from the user, immediately write a preventive rule for yourself (in `docs/guides/`, `docs/error-experience/entries/`, or the relevant best-practice doc) to prevent the same class of mistake from recurring. Do not wait — codify the lesson before resuming work.
- **User-pattern learning & auto-continue rule:**
  1. **Record**: Save notable user decisions, preferences, and interaction patterns to `docs/memory/user-patterns.md` (e.g., "user always picks option A when asked X", "user says 'continue' after lint passes").
  2. **Analyze**: Before asking the user a question or pausing at a task boundary, review accumulated patterns to determine if the answer is predictable.
  3. **Auto-continue**: If prior patterns indicate a high-confidence answer (same decision made ≥2 times in similar context), skip the question and proceed automatically. State what was auto-decided and why in a brief inline note (e.g., "[auto: chose X based on prior pattern]").
  4. **Still ask when**: The decision is genuinely ambiguous, involves irreversible/destructive actions, or no matching pattern exists. Safety gates and approval gates are never bypassed.
  5. **At task end**: Check if the next logical step is obvious from context + patterns. If so, continue into it instead of stopping to ask. Announce what you're doing.
