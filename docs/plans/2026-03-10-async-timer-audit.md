## Goal

Audit `internal/shared/async/` and `internal/shared/timer/` for dead code, goroutine leak risks, and overly complex timer logic; simplify the async execution model without changing intended behavior.

## Progress

- [x] Inspect both packages and their tests to map the current APIs, timer lifecycle rules, and async patterns.
- [x] Remove dead code and collapse redundant abstractions where callers already guarantee invariants.
- [x] Add focused tests for cancellation, stopped-manager behavior, and panic-safe async execution.
- [x] Run relevant lint/tests and perform code review.
- [ ] Commit and fast-forward merge to `main`.

## Notes

- `internal/shared/async` exported more surface than callers used. The panic-safe execution path now has a single implementation: `Run`, and `Go` is just the async wrapper.
- `internal/shared/timer` had duplicated scheduling branches and allowed stale callbacks to keep executing after `Cancel` or `Stop`. Scheduling now resolves by `timerID`, checks active state at fire time, and uses `context.AfterFunc` instead of a dedicated stop-watcher goroutine.
- The unused `Timer.Delay` field was removed from the Go runtime model.

## Validation

- `go test ./internal/shared/async ./internal/shared/timer`
- `CC=/usr/bin/clang ./scripts/run-golangci-lint.sh run ./internal/shared/async ./internal/shared/timer`
- `go test ./internal/delivery/server/bootstrap`
- `python3 skills/code-review/run.py review`
