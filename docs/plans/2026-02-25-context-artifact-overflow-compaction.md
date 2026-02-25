# 2026-02-25 Context Artifact Compaction + Overflow Trigger

## Objective

Implement Manus-style restorable context compaction:

1. Write compacted intermediate context to one session-scoped file.
2. Insert a file-path placeholder back into the conversation.
3. Add a two-iteration cooldown to avoid repeated compaction churn.
4. Trigger the same compaction plan when provider/API returns context overflow errors.

## Scope

- `internal/domain/agent/react`:
  - Artifact compaction writer and placeholder injector.
  - Overflow error classifier and retry path integration.
  - Budget enforcement integration with cooldown.
- `internal/domain/agent/ports/agent`:
  - Extend `TaskState` with compaction cooldown/artifact state.
- `internal/app/context`:
  - Treat new placeholder message as synthetic compression content in history/meta processing.

## Execution Plan

- [completed] Add state fields for compaction lifecycle tracking.
- [completed] Add artifact compaction implementation (file write + placeholder).
- [completed] Integrate preflight threshold trigger into `enforceContextBudget`.
- [completed] Integrate overflow error trigger into think retry flow.
- [completed] Add/adjust tests for classifier, cooldown, placeholder normalization, and budget path.
- [completed] Run targeted package tests.

## Progress Log

- 2026-02-25: Ran required pre-work checklist on `main` (`git diff --stat`, `git log --oneline -10`) and confirmed unrelated dirty files would be left untouched.
- 2026-02-25: Reviewed engineering/documentation guides and loaded latest error/good experience entries.
- 2026-02-25: Implemented `context_artifact_compaction.go`:
  - session-scoped artifact file output
  - `[CTX_PLACEHOLDER ...]` checkpoint insertion
  - sequence tracking + two-turn cooldown state updates
- 2026-02-25: Implemented `context_overflow_classifier.go` with provider-agnostic normalized phrase/code matching and explicit false-positive exclusions.
- 2026-02-25: Updated runtime think retry path to apply artifact compaction on overflow errors before retry.
- 2026-02-25: Updated preflight budget path to run artifact compaction first (cooldown-aware), then legacy fallback.
- 2026-02-25: Added and updated tests in:
  - `internal/domain/agent/react/context_artifact_compaction_test.go`
  - `internal/domain/agent/react/engine_internal_test.go`
  - `internal/domain/agent/react/message_normalization_test.go`
  - `internal/domain/agent/ports/agent/task_state_snapshot_test.go`
- 2026-02-25: Verified with:
  - `go test ./internal/domain/agent/react ./internal/domain/agent/ports/agent ./internal/app/context -count=1`
