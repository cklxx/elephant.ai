## Runtime Integration Tests

### Goal
Add `internal/runtime/runtime_integration_test.go` with integration-tagged coverage for runtime lifecycle, stall detection, leader intervention, pane pool reuse, and persistence recovery without requiring a real Kaku binary.

### Approach
1. Add a minimal adapter-factory interface in `internal/runtime/runtime.go` so tests can inject a mock factory while production continues to use `*adapter.Factory`.
2. Reuse the existing mock pane and manager patterns from `internal/runtime/runtime_test.go`.
3. Build integration tests around real runtime, store, bus, leader agent, and pool behavior with a test adapter that simulates split/direct-pane start and text injection.
4. Verify with focused `go test` runs using the `integration` build tag, then run code review and commit.

### Validation
- `go test ./internal/runtime`
- `go test -tags=integration ./internal/runtime`
- `python3 skills/code-review/run.py review`
