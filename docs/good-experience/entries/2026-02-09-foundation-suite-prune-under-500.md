# 2026-02-09 — Foundation Suite Prune Under 500

## Context
Suite scale had grown to 656 cases, and user explicitly required shrinkage under 500 with retirement (not only expansion).

## What Changed
- Applied explicit per-collection caps to reduce total suite volume while preserving all 25 dimensions.
- Retired 157 scenarios across large collections (`challenge`, `tool_coverage`, `complex_artifact_delivery`, etc.).
- Kept hard stress collections intact to maintain difficulty diversity.

## Impact
- Case volume: `656 -> 499`.
- pass@5 remained full: `656/656 -> 499/499`.
- pass@1 became `426/499` (85.4%), preserving challenge pressure in a smaller suite.
- Deliverable quality became `26/32` good.

## Learnings
- Enforce hard suite-size budgets with per-collection caps; do not rely on ad-hoc expansion.
- Report `added / retired / net` every round to avoid silent size drift.
