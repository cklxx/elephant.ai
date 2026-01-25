# 2026-01-25 - golangci-lint fmt blocked by import cycle/typecheck failures

## Error
- `make fmt` failed while running `golangci-lint run --fix ./...` with typecheck errors.
- `make vet` and `make test` failed with an import cycle reported as `alex/internal/logging` ↔ `alex/internal/observability`.
- Reported missing fields on `cmd/alex` `Container` (e.g., `SessionStore`, `AgentCoordinator`, `MCPRegistry`), suggesting build tags or build-only wiring mismatches.

## Impact
- Full lint step cannot complete, blocking the “fmt + lint” requirement in the engineering practices.

## Notes / Suspected Causes
- golangci-lint runs typecheck across `./...` and may ignore build-tagged wiring or use different build tags than the repo expects.
- The import cycle appears within the logging/observability packages and may be pre-existing.

## Remediation Ideas
- Align golangci-lint with the repo’s build tags (if any) or exclude `cmd/alex`/evaluation packages from lint if they require generated wiring.
- Validate whether `internal/logging` should depend on `internal/observability` (or vice-versa) to break the cycle.

## Resolution (This Run)
- Removed the observability adapter from `internal/logging` to break the logging/observability cycle; reran `make fmt`, `make vet`, and `make test` successfully.
