# 2026-02-10 Failed Session Diagnostics Persistence Fix

## Goal
- Ensure failed runs with explicit `session_id` do not fail early on `session not found`.
- Persist the missing session record so diagnostics pages can locate and inspect it.

## Scope
- `internal/app/agent/preparation/session.go`
- `internal/app/agent/preparation/session_test.go` (new)

## Plan
1. Reproduce/confirm failure path from logs and call chain.
2. Update session loading behavior:
   - When `session_id` is empty: keep existing `Create`.
   - When `session_id` is provided but missing: create and save a new empty session with the same ID.
3. Add unit tests for:
   - missing explicit session auto-created,
   - existing explicit session reused without extra save,
   - auto-create save failure surfaces error.
4. Run tests and lint.
5. Run mandatory code review checklist and fix findings.
6. Commit incrementally, merge back to `main`, cleanup worktree.

## Progress
- [x] Confirmed root cause via logs: repeated `failed to get session: session not found` in prepare stage.
- [x] Implemented loadSession auto-create-on-missing behavior.
- [x] Added unit tests for missing/existing/save-failure branches.
- [x] Ran `make fmt`, `make test`, `make check-arch`.
- [ ] Code review and commits.
