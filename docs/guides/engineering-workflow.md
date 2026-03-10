# Engineering Workflow

Updated: 2026-03-10

Use this flow for every engineering task.

See also: [Code Simplification](code-simplification.md) | [Code Review](code-review-guide.md) | [Memory Management](memory-management.md)

## Core Rules

- Priority order: safety > correctness > maintainability > speed.
- Trust caller and type invariants. Do not add defensive code without a real gap.
- Delete dead code. Do not leave deprecated branches, `_unused` names, or commented-out code.
- When requirements change, redesign cleanly. Do not add compatibility shims.
- Follow local patterns before introducing a new one.
- Change only files that matter to the task.

## Session Start

1. Greet `ckl`.
2. Load the [always-load set](memory-management.md#always-load-set).
3. If you are on `main`, run:

```bash
git diff --stat
git log --oneline -10
```

4. If the diff or recent history looks suspicious, stop and report it before editing.
5. Use a worktree for any code or doc change. Skip the `main` pre-check once you are inside the worktree.

## Planning

- Simple task: inspect the target files and implement directly.
- Non-trivial task: create `docs/plans/YYYY-MM-DD-short-slug.md` and keep it updated.
- Ask for missing information only when it changes correctness.

Decision order:
1. Explicit rules and hard constraints.
2. Reversibility and safe ordering.
3. Missing information that affects correctness.
4. User preference within the rules.

## Worktree Flow

```bash
git worktree add -b <branch> ../<dir> main
cp .env ../<dir>/
```

Create `../<dir>/.worktree-active.yaml`:

```yaml
status: in_progress
```

Finish with:

```bash
git checkout main
git merge --ff-only <branch>
```

Update the marker to `status: merged`, then remove the worktree:

```bash
git worktree remove ../<dir>
```

## Implementation Rules

- Keep naming and file structure consistent with nearby code.
- Prefer simpler APIs such as struct/options over long parameter lists.
- Follow [Code Simplification](code-simplification.md).
- If the user selects multiple numbered options, implement all of them in one delivery.

For `internal/**` work:
- Keep delivery -> application -> domain -> infra boundaries clean.
- Cross-layer dependencies go through ports/interfaces.
- Prefer typed events over unstructured logs.

For proactive behavior changes (`internal/agent/`, triggers, context injection):
- Use the minimum effective action: `clarify` -> `plan` -> reminder/schedule/task execution.
- Keep every suggestion user-overridable.
- No manipulative framing.
- Require approvals for external messages and irreversible actions.
- Honor stop signals immediately.

## Validation

- For logic changes, prefer TDD and cover edge cases.
- Use mocks only in unit tests. Integration and E2E tests should use real dependencies.
- Run relevant lint and tests before delivery. Default full gate:

```bash
alex dev lint
alex dev test
```

- On macOS, use `CGO_ENABLED=0` for `go test -race` unless cgo is required.

## Review And Commit

- Before every commit, run:

```bash
python3 skills/code-review/run.py review
```

- Fix P0 and P1 findings before commit.
- Create a follow-up for P2 findings.
- Commit after the task is complete. Keep commits small and scoped.
- Commit only your own changes. Surface unrelated diffs instead of sweeping them in.

## Delivery Rules

- Run `scripts/pre-push.sh` before `git push`. It mirrors CI.
- Push only from the primary repo or its managed worktrees.
- Restart only the service you changed.
- Avoid destructive operations or history rewrites unless explicitly requested.

## Experience Records

| Type | Entry path | Summary path |
|------|------------|--------------|
| Error | `docs/error-experience/entries/YYYY-MM-DD-slug.md` | `docs/error-experience/summary/entries/YYYY-MM-DD-slug.md` |
| Win | `docs/good-experience/entries/YYYY-MM-DD-slug.md` | `docs/good-experience/summary/entries/YYYY-MM-DD-slug.md` |

Index files stay index-only. See [Memory Management](memory-management.md) for loading and authoring rules.

## Codex Worker Protocol

Use EXPLORE -> PLAN -> EXECUTE -> REVIEW. Keep worker prompts self-contained. Limit retries to 2 per task.
