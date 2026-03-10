# 2026-03-10 External Infra Audit

## Goal

Audit `internal/infra/external/` for:

- dead code
- unused adapters
- stale external API wrappers
- overly long functions that can be simplified safely

## Checklist

- [x] Create worktree and marker.
- [x] Inspect package structure, references, and current test/lint baseline.
- [x] Remove verified dead code and simplify oversized functions.
- [x] Run validation and review.
- [x] Commit in worktree and fast-forward merge to `main`.

## Notes

- `go test ./internal/infra/external/...` passed before changes.
- `golangci-lint run ./internal/infra/external/...` produced no package-level findings before changes.
- Current external adapters under `teamrun/`, `workspace/`, and `bridge/` are all referenced; no directory/package is currently proven removable.
- `internal/infra/external/registry.go` had repetitive bridge config assembly (resolved: typed builders extracted).
- `internal/infra/external/workspace/manager.go` Merge decomposed into focused helpers.
- `bridge.Executor.ctrl` field was stored but never read (closure captures ctor parameter directly) — removed.
- Registry tests cover enabled agent registration plus generic bridge timeout precedence.
- Validation passed:
  - `go test ./internal/infra/external/...`
  - `golangci-lint run ./internal/infra/external/...`
  - `python3 skills/code-review/run.py review`
