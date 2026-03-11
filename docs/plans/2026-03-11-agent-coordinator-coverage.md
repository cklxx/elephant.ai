# Agent Coordinator Coverage

## Scope

- Improve unit coverage under `internal/app/agent/`.
- Focus on coordinator error paths.
- Focus on session management boundary conditions and persistence behavior.
- Keep changes test-only unless a real defect is exposed.

## Plan

1. Measure current `internal/app/agent/...` coverage and inspect uncovered coordinator/session branches.
2. Add unit tests for session persistence failures, history append variants, attachment migration fallback, and reset edge cases.
3. Add coordinator error-path tests around execution finalization and persistence failures.
4. Run focused and full validation, review, commit, and merge back to `main`.

## Findings

- `internal/app/agent/coordinator/session_manager.go` had good happy-path coverage but weak coverage on persistence failures and history/reset edge cases.
- `appendHistoryTurn` had no direct coverage for the optimized `AppendTurnWithExisting` path.
- `ResetSession` lacked assertions around missing-session history cleanup and error propagation order (`Save` before `ClearSession`).
- `SaveSessionAfterExecution` lacked direct coverage for store save failures and attachment migration fallback behavior.
- `finalizeExecution` lacked direct unit coverage for persist failure handling and the subagent execution-error path that stamps error metadata while skipping persistence.

## Coverage Impact

- `coordinator` package coverage increased from `69.9%` to `71.6%`.
- `finalizeExecution` increased to `91.7%`.
- `SaveSessionAfterExecution` increased to `100.0%`.
- `ResetSession` increased to `93.8%`.

## Validation

- `go test -coverprofile=/tmp/coordinator.cover ./internal/app/agent/coordinator`
- `go test -coverprofile=/tmp/agent.cover ./internal/app/agent/...`
- `./scripts/ci-check.sh lint`
- `go test ./...`
- `python3 skills/code-review/run.py review`
