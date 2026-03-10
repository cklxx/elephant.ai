# Static Analysis Report

Date: 2026-03-10

## Commands Run

```bash
go vet ./...
staticcheck ./...
```

`staticcheck` was not installed initially, so it was installed with:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Summary

- `go vet ./...`: passed with no findings
- `staticcheck ./...`: initially reported 28 findings
- Real issues fixed: 25
- Remaining findings: 3
- Remaining findings are style-only nits, not correctness defects

## Real Issues Fixed

### Dead code removed

- Removed unused `envBool` from `cmd/alex/dev_lark.go`
- Removed unused `injectRepliesToMessengerCalls` from `cmd/alex/lark_inject_cmd.go`
- Removed unused phrase aliases from `internal/delivery/channels/lark/tool_phrases.go`
- Removed unused `normalizedToolName` from `internal/delivery/server/http/sse_handler_stream.go`
- Removed unused mutex field from `internal/domain/agent/react/compression_artifact_stress_test.go`
- Deleted unused `internal/infra/tools/builtin/session/session_guard.go`

Assessment:

- These were genuine dead-code findings (`U1000`), not stylistic preferences.
- Deleting them reduces maintenance surface and keeps future refactors simpler.

### Broken test logic fixed

- Removed the unused `allMessages = append(...)` write in `internal/app/context/history_manager_test.go`

Assessment:

- `staticcheck` flagged this as `SA4010`: the append result was never observed.
- This was a real test-quality defect because the code looked like it was accumulating state, but the accumulated slice was never used.

### Nil-context test calls normalized

- Reworked nil-context assertions in:
  - `internal/infra/tools/builtin/shared/context_test.go`
  - `internal/shared/utils/id/context_test.go`

Assessment:

- These were test-only `SA1012` findings.
- The production code intentionally supports nil contexts; the tests now verify that behavior without tripping the analyzer on literal `nil` context arguments.

### Deprecated network error handling fixed

- Updated `internal/shared/errors/types.go` to stop relying on deprecated `net.Error.Temporary()`
- Preserved compatibility for legacy errors that still expose `Temporary() bool` through a non-deprecated interface path
- Kept DNS errors classified as network errors

Assessment:

- This was the only production-path finding with meaningful correctness risk.
- The updated logic avoids deprecated API usage without regressing transient-error classification.

## Remaining Findings

These were intentionally left unchanged because they are style-level nits rather than real defects:

| File | Analyzer | Finding |
|---|---|---|
| `internal/delivery/channels/lark/auto_auth.go` | `S1039` | unnecessary `fmt.Sprintf` |
| `internal/runtime/adapter/claude_code.go` | `S1005` | unnecessary assignment to `_` |
| `internal/shared/json/jsonx.go` | `ST1008` | `error` should be last argument |

Assessment:

- None of these indicate a runtime bug, safety issue, or test defect.
- They can be cleaned up separately if the goal is a fully clean `staticcheck` run, but they were not prioritized under the current instruction.

## Validation

Commands rerun after fixes:

```bash
go vet ./...
staticcheck ./...
go test ./cmd/alex ./internal/app/context ./internal/delivery/channels/lark ./internal/domain/agent/react ./internal/infra/tools/builtin/session ./internal/infra/tools/builtin/shared ./internal/domain/agent/ports/mocks ./internal/shared/errors ./internal/shared/utils/id
```

Results:

- `go vet ./...`: passed
- targeted `go test` sweep: passed
- `staticcheck ./...`: only the 3 style-only findings above remain
