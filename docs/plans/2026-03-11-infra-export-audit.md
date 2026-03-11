status: completed

## Scope

- Audit `internal/infra/**` exported API surface.
- Lower symbols that are only used within their own package.
- Delete dead exported surface where callers only need interfaces.
- Validate with `go vet` and focused `go test`, then commit.

## Plan

1. Confirm package-local-only exports via symbol/reference scans.
2. Lower concrete exported types and helper functions to unexported forms.
3. Remove dead exported interface surface where an anonymous interface is sufficient.
4. Run `go vet` and focused `go test`, then commit.

## Findings

- `internal/infra/process`: `MergeEnv`, `DefaultStderrTail`, `TailBuffer`, and `NewTailBuffer` were only used inside the package. Lowered them to package-private helpers and updated tests/call sites.
- `internal/infra/adapters`: concrete exported types were not referenced outside the package; callers only used constructors. Lowered `FileEventAppender`, `OSAtomicWriter`, and `OSStatusFileIO` to package-private concrete implementations while preserving constructor entrypoints.
- `internal/infra/notification`: `LarkSender`, `MoltbookSender`, `CompositeNotifier`, `InstrumentedNotifier`, and `LatencyRecorder` exposed unnecessary concrete surface. Lowered the concrete types and removed the standalone exported latency interface in favor of a package-private inline contract.
- `internal/infra/calendar`: `LarkCalendarProvider` concrete type was only constructed via `NewLarkCalendarProvider` and assigned to the domain port. Lowered the concrete type to package-private.

## Validation

- `go vet ./internal/infra/... ./internal/app/... ./internal/delivery/server/bootstrap ./internal/devops/process ./cmd/alex`
- `go test ./internal/infra/... ./internal/app/... ./internal/delivery/server/bootstrap ./internal/devops/process ./cmd/alex`
- `python3 skills/code-review/run.py review`
