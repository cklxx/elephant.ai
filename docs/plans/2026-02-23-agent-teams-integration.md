# 2026-02-23 Agent Teams Integration Plan

Created: 2026-02-23
Status: In Progress
Owner: Codex

## Background

The repository already contains `team_dispatch` orchestration logic and DI wiring, but runtime config loading currently does not persist `runtime.external_agents.teams` into `RuntimeConfig.ExternalAgents.Teams`. This prevents configured teams from being available at runtime.

Goal: enable production-ready `agent teams` using existing architecture, with explicit support for `file-based + cli full access` operation via configuration and documented best practices.

## Scope

1. Wire `external_agents.teams` through config file parsing and runtime loader.
2. Add tests proving team config is loaded and env-expanded correctly.
3. Normalize team config values for safer runtime behavior.
4. Update reference docs for config/tooling and include a practical team example.
5. Add a research note comparing mainstream agent-team implementations and tradeoffs.

## Implementation Steps

- [x] Analyze existing team_dispatch wiring and identify integration gap.
- [x] Add `teams` schema in `internal/shared/config/file_config.go`.
- [x] Apply `teams` into runtime config in `internal/shared/config/runtime_file_loader.go`.
- [x] Expand env placeholders for team role config values in loader.
- [x] Normalize team-related values in `internal/shared/config/load.go`.
- [x] Add/extend tests under `internal/shared/config/*_test.go`.
- [x] Update docs (`docs/reference/CONFIG.md`, `docs/reference/TOOLS.md`).
- [x] Add web-research-backed comparison report under `docs/research/`.
- [x] Run lint/tests and resolve failures.
- [x] Run mandatory code review workflow before commit.
- [x] Commit in incremental steps and merge back to `main`.

## Progress Log

- 2026-02-23 22:35: Completed repo scan and architecture trace; confirmed `team_dispatch` exists, runtime load path for `teams` is missing.
- 2026-02-23 22:52: Implemented config loader wiring for `external_agents.teams` (schema + apply + env expansion + normalization) and added targeted tests.
- 2026-02-23 22:58: Updated config/tool docs and added web research comparison note with primary-source references.
- 2026-02-23 23:08: Rebased work branch to latest `main`, reran full lint + full `go test ./...` and `make check-arch` successfully.
- 2026-02-23 23:12: Completed mandatory code review checklist (SOLID/security/quality/removal); no blocking findings.
- 2026-02-23 23:18: Split into incremental commits, fast-forward merged into `main`, pruned worktree metadata, and removed temporary branch.
