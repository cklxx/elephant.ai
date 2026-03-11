# Runtime Coverage

Date: 2026-03-11
Branch: `test/runtime-coverage`

## Scope

Increase unit test coverage for `internal/runtime/`, with emphasis on:
- error paths
- boundary conditions
- lowest-coverage runtime subpackages

## Plan

1. Measure `internal/runtime/...` coverage and inspect the lowest packages.
2. Add focused tests for uncovered failure paths and edge cases.
3. Run targeted validation, then commit and fast-forward merge back to `main`.

## Progress

- [x] Coverage gaps identified
- [x] Tests added
- [x] Validation completed
- [ ] Merged back to `main`

## Coverage Notes

- Baseline: `go test -cover ./internal/runtime/...`
  - `internal/runtime`: `77.9%`
  - `internal/runtime/panel`: `43.1%`
  - `internal/runtime/adapter`: `74.4%`
- After tests:
  - `internal/runtime`: `87.8%`
  - `internal/runtime/panel`: `79.9%`
  - `internal/runtime/adapter`: `79.5%`

## Implemented Tests

- `internal/runtime/runtime_test.go`
  - factory creation failure marks the session failed
  - adapter start failure in pool mode releases the pane and clears runtime state
  - completion clears adapter/cancel bookkeeping
  - `RecordEvent` appends the session event log
- `internal/runtime/panel/panel_test.go`
  - Kaku manager split/list happy path with defaulted direction, percent, and cwd
  - split parse failure and command stderr propagation
  - pane command paths for inject, submit, send, send-key, capture, activate, and kill
  - `sendViaPane` inject/submit error propagation
- `internal/runtime/adapter/claude_code_internal_test.go`
  - completion watcher handles pane capture failure
  - completion watcher detects shell prompt and removes pane state

## Validation

- `go test ./internal/runtime/panel ./internal/runtime/adapter ./internal/runtime`
- `go test -cover ./internal/runtime/...`
- `CC=/usr/bin/clang ./scripts/run-golangci-lint.sh run ./internal/runtime/...`
