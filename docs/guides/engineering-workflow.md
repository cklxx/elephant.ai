# Engineering Workflow

Updated: 2026-03-09

End-to-end engineering standards. Every contributor (human and AI agent) follows this flow.

See also: [Code Simplification](code-simplification.md) | [Code Review](code-review-guide.md) | [Memory Management](memory-management.md)

---

## 1. Principles

- Trust invariants guaranteed by types or caller contracts -- no unnecessary defensive code.
- Delete dead code completely -- no `// deprecated`, `_unused` prefixes, or commented-out code.
- Redesign interfaces when requirements change -- no adapter shims or backward-compat wrappers.
- Use existing project patterns -- survey neighboring files before inventing new patterns.
- Only modify files relevant to the task.

### Architecture

- **Context engineering over prompt hacking** -- modify context assembly (`internal/context/`) first.
- **Typed events over unstructured logs** -- use typed event structs (`internal/agent/domain/events/`).
- **Clean port/adapter boundaries** -- cross-layer imports go through port interfaces. Enforce with `make check-arch`.
- **Multi-provider LLM support** -- new LLM features must work across all providers in `internal/llm/`.
- **Skills and memory over one-shot answers** -- persist learnings; encode reusable workflows as skills.

### Proactive Behavior Constraints

When modifying proactive behavior code (`internal/agent/`, skill triggers, context injection):
- Minimum-effective intervention: `clarify` -> `plan` -> reminder/schedule/task execution.
- Every proactive suggestion must remain user-overridable; never remove opt-out paths.
- No manipulative framing (fear, guilt, urgency) in LLM prompt construction.
- External messages or irreversible operations must pass approval gates.
- Honor stop signals immediately.

---

## 2. Session Start

1. Greet **ckl**.
2. Load the [always-load memory set](memory-management.md#always-load-set).
3. On `main`, execute **pre-work checklist** before any code change:
   - `git diff --stat` + `git log --oneline -10`
   - Flag suspicious changes to ckl before proceeding.
   - Applies to every new task, including mid-conversation topic switches.

Skip pre-work checklist when inside a worktree.

---

## 3. Planning

- **Simple tasks**: implement directly.
- **Non-trivial tasks**: plan under `docs/plans/YYYY-MM-DD-short-slug.md`. Update as work progresses.
- For governance tasks, specify deterministic file-type-to-directory mapping; include explicit first-level namespace routing for `internal/**`.

### Decision Priorities

1. Hard constraints and explicit rules.
2. Reversibility / order of operations.
3. Missing info only if it changes correctness.
4. User preferences within constraints.

---

## 4. Worktree Workflow

Use worktrees for isolation. See CLAUDE.md for worktree preferences (auto-merge, `.worktree-active.yaml`).

```bash
git worktree add -b <branch> ../<dir> main
cp .env ../<dir>/
# develop in the worktree
git checkout main && git merge --ff-only <branch>
git worktree remove ../<dir>
```

Branch delete fallback: `git update-ref -d refs/heads/<branch>` then `git worktree prune`.

---

## 5. Code Style

- Keep naming consistent; follow local naming guidelines.
- Prefer struct/options pattern over long parameter lists.
- Prefer `internal/shared/json` (`jsonx`) for hot-path JSON.
- Follow [Code Simplification Rules](code-simplification.md).
- When users choose multiple numbered options, implement all in the same delivery.

### Go + OSS

- `gofmt` + `goimports`. Package names lowercase, no underscores.
- Exported identifiers: full-sentence doc comments starting with identifier name.
- `context.Context` as first param; never store in structs.
- Check/handle errors; wrap with context. No `panic` for normal flow.
- Avoid fire-and-forget goroutines; prefer table-driven tests.

---

## 6. Testing

- **TDD** when touching logic; cover edge cases.
- Mocking: only in unit tests. Integration/E2E use real dependencies.
- Run `alex dev lint` + `alex dev test` before delivery.
- macOS: `CGO_ENABLED=0` for `go test -race`; `CGO_ENABLED=1` only when cgo required.

---

## 7. Code Review

**Mandatory before every commit.** After lint + tests: `python3 skills/code-review/run.py review`

- **P0/P1**: must fix before commit.
- **P2**: create follow-up task.
- **P3**: optional.

---

## 8. Commit & Delivery

- Always commit after completing changes. Prefer small, incremental commits.
- Only commit your own code. Surface unrelated modified files; exclude unless asked.
- Restart only the changed service: `alex dev restart backend` / `alex dev restart web`.

### Pre-Push Gate

`scripts/pre-push.sh` mirrors CI. Always before `git push`. Skip: `SKIP_PRE_PUSH=1`.
Push only from primary workspace or its managed worktrees.

---

## 9. Safety

- Avoid destructive operations or history rewrites unless explicitly requested.
- Prefer reversible steps and explain risks when needed.
- For OAuth: return provider's official authorization URL; keep `/api/*` routes as internal relay.

---

## 10. Experience Records

| Type | Entry path | Summary path |
|------|-----------|--------------|
| Error | `docs/error-experience/entries/YYYY-MM-DD-slug.md` | `docs/error-experience/summary/entries/YYYY-MM-DD-slug.md` |
| Win | `docs/good-experience/entries/YYYY-MM-DD-slug.md` | `docs/good-experience/summary/entries/YYYY-MM-DD-slug.md` |

Index files are **index-only**. See [Memory Management](memory-management.md) for authoring rules.

---

## 11. Codex Worker Protocol

See [Orchestration](orchestration.md) for full protocol. Summary: Brain (Claude Code) + Hands (Codex) pattern — EXPLORE → PLAN → EXECUTE → REVIEW. Every Codex prompt must be self-contained. Max 2 retries per task.

---

## Quick Reference

```
Session Start -> Pre-work Checklist -> Plan (if non-trivial)
  -> Worktree -> Implement -> Test -> Code Review
  -> Fix P0/P1 -> Commit -> Pre-push Gate -> Push
  -> Record Experience -> Update Memory
```
