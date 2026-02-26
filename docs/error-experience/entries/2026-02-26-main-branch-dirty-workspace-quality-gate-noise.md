# Main-branch dirty workspace caused unrelated quality-gate failures during maintainability pass

**Date:** 2026-02-26
**Severity:** Medium
**Category:** Validation / Workspace Hygiene

## What happened

A non-web maintainability refactor pass was validated with targeted package tests successfully, but full `./scripts/pre-push.sh` failed on unrelated existing workspace issues.

## Root cause

1. `main` branch had in-flight uncommitted changes outside this refactor scope.
2. Existing changes introduced interface drift (`agent.ContextManager` now requires `BuildSummaryOnly`) without updating multiple test stubs.
3. Existing changes also triggered env-usage guard in `internal/infra/llm/thinking.go`.

## Fix / mitigation

1. Keep mandatory pre-work checklist and explicitly flag dirty unrelated diffs before coding.
2. Prefer worktree isolation for broad cleanup tasks to avoid mixed failure signals.
3. If full gate fails due unrelated dirty diffs, preserve local-scope verification and report blocker explicitly with file-level evidence.

## Validation

- `./scripts/pre-push.sh` (failed due unrelated pre-existing files)
- Scoped checks for touched packages passed.

## Lessons

- Full-repo validation on `main` is noisy when unrelated local edits are present.
- For system-wide cleanup tasks, separate branch/worktree isolation is as important as test coverage.

## Metadata
- id: err-2026-02-26-main-dirty-workspace-quality-noise
- tags: [error, validation, workspace, pre-push, process]
- links:
  - docs/plans/2026-02-26-non-web-systematic-maintainability-optimization.md
  - internal/infra/llm/thinking.go
  - internal/app/agent/preparation/analysis_routing_test.go
  - internal/app/agent/coordinator/coordinator_acceptance_test.go
