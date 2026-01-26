# Plan: Move builtin artifact tools into artifacts package (2026-01-26)

## Goal
- Relocate artifact-related builtin tool implementations and tests under `internal/tools/builtin/artifacts` with `package artifacts`, updating imports only as needed.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Move the specified artifact-related files and tests into `internal/tools/builtin/artifacts/`.
2. Update package declarations to `package artifacts` and fix imports (including shared/pathutil) without changing registry wiring or shared helpers.
3. Ensure tests remain in the same package and compile.
4. Run `make fmt` and `make test` for full lint and test coverage.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed; move scoped to builtin artifact tools.
