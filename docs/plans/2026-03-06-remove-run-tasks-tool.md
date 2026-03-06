# Remove `run_tasks` / `reply_agent` Tool Surface

Date: 2026-03-06
Branch: feat/remove-run-tasks-tool

## Goal

Remove `run_tasks` and `reply_agent` as tool-layer concepts for team orchestration, while preserving all team capabilities through `alex team ...` and `skills/team-cli`.

## Scope

- Extract reusable team execution logic away from tool wrappers.
- Update `alex team` to call the non-tool execution path directly.
- Delete tool registry registration surface and direct tool constructors.
- Update compile-blocking tests and code paths that still reference the removed tool wrappers.
- Verify real CLI behavior still works for `run`, `status`, and `inject`.

## Steps

1. Introduce a non-tool team runner service.
2. Switch CLI to the new service.
3. Remove `run_tasks` / `reply_agent` wrapper code and registry entry points.
4. Update impacted tests and internal references.
5. Run focused tests plus real CLI smoke checks.

## Progress

- [x] Baseline scan
- [x] Implement non-tool runner
- [x] Remove tool wrappers and dead references
- [x] Run tests
- [x] Real CLI validation
- [ ] Commit
