# 2026-02-09 Harder Cases Expansion + Easy pass@1 Case Retirement

**Status**: Completed  
**Branch**: `feat/harder-cases-20260209`  
**Updated**: 2026-02-09 12:10

## Goal
- Increase difficulty of foundation evaluation by adding more conflict-heavy and implicit hard cases.
- Retire cases that repeatedly achieve pass@1=100% across multiple historical runs.

## Workstreams
- [x] Mine historical suite artifacts and identify repeated pass@1-perfect cases (`>=3` runs, always hit_rank=1).
- [x] Replace easy cases in core collections with harder variants (same coverage dimension, stronger ambiguity/conflict).
- [x] Add more hard cases to challenge set with implicit tool cues and multi-constraint tasks.
- [x] Re-run full foundation suite and compare pass@1/pass@5 deltas.
- [x] Update evaluation docs and add experience record.
- [x] Run changed-scope tests and targeted validation.

## Acceptance Criteria
- At least 12 previously repeated pass@1-perfect cases are retired/replaced.
- Hard-case count increases materially (new scenarios added).
- Suite remains runnable and reports updated x/x metrics.
- New baseline reflects reduced easy-case inflation (pass@1 no longer trivially saturated by retired items).

## Execution Notes
- Retired/replaced repeated-pass@1 easy cases: `16`（tool_coverage/prompt_effectiveness/availability_recovery/complex_tasks 各 4）。
- Added new hard challenge cases: `12`（`challenge_hard_v2` 从 `37` 增至 `49`）。
- Suite rerun (`foundation-suite-20260209-040722`):
  - Collections: `17/19`
  - Cases: `501/505`
  - pass@1: `383/505`
  - pass@5: `502/505`
- New failed hard cases (top priorities):
  - `tool-artifacts-write-report-with-delivery-constraint`
  - `tool-list-workspace-directory-ambiguous-scope`
  - `challenge-memory-recall-before-offset-known`
  - `challenge-authoritative-sources-not-page-pull-yet`
