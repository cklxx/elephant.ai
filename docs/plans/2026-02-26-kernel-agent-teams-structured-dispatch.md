# 2026-02-26 Kernel Agent Teams Structured Dispatch Implementation

Created: 2026-02-26
Status: Completed
Owner: Codex

## Context

Kernel currently relies on free-form prompts for `run_tasks` usage. Agent teams are available, but kernel lacks a structured dispatch model that can explicitly represent and validate team runs.

## Goals

1. Add structured kernel dispatch support for team execution.
2. Enable LLM planner to emit explicit `team` decisions with strict constraints.
3. Enforce single-team-per-cycle execution policy for reliability.
4. Preserve static planner fallback and current kernel autonomy guarantees.

## Steps

- [x] Extend kernel domain dispatch types with `kind` and team payload.
- [x] Update file-backed kernel store to persist new fields.
- [x] Implement kernel team dispatch execution path in coordinator executor.
- [x] Upgrade LLM planner schema/prompt/parsing to support team decisions.
- [x] Wire allowed team templates into kernel planner bootstrap.
- [x] Add/update unit and integration-adjacent tests.
- [x] Run focused test suites and code review skill.
- [ ] Commit incremental changes.

## Verification Commands

- `go test ./internal/app/agent/kernel/... -count=1`
- `go test ./internal/infra/tools/builtin/orchestration ./internal/domain/agent/taskfile -count=1`
- `go test ./internal/shared/config -run 'ExternalAgentTeams|external_agents' -count=1`
- `go test -tags=integration ./internal/infra/integration -run 'TestAgentTeamsKimiInjectE2E_ParallelTemplate' -count=1`

## Progress Log

- 2026-02-26 17:20: Added kernel domain dispatch kind (`agent`/`team`) and structured team payload.
- 2026-02-26 17:27: Added coordinator team execution path with deterministic `run_tasks` prompt construction.
- 2026-02-26 17:35: Extended LLM planner to emit/validate team decisions with template allowlist and max-team-per-cycle limits.
- 2026-02-26 17:42: Wired runtime-configured team template names into kernel planner bootstrap.
- 2026-02-26 17:50: Added unit coverage for planner team decisions, coordinator team execution, and engine team dispatch routing.
- 2026-02-26 17:56: Ran focused test suites; all green.
- 2026-02-26 18:00: Ran `skills/code-review` review pass; no blocking P0/P1 findings surfaced.
