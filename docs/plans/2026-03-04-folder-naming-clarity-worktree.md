# 2026-03-04 Folder Naming Clarity (Worktree)

Status: completed
Owner: codex
Updated: 2026-03-04

## Goal

Rename ambiguous folder names to semantically explicit names with minimal behavioral risk, then merge back to `main` via fast-forward.

## Scope

1. Rename `internal/domain/materials` to `internal/domain/materialregistry`.
2. Rename `internal/infra/task` to `internal/infra/taskadapters`.
3. Update all import paths and package names affected by folder renames.
4. Update canonical scripts/docs references where they are directly used for code indexing.

## Progress

- [x] Baseline audit on `main` + isolate work in a dedicated worktree branch.
- [x] Identify rename targets and impact surface.
- [x] Execute folder renames and reference updates.
- [x] Run lint/tests and mandatory code review.
- [x] Commit in worktree and fast-forward merge to `main`.

## Verification

- `go test ./...` (fails on pre-existing baseline compile issue in `internal/app/di/container_builder.go`)
- `./scripts/run-golangci-lint.sh run ./...` (fails on same pre-existing baseline issue)
- Targeted packages for this rename pass:
  - `go test ./internal/domain/materialregistry ./internal/infra/taskadapters ./internal/domain/agent/react ./internal/app/agent/coordinator` (pass)
  - `go test ./internal/delivery/server/bootstrap` (blocked by same baseline `internal/app/di` issue)
- `python3 skills/code-review/run.py '{"action":"review"}'` (executed)
