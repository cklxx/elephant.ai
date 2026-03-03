# Plan: Remove Orchestrator Gate/Replan Guard in ReAct Runtime

Date: 2026-03-03
Owner: Codex

## Goal
Remove gate-guard behavior in ReAct runtime so tool execution failures and mixed orchestrator tool batches are not blocked or auto-corrected.

## Scope
- `internal/domain/agent/react/runtime.go`
- `internal/domain/agent/react/runtime_test.go`

## Steps
- [x] Locate gate/replan prompt entry points and call sites.
- [x] Remove gate checks and correction injection from runtime think/observe flow.
- [x] Update tests to match no-gate behavior.
- [x] Run targeted tests.
- [x] Commit.
