# elephant.ai — Agent Contract (Lean)

Goal: minimize mandatory reading and enforce strict conditional progressive disclosure.

## 0. Read Policy (hard rule)

1. Read **Section 1** on every task (mandatory core only).
2. Read **Section 2** only when a trigger matches.
3. If no trigger matches, follow **Section 3 simplified route** and stop expanding context.
4. Never bulk-load all referenced docs by default.

---

## 1. Mandatory Core (always read)

### 1.1 Identity and priority
- First greeting in each conversation: `ckl`.
- Priority order: **safety > correctness > maintainability > speed**.
- User profile: senior backend/database engineer; prefers deep reasoning and clean architecture.

### 1.2 Non-negotiable coding standards
- Trust type/caller invariants; avoid unnecessary defensive code.
- No compatibility shims or adapter backfills when requirements change; redesign cleanly.
- Delete dead code outright. No `// deprecated`, `_unused`, or commented-out legacy blocks.
- Modify only relevant files.
- Config examples must be YAML (`.yaml`), not JSON.

### 1.3 Branch and pre-work safety
- Default: use worktree for code changes; do not edit directly on `main`.
- If and only if working on `main`, run before any edit:
  1. `git diff --stat`
  2. `git log --oneline -10`
  3. Check suspicious diffs (unrelated files, reverted intended logic, accidental deletions, style regressions).
  4. If suspicious: report to ckl before continuing.

### 1.4 Delivery baseline
- For logic changes, prefer TDD and include edge cases.
- Run lint + tests before delivery.
- Run code review before commit: `python3 skills/code-review/run.py review`.
- Fix P0/P1 before commit; create follow-up for P2.
- Commit after completing changes; prefer small incremental commits.
- Warn before destructive operations; avoid history rewrites unless explicitly requested.

---

## 2. Conditional Progressive Disclosure (read only if triggered)

| Trigger | Read / Apply |
|---|---|
| Task is non-trivial and needs staged execution | `docs/guides/engineering-workflow.md` Planning section; create/update `docs/plans/*` |
| Task touches proactive behavior (`internal/agent/`, triggers, context injection) | Proactive constraints in `docs/guides/engineering-workflow.md` |
| Task touches architecture boundaries (`internal/**`) | Architecture rules in `docs/guides/engineering-workflow.md` and `configs/arch/*` |
| Task needs memory/history retrieval | `docs/guides/memory-management.md`; load summaries first, then full entries only if insufficient |
| High-impact regression / leakage / safety incident | `docs/postmortems/templates/incident-postmortem-template.md` + checklist |
| Large multi-file or mechanical edits | Codex worker protocol (explore/plan/execute/review), self-contained prompts, max 2 retries |
| User gives correction | Immediately codify preventive rule in `docs/guides/` or `docs/error-experience/entries/` before continuing |
| Ready to merge worktree | Worktree lifecycle in Section 4 |

Rule: if trigger is not met, do not read that module.

---

## 3. Simplified Default Route (no trigger matched)

1. Read Section 1 only.
2. Inspect target files and neighboring patterns.
3. Implement minimal correct change.
4. Run proportionate verification (tests/lint relevant to scope).
5. Commit and report: changes + validation + known limits.

---

## 4. Worktree Lifecycle (compact)

1. `git worktree add -b <branch> ../<dir> main`
2. `cp .env ../<dir>/`
3. Create `<worktree>/.worktree-active.yaml`:
   ```yaml
   status: in_progress
   ```
4. Develop and commit in worktree.
5. Auto-merge when done: `git checkout main && git merge --ff-only <branch>`
6. Update marker to `status: merged`.
7. Remove worktree: `git worktree remove ../<dir>`
8. Push only from primary repo or managed worktrees sharing the same `.git`.

---

## 5. Project Snapshot (minimal)

- Product: proactive AI assistant across Lark, WeChat, CLI, Web.
- Architecture:
  - Delivery layer -> Application layer -> Domain (ReAct/events/approvals) -> Infra adapters.
- Key directories:
  - `internal/agent/`, `internal/llm/`, `internal/memory/`, `internal/context/`, `internal/rag/`
  - `internal/infra/tools/builtin/`, `internal/delivery/channels/`, `internal/infra/observability/`
  - `web/`

---

## 6. Source of Truth for Details

Use these only when triggered by Section 2:
- `docs/guides/engineering-workflow.md`
- `docs/guides/code-simplification.md`
- `docs/guides/code-review-guide.md`
- `docs/guides/memory-management.md`
- `docs/postmortems/**`
