# elephant.ai — Agent Contract

Minimize mandatory reading. Expand context only when triggered.

## 0. Read Policy

1. Read **§1** on every task.
2. Read **§2** only when a trigger matches.
3. No trigger? Follow **§3** and stop expanding context.

---

## 1. Mandatory Core

### 1.1 Identity and priority
- Greet **ckl** at conversation start.
- Priority: **safety > correctness > maintainability > speed**.
- User: senior backend/database engineer; values deep reasoning, clean architecture.

### 1.2 Code style (non-negotiable)
- Max function body: **15 lines**. Extract or redesign if exceeded.
- No comments that restate code. Only "why" comments for non-obvious decisions.
- Prefer composition over inheritance. Prefer data transforms over mutation.
- Every abstraction must justify itself: if used <2 places, inline it.
- Delete dead code immediately. No TODOs in committed code.
- Type signatures are documentation. Verbose names > comments.
- Between two correct approaches, pick the one with fewer moving parts.
- Trust type/caller invariants; no unnecessary defensive code.
- No compatibility shims when requirements change; redesign cleanly.
- Modify only relevant files.
- Config examples must be YAML.

Reference density:
```ts
const authenticate = (token: string, secret: string): Result<Claims, AuthError> =>
  pipe(
    decode(token),
    chain(verify(secret)),
    mapErr(toAuthError)
  );
```
No wrapper classes. No builders. No config objects. Transform pipeline.

### 1.3 Branch safety
- Use worktree for code changes; never edit directly on `main`.
- On `main`, run before any edit:
  1. `git diff --stat` + `git log --oneline -10`
  2. If suspicious diffs: report to ckl before continuing.

### 1.4 Delivery
- Prefer TDD for logic changes; cover edge cases.
- Run lint + tests before delivery.
- Code review before commit: `python3 skills/code-review/run.py review`.
- Fix P0/P1 before commit; follow-up for P2.
- Small, scoped commits. Warn before destructive ops.

---

## 2. Progressive Disclosure (trigger-gated)

| Trigger | Load |
|---|---|
| Non-trivial staged execution | `docs/guides/engineering-workflow.md` Planning; create `docs/plans/*` |
| Proactive behavior (`internal/agent/`, triggers, context) | Proactive constraints in engineering-workflow |
| Architecture boundaries (`internal/**`) | Architecture rules + `configs/arch/*` |
| Memory/history retrieval | `docs/guides/memory-management.md`; summaries first |
| Safety incident / regression | `docs/postmortems/templates/incident-postmortem-template.md` |
| Large mechanical edits | Codex worker: explore → plan → execute → review, max 2 retries |
| User correction | Codify preventive rule before resuming |
| Worktree merge | §4 |

No trigger → do not load.

---

## 3. Default Route

1. Read §1 only.
2. Inspect target files and neighboring patterns.
3. Implement minimal correct change.
4. Proportionate verification (scoped tests/lint).
5. Commit and report.

---

## 4. Worktree Lifecycle

```bash
git worktree add -b <branch> ../<dir> main
cp .env ../<dir>/
```
Mark `<worktree>/.worktree-active.yaml` → `status: in_progress`.

Finish: `git checkout main && git merge --ff-only <branch>` → update marker to `merged` → `git worktree remove ../<dir>`.

---

## 5. Project Snapshot

- Product: proactive AI assistant (Lark, WeChat, CLI, Web).
- Layers: Delivery → Application → Domain (ReAct/events/approvals) → Infra adapters.
- Key dirs: `internal/agent/`, `internal/llm/`, `internal/memory/`, `internal/context/`, `internal/rag/`, `internal/delivery/channels/`, `web/`

---

## 6. Detail Sources (trigger-gated only)

`docs/guides/engineering-workflow.md` · `docs/guides/code-simplification.md` · `docs/guides/code-review-guide.md` · `docs/guides/memory-management.md` · `docs/postmortems/**`
