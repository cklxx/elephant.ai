# 2026-02-09 — Foundation Retire/Expand/Optimize Round 3

## Context
Foundation suite had become relatively saturated in several dimensions. We needed to increase challenge pressure with implicit/conflict-heavy tasks, then systematically recover routing quality.

## What Changed
- Retired multiple easy/explicit tool intents and replaced them with implicit disambiguation in tool coverage.
- Expanded hard collections with concrete examination items:
  - intent decomposition constraint matrix: `20 -> 32`
  - challenge hard v2: `49 -> 58`
  - complex artifact delivery: `32 -> 38`
- Added routing convergence rules for scheduler semantics, message-vs-upload, fixed-url fetch, artifact-vs-browser-action, and path-first retrieval.
- Added a regression case for motivation-progress artifact routing in `foundation_eval_test.go`.

## Impact
- Case volume increased from `581` to `608`.
- After hardening only: `pass@1=516/608`, `pass@5=606/608` (challenge pressure successfully increased).
- After optimization: `pass@1=526/608`, `pass@5=608/608`, failed cases=`0`.
- Deliverable quality improved from `31/44` to `33/44` good.

## Learnings
- Hard-case expansion should be followed by an explicit pass@5 recovery phase before top1 polishing.
- Scheduler/timer and message/upload conflicts are high-leverage families; small rule changes can close multiple failures at once.
- Token normalization side effects (e.g., stemming) can silently reduce heuristic trigger rates; add intent-level regression tests, not only token-set tests.
