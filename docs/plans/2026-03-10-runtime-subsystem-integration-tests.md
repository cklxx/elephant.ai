## Runtime Subsystem Integration Tests

### Goal
Close the remaining direct Kaku runtime subsystem integration-test gaps after `runtime`, `adapter`, and `hooks` coverage landed.

### Remaining gaps
- `internal/runtime/session/` has only unit tests.
- `internal/runtime/pool/` has only unit tests.
- `internal/runtime/store/` has only unit tests.
- `internal/runtime/panel/` is intentionally excluded here because its integration suite is already being created in another pane.

### Approach
1. Add `//go:build integration` suites for `session`, `pool`, and `store`.
2. Keep them subsystem-focused:
   - `session`: end-to-end lifecycle timestamps, stall/recovery semantics.
   - `pool`: blocking handoff, waiter cancellation, slot ownership updates.
   - `store`: real filesystem persistence, `LoadAll` recovery, event log append behavior.
3. Run targeted package tests with and without the integration tag, then mandatory code review.

### Validation
- `go test ./internal/runtime/session ./internal/runtime/pool ./internal/runtime/store`
- `go test -tags=integration ./internal/runtime/session ./internal/runtime/pool ./internal/runtime/store`
- `python3 skills/code-review/run.py review`
