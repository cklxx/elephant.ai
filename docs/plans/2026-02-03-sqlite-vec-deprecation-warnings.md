# Plan: Silence sqlite-vec Deprecation Warnings on macOS

Owner: cklxx
Date: 2026-02-03

## Goal
Eliminate sqlite3_auto_extension deprecation warnings during `make build` on macOS while preserving sqlite-vec behavior.

## Scope
- Add a darwin-only CGO flag injection for deprecated-declaration warnings.
- Keep sqlite-vec usage and memory index logic unchanged.

## Non-Goals
- Replace sqlite-vec or change vector indexing behavior.
- Rework CGO auto-detection or build pipelines outside the Go wrapper.

## Plan of Work
1) Locate build entry points that compile sqlite-vec during `make build`.
2) Add darwin-only `-Wno-deprecated-declarations` injection without clobbering existing `CGO_CFLAGS`.
3) Verify `make build` no longer emits the warnings.
4) Run full lint + tests.

## Test Plan
- `make build`
- `./dev.sh lint`
- `./dev.sh test`

## Progress
- [x] Locate build entry points
- [x] Add darwin-only CFLAGS injection
- [x] Verify build warning-free
- [x] Lint + tests
