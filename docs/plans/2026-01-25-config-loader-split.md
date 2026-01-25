# Plan: Split Config Loader (2026-01-25)

## Goal
- Break `internal/config/loader.go` into smaller, single-responsibility files without behavior changes.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Extract types/defaults/options from `loader.go` into dedicated files.
2. Extract file/env/override loading helpers and provider resolution helpers.
3. Ensure existing config tests remain green.
4. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Split runtime loader into focused files (types/options/defaults/env/file/providers/overrides).
