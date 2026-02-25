# Plan: ANSI/tool display formatting extraction (Phase 1) (2026-01-26)

## Goal
- Move ANSI + tool display formatting out of `internal/agent/domain` with minimal behavior change.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Current scope notes
- `internal/agent/domain/formatter/formatter.go` owns ANSI sequences and tool display formatting.
- Output renderers (`internal/output`, `internal/server/http`) currently depend on the domain formatter.

## Plan
1. Create a presentation-side package for tool display formatting (`internal/presentation/formatter`).
2. Move `formatter.go` and its tests into the new package with identical behavior (no refactor beyond package rename/import paths).
3. Update imports and constructors in:
   - `internal/output/cli_renderer.go`
   - `internal/output/sse_renderer.go`
   - `internal/server/http/sse_handler.go`
4. Ensure gofmt and run targeted tests first (`go test ./internal/output/...`), then full `make fmt && make vet && make test`.
5. Commit in small steps if more than one logical change is needed.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed; analysis scoped to formatter move.
- 2026-01-26: Moved formatter into `internal/presentation/formatter` and updated imports.
- 2026-01-26: Validated with `make fmt`, `make vet`, `make test`.
