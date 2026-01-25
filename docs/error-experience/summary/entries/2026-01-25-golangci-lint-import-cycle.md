# 2026-01-25 - golangci-lint fmt blocked by import cycle/typecheck failures

- Summary: `make fmt`, `make vet`, and `make test` failed because golangci-lint/go vet/go test hit an `internal/logging` ↔ `internal/observability` import cycle plus typecheck errors in `cmd/alex` `Container` fields (e.g., missing `SessionStore`, `AgentCoordinator`, `MCPRegistry`).
- Remediation: align lint build tags or exclude build-only packages; break the logging/observability cycle or adjust dependencies.
- Resolution: removed the logging → observability adapter to break the cycle; fmt/vet/test then passed.
