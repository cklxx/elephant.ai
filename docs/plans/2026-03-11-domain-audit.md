status: completed

## Scope

- Audit `internal/domain/**` for infrastructure imports that violate domain-layer boundaries.
- Find dead code and exported symbols that are unused outside their own package.
- Apply minimal refactors within `internal/domain/**`.
- Validate with `go vet` and focused `go test`, then commit.

## Plan

1. Scan `internal/domain/**` imports and confirm whether any non-test files pull in infrastructure packages.
2. Identify dead code and package-local-only exported symbols with reference scans.
3. Lower or delete safe targets, then run `go vet`, `go test`, and code review before commit.

## Findings

- `internal/domain/**` non-test code has no direct `os`, `os/exec`, or `net/http` imports; the only filesystem-adjacent standard import is `io/fs` in a port contract for file-mode bits.
- Lowered package-local-only export surface in `domain` internals: `react.contextBudgetStatus`, `ports.compressionPlan`, and `presets.filteredToolRegistry`.
- Removed the redundant exported alias `domain.EventKind`; `Event.Kind` and `NewEvent` now use plain `string`.
- Removed the now-obsolete compile-time assertion in `preset_resolver_test.go` after `FilteredToolRegistry` stopped being part of the exported API.

## Validation

- `go vet ./internal/domain/... ./internal/app/... ./internal/infra/... ./internal/delivery/... ./evaluation/... ./cmd/alex`
- `go test ./internal/domain/... ./internal/app/... ./internal/infra/... ./internal/delivery/... ./evaluation/... ./cmd/alex`
- `python3 skills/code-review/run.py review`
