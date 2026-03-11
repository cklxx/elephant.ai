# 2026-03-11 Infra Tools Coverage

## Goal

Increase unit test coverage in `internal/infra/tools/`, with focused edge-case coverage for tool policy, SLA collector, and approval executor behavior.

## Constraints

- Work in a dedicated worktree.
- Keep production changes minimal; prefer tests unless a test exposes a safe bug fix.
- Preserve existing delivery/app/domain/infra boundaries.
- Run relevant tests, lint, and review before commit.

## Plan

1. Inspect `internal/infra/tools/` implementations and existing tests to find missing edge coverage.
2. Add targeted unit tests for tool policy, SLA collector, and approval executor boundary conditions.
3. Run focused tests with coverage for touched packages and verify no regressions.
4. Run `python3 skills/code-review/run.py review`, fix any P0/P1 findings, commit, and fast-forward merge.

## Findings

- `approval_executor.go` had no direct unit coverage, including nil delegate handling, approval bypass paths, approval rejection/error handling, and L3/L4 request-shaping helpers.
- `policy.go` already covered common matching flows but missed fallback behavior for non-positive timeout config and selector/helper edge cases.
- `sla.go` already covered steady-state collection but missed empty-window percentile behavior, zero-call averaging, and Prometheus register/type-mismatch error paths.

## Results

- Added targeted tests in `internal/infra/tools/approval_executor_test.go`.
- Extended `internal/infra/tools/policy_test.go` for timeout fallback and selector/helper edge cases.
- Extended `internal/infra/tools/sla_test.go` for empty-window, percentile clamp, average-cost, and register error/type-mismatch branches.
- `go test -coverprofile=/tmp/infra_tools.cover ./internal/infra/tools` now reports `93.8%` statement coverage for `internal/infra/tools` (up from `73.4%`).
