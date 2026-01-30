# Plan: Eliminate LC_DYSYMTAB linker warnings on macOS (2026-01-30)

## Goal
- Remove `ld: warning: ... malformed LC_DYSYMTAB` during `./dev.sh test` on Darwin.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Identify source of warnings (external linker via cgo on darwin).
2. Update `./dev.sh test` to disable CGO by default on Darwin (override via env) so Go uses the pure Go implementation for Prometheus process collector.
3. Run full lint + tests to confirm warnings are gone.
4. Commit changes.

## Progress
- 2026-01-30: Plan created; engineering practices reviewed.
- 2026-01-30: Defaulted CGO to off for `./dev.sh test` on Darwin to avoid linker warnings.
- 2026-01-30: Removed stale `WithMemoryService` option from DI container builder (fixes lint/build).
- 2026-01-30: `./dev.sh lint` passed.
- 2026-01-30: `./dev.sh test` passed without LC_DYSYMTAB warnings.
