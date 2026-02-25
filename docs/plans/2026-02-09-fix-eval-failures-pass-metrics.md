# 2026-02-09 Fix Eval Failures (pass@1/pass@5)

**Status**: In Progress  
**Branch**: `fix/eval-failures-20260209`  
**Updated**: 2026-02-09

## Goal
Reduce current foundation-suite failing cases (`7`) by targeted routing heuristic improvements, based on latest pass@1/pass@5 report.

## Baseline (latest)
- Suite: `454/461` (failed `7`)
- pass@1: `337/461`
- pass@5: `454/461`
- Main failure patterns:
  - user-gated intents not routing to `request_user`
  - memory evidence-open intents not routing to `memory_get`
  - fast regex repo scan intents not routing to `ripgrep`
  - a few ranked-below-top5 cases (`request_user`, `ripgrep`)

## Steps
- [x] Confirm baseline failure cases and root patterns from suite JSON report.
- [x] Adjust `heuristicIntentBoost` rules for disambiguation between close tool pairs.
- [x] Add/adjust tests for the identified failure signatures.
- [x] Run focused tests (`evaluation/agent_eval`, `cmd/alex`).
- [x] Run full suite and verify improved pass ratio and failure breakdown.
- [x] Run full repo tests/lint per repo practice.
- [x] Update docs/records and commit in incremental commits.

## Result
- Final suite: `461/461`, failed `0`.
- pass@1: `369/461` (`+32` vs baseline).
- pass@5: `461/461` (`+7` vs baseline).
