# Plan: Review builtin import fixes after shared/pathutil extraction (2026-01-26)

## Goal
- Verify `internal/tools/builtin` compiles after extracting `shared` and `pathutil` helpers; identify any missing/incorrect imports or references.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Current state (systematic view)
- `internal/tools/builtin` now has `shared/` and `pathutil/` subpackages plus updated tool implementations/tests.
- Many builtin files are already modified for new helper imports; need to confirm no lingering references to old helper locations.

## Plan
1. Scan `internal/tools/builtin` for `shared`/`pathutil` usage and import-path mismatches.
2. Compile builtin packages (`go test ./internal/tools/builtin/...`) to catch missing symbols.
3. Summarize any missing/incorrect imports or references that would fail to compile.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Scanned for `shared`/`pathutil` usage; imports aligned to `alex/internal/tools/builtin/{shared,pathutil}`.
- 2026-01-26: Ran `go test ./internal/tools/builtin/... -count=1`; compile succeeded.
- 2026-01-26: Ran `./dev.sh lint` (failed: unused functions in `cmd/alex/tui_bubbletea.go`).
- 2026-01-26: Ran `./dev.sh test` (passed; ld warnings observed during linking on macOS).
