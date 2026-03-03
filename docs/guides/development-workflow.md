# Development Workflow

Updated: 2026-03-03

The end-to-end development cycle for elephant.ai. Every contributor (human and AI agent) follows this flow.

---

## 1. Session Start

1. Greet **ckl**.
2. Load the [always-load memory set](memory-management.md#always-load-set).
3. On the `main` branch, execute the **pre-work checklist** before any code change:
   - `git diff --stat` — see what files are currently modified.
   - `git log --oneline -10` — review recent commit history.
   - Identify changes that should not have been made: unrelated files touched, intentional logic reverted, needed code deleted, style regressions.
   - If suspicious changes exist → flag to ckl before proceeding. Never silently overwrite or build on top of incorrect diffs.
   - Applies to every new task, including mid-conversation topic switches.

Skip the pre-work checklist only when inside a worktree (all changes there are your own).

---

## 2. Decision Priorities

When conflicts arise, resolve in this order:

1. **Safety** > correctness > maintainability > speed.
2. Hard constraints and explicit rules.
3. Reversibility / order of operations.
4. Missing info only if it changes correctness.
5. User preferences within constraints.

---

## 3. Planning

- **Simple tasks**: implement directly.
- **Non-trivial tasks**: write a plan first under `docs/plans/YYYY-MM-DD-short-slug.md`.
- Before each task, review engineering practices under `docs/guides/`; if guidance is missing, search industry sources and add it.
- Update plan files as work progresses — not just at creation.

---

## 4. Worktree Workflow

Use worktrees for isolation when working on non-trivial changes.

```bash
git worktree add -b <branch> ../<dir> main
cp .env ../<dir>/
# develop in the worktree
git checkout main && git merge --ff-only <branch>
git worktree remove ../<dir>
```

If `git branch -d` is denied by policy: `git update-ref -d refs/heads/<branch>` then `git worktree prune`; verify with `git branch --list '<branch>'`.

---

## 5. Implementation

Follow the [Engineering Practices](engineering-practices.md) and [Code Simplification Rules](code-simplification.md).

Key principles:
- Trust invariants guaranteed by types or caller contracts — no unnecessary defensive code.
- Delete dead code completely — no `// deprecated`, `_unused` prefixes, or commented-out code.
- Redesign interfaces when requirements change — no adapter shims or backward-compat wrappers.
- Use existing project patterns — survey neighboring files before inventing new patterns.
- Only modify files relevant to the task.

---

## 6. Testing

- Use **TDD** when touching logic; cover edge cases.
- Run full lint + tests before delivery:
  ```bash
  alex dev lint
  alex dev test
  ```
- On macOS, prefer `CGO_ENABLED=0` for `go test -race` to avoid LC_DYSYMTAB linker warnings.

---

## 7. Code Review

**Mandatory before every commit.** After lint + tests pass:

```bash
python3 skills/code-review/run.py '{"action":"review"}'
```

See [Code Review Guide](code-review-guide.md) for full process.

Blocking rules:
- **P0/P1**: must fix before commit.
- **P2**: create follow-up task.
- **P3**: optional.

---

## 8. Commit

- Always commit after completing changes.
- One solution → multiple **incremental commits** (prefer small, reviewable commits).
- Only commit your own code. Fix conflicts, but never roll back others' code.

---

## 9. Service Restart

Restart only the changed service — never full stack:

```bash
alex dev restart backend   # backend changes
alex dev restart web       # web changes
```

Never `./dev.sh down && ./dev.sh`.

For kernel validation: `go run ./cmd/alex-server kernel-once`.

---

## 10. Pre-Push Gate

`scripts/pre-push.sh` mirrors CI fast-fail checks. Always runs before `git push`. Skip only with `SKIP_PRE_PUSH=1`.

Note: parallel checks on large diffs may produce `-race` flakes and lint timeouts. Confirm with a targeted re-run before treating as a real regression.

---

## 11. Experience Records

After completing work, record notable outcomes:

| Type | Entry path | Summary path |
|------|-----------|--------------|
| Error | `docs/error-experience/entries/YYYY-MM-DD-slug.md` | `docs/error-experience/summary/entries/YYYY-MM-DD-slug.md` |
| Win | `docs/good-experience/entries/YYYY-MM-DD-slug.md` | `docs/good-experience/summary/entries/YYYY-MM-DD-slug.md` |

Index files (`docs/error-experience.md`, `docs/good-experience.md`, and their `summary.md`) are **index-only** — never put content there.

See [Memory Management](memory-management.md) for authoring rules and networked memory graph.

---

## 12. Self-Correction Rule

Upon receiving **any** correction from the user, immediately write a preventive rule (in `docs/guides/`, `docs/error-experience/entries/`, or the relevant best-practice doc) to prevent the same class of mistake from recurring. Do not wait — codify the lesson before resuming work.

---

## 13. Codex Worker Protocol

For multi-file or mechanical implementations, use the Brain + Hands pattern:

| Phase | Owner | What |
|-------|-------|------|
| **EXPLORE** | Codex (read-only) | ≤5 probes, parallel when independent |
| **PLAN** | Claude Code | Architecture, task decomposition, verify commands |
| **EXECUTE** | Codex | Self-contained prompts, zero cross-call memory |
| **REVIEW** | Claude Code | Run tests, review diff, targeted re-dispatch |

Rules:
- Every Codex prompt is fully self-contained (paths, signatures, constraints, code snippets).
- End every prompt with: `"After changes, run: <verify command>"`.
- Max 2 retries per task. After 2 failures → escalate to user.
- Architecture decisions stay with Claude Code — Codex only executes designs.

See [Orchestration](orchestration.md) for task file format and multi-agent dispatch.

---

## Quick Reference: Full Cycle

```
Session Start → Pre-work Checklist → Plan (if non-trivial)
  → Worktree (if isolating) → Implement → Test
  → Code Review → Fix P0/P1 → Commit → Pre-push Gate → Push
  → Record Experience → Update Memory
```
