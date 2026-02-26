# 2026-02-26 Lark Sync/Async Terminal Status Unification

## Goal

Systematically unify foreground (`runTask -> dispatchResult`) and background (`backgroundProgressListener`) terminal status handling so failures are visible, completion semantics are stable, and debugging is straightforward.

## Scope

- `internal/delivery/channels/lark/` only.
- No behavior changes for `web/`.
- Keep existing user-facing command contracts (`/stop`, `/new`, `/task*`) intact.

## Plan

- [x] Re-check `main` pre-work checklist and detect unrelated dirty files.
- [x] Review engineering guide + recent memory/postmortem entries.
- [x] Introduce shared status normalization/terminal helpers for Lark tasks.
- [x] Apply shared helpers to task store, task command, and background progress listener.
- [x] Add regression tests for status normalization and async completion convergence.
- [x] Run targeted tests for `internal/delivery/channels/lark`.
- [x] Run mandatory code review skill and fix blocking findings.
- [x] Commit only task-related files.

## Notes

- Existing unrelated local diffs in `internal/infra/llm/*` are intentionally excluded from this task.

## Progress Log

- 2026-02-26 14:45 CST: Added shared task status helpers (`normalize/isActive/isTerminal/normalizeCompletion`).
- 2026-02-26 14:46 CST: Replaced repeated status string branches in `task_store_local`, `task_command`, and `background_progress_listener`.
- 2026-02-26 14:47 CST: Added regression tests for status normalization and async completion convergence.
- 2026-02-26 14:48 CST: Verified with `go test ./internal/delivery/channels/lark -count=1` (pass).
- 2026-02-26 14:49 CST: Ran mandatory `python3 skills/code-review/run.py '{"action":"review"}'` (no P0/P1 findings reported).
