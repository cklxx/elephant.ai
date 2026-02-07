# Plan: Fix All Remaining Validation Failures

**Status:** done
**Branch:** fix/all-failures-20260208
**Created:** 2026-02-08

## Goal

Resolve all currently known validation failures so full delivery gates are green:
- Go lint
- Go tests
- Web lint
- Web tests

## Steps

### 1. Reproduce failures
- [x] Create fresh worktree from `main` and copy `.env`.
- [x] Load engineering practices + active memory summaries.
- [x] Re-run full lint/test commands and capture exact failing files.

### 2. Implement fixes
- [x] Fix Go lint issues in `evaluation/...` and `internal/delivery/eval/http/...`.
- [x] Fix failing web tests in `web/lib/__tests__/webgl.test.ts` or source code mismatch.
- [x] Keep changes minimal and scoped to failures.

### 3. Validate
- [x] Run targeted tests for touched packages/files first.
- [x] Run full repo lint.
- [x] Run full repo test suites used by `dev.sh`/web scripts.

### 4. Finish
- [x] Update this plan to `done`.
- [x] Add error/good experience records if new patterns are discovered.
- [ ] Commit incremental fixes and merge back into `main`.
- [ ] Remove temporary worktree (and optional branch cleanup).

## Validation Notes

- Root cause of `/api/auth/google/login` returning `503`: auth config only read YAML values and could disable auth module entirely when JWT/database config was unavailable.
- Added auth env fallback in server bootstrap config resolution and development-mode fallback behavior for missing JWT/database.
- Added server-side e2e tests for auth/login and dev log index routes over `httptest.NewServer`.
- Full validation passed:
  - `./dev.sh lint`
  - `./dev.sh test`
  - `npm --prefix web run test`
