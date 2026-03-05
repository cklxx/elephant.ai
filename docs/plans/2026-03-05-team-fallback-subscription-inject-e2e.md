# 2026-03-05 Team Fallback + Subscription + Inject E2E

## Goal

Fix teams execution behavior so it is independent from manual provider pinning:

1. External role execution must apply `fallback_clis` automatically.
2. `alex team run` must inherit pinned CLI subscription selection.
3. Injection path must be verified by an end-to-end test (not only unit tests).

## Plan

- [x] Implement fallback chain in external registry (`primary + fallback_clis`).
- [x] Add tests for fallback chain normalization and execution behavior.
- [x] Wire pinned CLI subscription into team-run context.
- [x] Add regression test for team-run context selection propagation.
- [x] Add E2E injection test: `run_tasks(wait=false, mode=team)` + `reply_agent(message=...)`.
- [x] Run targeted test suites for changed modules.

## Validation

- `go test ./internal/infra/external`
- `go test ./cmd/alex -run 'TestParseTeamRunOptions|TestBuildTeamRunContextAppliesPinnedSelection|TestHandleTeamRoutes'`
- `go test ./internal/infra/tools/builtin/orchestration -run 'TestRunTasks|TestReplyAgent'`
- `go test ./internal/infra/integration -run TestAgentTeamsInjectE2E_RunTasksAndReplyAgent`
