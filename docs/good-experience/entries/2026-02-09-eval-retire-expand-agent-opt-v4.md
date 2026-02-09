# 2026-02-09 — Eval Retire/Expand + Agent Optimization V4

## Context
Need to continue retiring easy evaluation prompts, add harder conflict-driven cases, and improve agent intent decomposition with implicit prompts.

## What Worked
- Replaced easy motivation cases with harder implicit-intent conflicts.
- Added a new hard collection `intent_decomposition_constraint_matrix` (20 cases) with explicit examination items for persistent top1 miss families.
- Added decomposition-aware routing heuristics, especially for:
  - consent-gate vs task mutation
  - message vs upload under explicit no-upload constraints
  - memory policy recall vs file search
  - baseline-read (`okr_read`) vs direct write
  - non-UI artifact/proof intents vs browser action

## Outcome
- Full suite expanded from `21` to `22` collections.
- Full suite scores:
  - `pass@1`: `505/581`
  - `pass@5`: `581/581`
  - failed cases: `1`
- Motivation standalone after retire/expand:
  - `pass@1`: `30/32`
  - `pass@5`: `32/32`

## Reusable Rule
When increasing difficulty, prefer implicit constraint-rich prompts (`before`, `without`, `rather than`, consent boundaries) over explicit tool-name prompts to improve real intent decomposition pressure.
