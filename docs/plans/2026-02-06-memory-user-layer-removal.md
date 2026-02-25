# Plan: Remove `<user-id>` Layer From Memory Paths (2026-02-06)

## Goal
- Remove the `/memory/<user-id>/` directory layer from runtime memory storage and prompt guidance.
- Keep memory behavior stable by migrating legacy layouts into the flat root layout.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.
- Loaded recent error/good summaries and `docs/memory/long-term.md` active memory.

## Scope
1. Flatten runtime memory storage path handling to `~/.alex/memory/`.
2. Update legacy migration to merge `users/<user-id>/` and `<user-id>/` into root.
3. Remove `<user-id>` path hints from identity prompts/templates.
4. Update affected tests and docs that define current runtime behavior.
5. Run lint/tests for touched modules and then full validation.

## Progress
- 2026-02-06 14:10: Created worktree branch `eli/memory-path-flatten` from `main` and copied `.env`.
- 2026-02-06 14:12: Reviewed engineering practices, memory summaries, and long-term memory constraints.
- 2026-02-06 14:16: Located impacted codepaths: `internal/infra/memory/*`, `internal/app/context/manager_memory.go`, `internal/app/context/manager_prompt.go`, and related tests/docs.
- 2026-02-06 14:20: Flattened runtime memory handling and migration logic to remove `<user-id>` directory usage; now merge legacy `users/<user-id>/` and `<user-id>/` into memory root.
- 2026-02-06 14:21: Updated identity prompt/template path hints to `~/.alex/memory/USER.md`.
- 2026-02-06 14:22: Updated impacted tests and reference docs for flattened memory layout.
- 2026-02-06 14:23: Refreshed `docs/memory/long-term.md` timestamp to hour precision on first daily load.
- 2026-02-06 14:25: Ran targeted package tests (memory/context/hooks/memory tools) successfully.
- 2026-02-06 14:27: Ran full lint + full test; both fail due pre-existing unrelated `internal/infra/tools/builtin/pathutil` and container API typecheck issues.

## Validation
- `CGO_ENABLED=0 go test ./internal/infra/memory -count=1` ✅
- `CGO_ENABLED=0 go test ./internal/app/context -count=1` ✅
- `CGO_ENABLED=0 go test ./internal/app/agent/hooks -count=1` ✅
- `CGO_ENABLED=0 go test ./internal/infra/tools/builtin/memory -count=1` ✅
- `./dev.sh lint` ❌ (pre-existing unrelated typecheck failures in `internal/infra/tools/builtin/pathutil` and `cmd/alex/*`)
- `./dev.sh test` ❌ (same pre-existing unrelated build failures)
