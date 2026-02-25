# Plan: Split Server Coordinator (2026-01-25)

## Goal
- Reduce `internal/server/app/server_coordinator.go` size by splitting into focused files with no behavior changes.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Split coordinator code by responsibility (core/types/options, async execution, snapshots, tasks, sessions).
2. Keep all public APIs and behavior identical.
3. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Split coordinator into async/snapshots/sessions/tasks files; ran fmt/vet/test.
