# Engineering Workflow

Updated: 2026-03-04

End-to-end engineering standards and development cycle for elephant.ai. Every contributor (human and AI agent) follows this flow.

See also: [Code Simplification](code-simplification.md) | [Code Review](code-review-guide.md) | [Incident Response](incident-response.md) | [Memory Management](memory-management.md)

---

## 1. Principles

- **Safety > correctness > maintainability > speed.**
- Prefer correctness and maintainability over short-term speed.
- Trust invariants guaranteed by types or caller contracts -- no unnecessary defensive code.
- Delete dead code completely -- no `// deprecated`, `_unused` prefixes, or commented-out code.
- Redesign interfaces when requirements change -- no adapter shims or backward-compat wrappers.
- Use existing project patterns -- survey neighboring files before inventing new patterns.
- Only modify files relevant to the task.
- Keep config examples in YAML only.

### Architecture

- **Context engineering over prompt hacking** -- modify context assembly (`internal/context/`) first. Prompt templates only if context changes are verified insufficient.
- **Typed events over unstructured logs** -- use typed event structs (`internal/agent/domain/events/`). No free-form log strings for state transitions.
- **Clean port/adapter boundaries** -- cross-layer imports go through port interfaces. Direct infra-to-domain imports forbidden. Keep `agent/ports` free of memory/RAG deps. Enforce with `make check-arch`.
- **Multi-provider LLM support** -- new LLM features must work across all providers in `internal/llm/`. No provider-specific APIs without adapter.
- **Skills and memory over one-shot answers** -- persist learnings to memory; encode reusable workflows as skills.
- **Proactive context injection** -- auto-inject relevant context before user asks. Manual retrieval is fallback.

### Proactive Behavior Constraints

When modifying proactive behavior code (`internal/agent/`, skill triggers, context injection):
- Detect motivation state before proactive actions: low energy, overload, ambiguity, or clear readiness.
- Minimum-effective intervention: `clarify` -> `plan` -> reminder/schedule/task execution.
- Every proactive suggestion must remain user-overridable; never remove opt-out paths.
- Prefer progress visibility (artifacts/checkpoints) over high-frequency nudges.
- No manipulative framing (fear, guilt, urgency) in any LLM prompt construction.
- External messages or irreversible operations must pass approval gates. No exceptions.
- Honor stop signals immediately.

---

## 2. Session Start

1. Greet **ckl**.
2. Load the [always-load memory set](memory-management.md#always-load-set).
3. On the `main` branch, execute the **pre-work checklist** before any code change:
   - `git diff --stat` -- see what files are currently modified.
   - `git log --oneline -10` -- review recent commit history.
   - Identify changes that should not have been made: unrelated files touched, intentional logic reverted, needed code deleted, style regressions.
   - If suspicious changes exist -> flag to ckl before proceeding. Never silently overwrite or build on top of incorrect diffs.
   - Applies to every new task, including mid-conversation topic switches.

Skip the pre-work checklist only when inside a worktree (all changes there are your own).

---

## 3. Planning

- **Simple tasks**: implement directly.
- **Non-trivial tasks**: write a plan under `docs/plans/YYYY-MM-DD-short-slug.md`.
- Before each task, review engineering practices under `docs/guides/`; if guidance is missing, search industry sources and add it.
- Update plan files as work progresses.
- For governance and folder-rule tasks, specify deterministic file-type-to-directory mapping and naming rules; avoid ambiguous high-level wording.
- For `internal/**` governance, always include explicit first-level namespace routing (`app/domain/infra/delivery/shared/devops/testutil`) and forbidden placements.

### Decision Priorities

When conflicts arise, resolve in this order:

1. Hard constraints and explicit rules.
2. Reversibility / order of operations.
3. Missing info only if it changes correctness.
4. User preferences within constraints.

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

### Active Worktree Marking (mandatory)

To avoid accidental worktree deletion during active work:

1. Immediately after creating or re-entering a worktree, create `<worktree>/.worktree-active.yaml`:
   ```yaml
   owner: ckl
   agent: codex
   branch: <branch>
   path: <absolute-worktree-path>
   task: <short-task-name>
   status: in_progress
   started_at: <RFC3339 timestamp>
   last_touch_at: <RFC3339 timestamp>
   ```
2. Update `last_touch_at` whenever handing off phases (explore -> implement -> test -> review) and before long-running commands.
3. Never remove a worktree while `.worktree-active.yaml` has `status: in_progress`.
4. After successful `git merge --ff-only <branch>`, set `status: merged` and then remove the worktree.
5. On interruption/crash, keep the marker file untouched so the next session can detect that work was in progress.

---

## 5. Implementation

### Code Style

- Avoid unnecessary defensive code; trust invariants when guaranteed.
- Keep naming consistent; follow local naming guidelines when present.
- Be cautious with long parameter lists; prefer grouping into a struct or options pattern and document the boundary explicitly.
- Prefer `internal/shared/json` (`jsonx`) for JSON encode/decode hot paths; avoid direct `encoding/json` unless `jsonx` cannot provide the required API.
- Follow the [Code Simplification Rules](code-simplification.md).
- When users choose multiple numbered options, implement every selected item in the same delivery unless explicitly constrained.

### Go + OSS

- Formatting/imports: always run `gofmt`; use `goimports` to manage imports.
- Naming: package names are lowercase, no underscores/dashes; avoid redundant interface names (`storage.Interface` not `storage.StorageInterface`).
- Comments: exported identifiers have full-sentence doc comments starting with the identifier name.
- Context: pass `context.Context` explicitly (first param); never store it in structs.
- Errors: check/handle errors; avoid `panic` for normal flow; wrap with context when returning.
- Concurrency & tests: avoid fire-and-forget goroutines (make lifetimes explicit); prefer table-driven tests for multi-case coverage.

Sources: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Kubernetes Coding Conventions](https://www.kubernetes.dev/docs/guide/coding-convention/).

---

## 6. Testing

- Use **TDD** when touching logic; cover edge cases.
- Mocking policy: only unit tests may use mock/fake/stub. Integration/E2E/inject/live validation must use real runtime dependencies and real external integration paths.
- Run full lint + tests before delivery:
  ```bash
  alex dev lint
  alex dev test
  ```
- On macOS, prefer `CGO_ENABLED=0` for `go test -race` to avoid LC_DYSYMTAB linker warnings; set `CGO_ENABLED=1` when cgo is required.

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

## 8. Commit & Delivery

- Always commit after completing changes.
- One solution -> multiple **incremental commits** (prefer small, reviewable commits).
- Only commit your own code. Fix conflicts, but never roll back others' code.
- When unrelated modified files are present, explicitly surface them and exclude them from staging/commit unless the user asks to include them.

### Service Restart

Restart only the changed service -- never full stack:

```bash
alex dev restart backend   # backend changes
alex dev restart web       # web changes
```

Never `./dev.sh down && ./dev.sh`.

For kernel validation: `go run ./cmd/alex-server kernel-once`.

### Pre-Push Gate

`scripts/pre-push.sh` mirrors CI fast-fail checks. Always runs before `git push`. Skip only with `SKIP_PRE_PUSH=1`.

Note: parallel checks on large diffs may produce `-race` flakes and lint timeouts. Confirm with a targeted re-run before treating as a real regression.

---

## 9. Safety

- Avoid destructive operations or history rewrites unless explicitly requested.
- Prefer reversible steps and explain risks when needed.
- For user-facing OAuth guidance, always return the provider's official authorization URL; keep local `/api/*` OAuth routes as internal relay/callback plumbing.

---

## 10. Experience Records

After completing work, record notable outcomes:

| Type | Entry path | Summary path |
|------|-----------|--------------|
| Error | `docs/error-experience/entries/YYYY-MM-DD-slug.md` | `docs/error-experience/summary/entries/YYYY-MM-DD-slug.md` |
| Win | `docs/good-experience/entries/YYYY-MM-DD-slug.md` | `docs/good-experience/summary/entries/YYYY-MM-DD-slug.md` |

Index files (`docs/error-experience.md`, `docs/good-experience.md`, and their `summary.md`) are **index-only** -- never put content there.

See [Memory Management](memory-management.md) for authoring rules and networked memory graph.

---

## 11. Self-Correction

Upon receiving **any** correction from the user, immediately write a preventive rule (in `docs/guides/`, `docs/error-experience/entries/`, or the relevant best-practice doc) to prevent the same class of mistake from recurring. Do not wait -- codify the lesson before resuming work.

---

## 12. Codex Worker Protocol

For multi-file or mechanical implementations, use the Brain + Hands pattern:

| Phase | Owner | What |
|-------|-------|------|
| **EXPLORE** | Codex (read-only) | <=5 probes, parallel when independent |
| **PLAN** | Claude Code | Architecture, task decomposition, verify commands |
| **EXECUTE** | Codex | Self-contained prompts, zero cross-call memory |
| **REVIEW** | Claude Code | Run tests, review diff, targeted re-dispatch |

Rules:
- Every Codex prompt is fully self-contained (paths, signatures, constraints, code snippets).
- End every prompt with: `"After changes, run: <verify command>"`.
- Max 2 retries per task. After 2 failures -> escalate to user.
- Architecture decisions stay with Claude Code -- Codex only executes designs.

See [Orchestration](orchestration.md) for task file format and multi-agent dispatch.

---

## Quick Reference

```
Session Start -> Pre-work Checklist -> Plan (if non-trivial)
  -> Worktree (if isolating) -> Implement -> Test
  -> Code Review -> Fix P0/P1 -> Commit -> Pre-push Gate -> Push
  -> Record Experience -> Update Memory
```
