status: completed

## Scope

- Audit goroutine lifetime issues across the project.
- Prioritize `internal/runtime/` and `internal/app/agent/`.
- Check channel lifecycle and context cancellation propagation.
- Fix concrete leaks found and validate with focused tests.

## Plan

1. Scan `go func`, channel creation/close, and context cancellation sites.
2. Confirm real leak risks in runtime and agent code paths.
3. Patch exit paths and add focused regression tests.
4. Run relevant tests, lint if feasible, then code review and commit.

## Findings

- Fixed `internal/app/agent/coordinator/session_manager.go`: async session save used `context.WithoutCancel` plus `sync.Once`, which kept one background goroutine alive for the coordinator lifetime. Replaced it with an on-demand save loop that exits once idle and can restart later.
- Fixed `internal/runtime/runtime.go`: session adapters inherited request-scoped contexts, so HTTP request completion could cancel long-running runtime sessions. Runtime now creates per-session contexts and cancels them on terminal state or explicit stop.
- Fixed `internal/runtime/hooks/bus_impl.go`: subscription channels created by `Subscribe`/`SubscribeAll` were never closed. Cancel now removes and closes them safely with an idempotent `sync.Once`.
- Fixed `internal/runtime/adapter/codex.go`: runtime-triggered session cancellation no longer reports a spurious failure event for a deliberately stopped session.

## Validation

- `go test ./internal/runtime/... ./internal/app/agent/coordinator`
- `golangci-lint run ./internal/runtime/... ./internal/app/agent/coordinator`
- `python3 skills/code-review/run.py review`
- `alex dev lint` was attempted but the repo environment is missing `eslint`, so the web lint phase could not run.
