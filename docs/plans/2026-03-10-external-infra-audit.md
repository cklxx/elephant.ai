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
- [ ] Commit in worktree and fast-forward merge to `main`.

## Notes

- `go test ./internal/infra/external/...` passed before changes.
- `golangci-lint run ./internal/infra/external/...` produced no package-level findings before changes.
- Current external adapters under `teamrun/`, `workspace/`, and `bridge/` are all referenced; no directory/package is currently proven removable.
- `internal/infra/external/registry.go` has repetitive bridge config assembly and retains an unused `ctrl` field.
- `internal/infra/external/workspace/manager.go` has a long `Merge` function with repeated conflict/result handling.
- Removed the unused `Registry.ctrl` field and extracted typed bridge config builders for Claude Code and CLI-style agents.
- Added registry tests covering enabled agent registration plus generic bridge timeout precedence.
- Split `workspace.Manager.Merge` into focused helpers for review mode, strategy execution, failure reporting, and final result population.
- Validation passed:
  - `go test ./internal/infra/external/...`
  - `golangci-lint run ./internal/infra/external/...`
  - `python3 skills/code-review/run.py review`
