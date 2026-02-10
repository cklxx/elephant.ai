# Plan: Remove Steward Persona + NEW_STATE Protocol

> Created: 2026-02-10
> Status: in-progress
> Trigger: User requested deletion of `/configs/context/personas/steward.yaml` related design/logic and removal of `<NEW_STATE>` tags.

## Goal & Success Criteria
- **Goal**: Remove runtime dependency on steward persona config and NEW_STATE tagged output protocol.
- **Done when**:
  - `configs/context/personas/steward.yaml` is removed.
  - No production code path depends on `<NEW_STATE>` tags.
  - Related tests are updated/removed and pass.
  - Lint and full test suite pass.
- **Non-goals**:
  - Full removal of all historical docs mentioning steward/NEW_STATE.
  - Re-architecting unrelated memory/context features.

## Current State
- Steward mode activation can be driven by persona `steward` and channel/session rules.
- ReAct runtime has dedicated NEW_STATE parser/filter/correction loop and related tests.
- Prompt builder emits NEW_STATE-specific instruction when steward mode is active.
- Persona config includes `configs/context/personas/steward.yaml`.

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | Remove steward persona config and references | `configs/context/personas/steward.yaml`, persona loading/tests | S | — |
| 2 | Remove NEW_STATE runtime parsing/filter/correction loop | `internal/domain/agent/react/*` | M | T1 |
| 3 | Remove/adjust steward-mode prompt and activation tests tied to persona | `internal/app/context/*`, `internal/app/agent/*_steward_*` | M | T2 |
| 4 | Run lint/test and fix regressions | repo-wide | M | T1-T3 |
| 5 | Code review workflow + commits + merge back main | `skills/code-review/*`, git history | S | T4 |

## Technical Design
- **Approach**:
  - Keep the stable core state/context interfaces, but remove the NEW_STATE extraction and stream filtering behavior from runtime iteration.
  - Remove persona-specific auto-enable path (`persona == steward`) while preserving explicit global/channel/session steward toggles if still used elsewhere.
  - Delete steward persona YAML and tests that require persona-based activation.
- **Alternatives rejected**:
  - Hard deleting all steward-related structs (`StewardState`) in one shot: too risky and broad for this request.
  - Keeping NEW_STATE parser but disabling usage: leaves dead design direction in runtime.
- **Key decisions**:
  - Remove runtime references to NEW_STATE protocol completely.
  - Preserve backward-safe session/context data fields unless proven unused by compile/test.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Hidden dependency on NEW_STATE parser/filter in tests | M | M | Run targeted then full tests; remove stale test cases together with code |
| Steward mode behavior regresses for non-persona toggles | M | M | Keep explicit config/session toggles; only remove persona-driven enable path |
| Large test impact from runtime changes | M | H | Iterate with focused package tests before full suite |

## Verification
- Run targeted tests for changed packages first.
- Run `./dev.sh lint` and full `go test ./...`.
- Rollback plan: revert commits in reverse order before merge.
