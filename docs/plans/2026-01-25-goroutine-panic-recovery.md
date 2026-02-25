# Plan: Goroutine Panic Recovery Coverage (2026-01-25)

## Goal
- Ensure all production goroutines launched via `go` are guarded by panic recovery via `internal/async` helper.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Replace unguarded `go` spawns with `async.Go` where a logger is available.
2. Add unit tests for `internal/async` panic recovery behavior.
3. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Switched remaining unguarded goroutines to async recovery helper.
- 2026-01-25: Added async panic recovery tests.
