# Plan: Log Analyzer Availability and Full-Page Validation

**Status:** done
**Branch:** fix/log-analyzer-availability
**Created:** 2026-02-07

## Goal

Stabilize the dev log analyzer chain so `/dev/log-analyzer` can reliably fetch log index data, include Lark chain logs in the same trace path, and verify local pages/APIs end-to-end.

## Scope

1. Reproduce and root-cause `/api/dev/logs/index` returning `404`.
2. Fix route/config/startup issues and harden fallback behavior.
3. Ensure one-command startup for visual log analysis workflow.
4. Execute API + page validation and automated tests.

## Execution Steps

### Step 1: Reproduce and Inspect
- [x] Create fresh worktree from `main` and copy `.env`.
- [x] Load engineering practices and memory summaries.
- [x] Reproduce backend/frontend issue from clean local run.
- [x] Identify exact mismatch (routing, devMode, auth, or startup script).

### Step 2: Implement Fix
- [x] Apply minimal code/script fix for `/api/dev/logs/index` availability.
- [x] Confirm Lark chain logs are in the same index scan path.
- [x] Add/adjust UX fallback if API is unavailable.

### Step 3: Validate
- [x] Verify `http://localhost:3000/dev/log-analyzer` loads and displays data.
- [x] Verify `GET /api/dev/logs/index?limit=120` returns expected status/body.
- [x] Run targeted unit/integration tests.
- [x] Run full lint + test suite.

### Step 4: Finish
- [x] Update plan status to `done` with final checklist.
- [x] Add error/good experience entries if new patterns are discovered.
- [x] Commit incremental changes and merge back to `main`.
- [x] Remove temporary worktree and optional branch cleanup.

## Validation Notes

- Root cause of user-observed `404` was stale backend process state and missing logs-ui readiness verification; endpoint code and router registration were already present.
- Added logs-ui readiness probes to auto-restart backend/web if the log analyzer chain is unavailable.
- Local verification: `/dev` and `/dev/log-analyzer` returned `200`; `/api/dev/logs/index` and `/api/dev/logs` returned `401` (auth protected, route present).
- Full Go tests passed via `./dev.sh test`.
- Full lint/test status:
  - `./dev.sh lint` failed due pre-existing unrelated issues under `evaluation/...` and `internal/delivery/eval/http/...`.
  - `npm --prefix web run test` failed due pre-existing `web/lib/__tests__/webgl.test.ts` expectation mismatch (2 failures).
