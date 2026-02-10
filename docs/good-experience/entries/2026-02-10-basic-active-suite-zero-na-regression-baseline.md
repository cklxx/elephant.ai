# 2026-02-10 — Basic Active Suite with Zero N/A Baseline

Impact: Rebuilt the basic evaluation suite to align with currently available tools/skills and eliminated invalid tool-call cases, restoring a stable `N/A=0` baseline for daily regression.

## What changed
- Pruned invalid tool-call cases from:
  - `foundation_eval_cases_tool_coverage.yaml`
  - `foundation_eval_cases_prompt_effectiveness.yaml`
- Rebuilt:
  - `foundation_eval_cases_proactivity.yaml` (active tools + skills only)
- Added:
  - `foundation_eval_suite_basic_active.yaml`

## Result
- Basic suite run:
  - Collections: `3/3`
  - Cases: `31/31`
  - N/A: `0`
  - pass@1: `27/31`
  - pass@5: `31/31`
  - failed: `0`

## Why this worked
- Used runtime inventory as source of truth and removed unavailable-tool expectations before semantic tuning.
- Kept base coverage compact and explicit, including `skills` and `channel` routes.
