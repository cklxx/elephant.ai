# Plan: Move file ops builtins to domain package (2026-01-26)

## Goal
- Move file operation tools into `internal/tools/builtin/fileops` with package `fileops` while keeping registry wiring untouched.

## Constraints
- Do not touch registry wiring or shared helpers.
- No commits.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Current state (systematic view)
- File ops tools live in `internal/tools/builtin` (`file_read.go`, `file_write.go`, `file_edit.go`, `list_files.go` plus tests).
- Shared helpers are already isolated under `internal/tools/builtin/shared` and `internal/tools/builtin/pathutil`.
- Registry wiring imports `internal/tools/builtin` and registers `NewFileRead`, `NewFileWrite`, `NewFileEdit`.

## Plan
1. Move file ops sources/tests into `internal/tools/builtin/fileops/` via `git mv`.
2. Update package declarations to `package fileops` and fix imports for `shared`/`pathutil`.
3. Run `gofmt` on moved files.
4. Run full lint + test suite; capture failures (expected if registry wiring still points at `builtin`).
5. Summarize changes and issues.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Moved file ops sources/tests into `internal/tools/builtin/fileops`, updated package names/imports, ran `gofmt`.
- 2026-01-26: Ran `make fmt` and `make test`; both failed due to pre-existing build errors in builtin/execution/sandbox/artifacts (see run output).
