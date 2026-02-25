# 2026-02-09 — Complex Artifact Delivery Eval Layer

## Context
Existing foundation suite had broad routing coverage, but lacked explicit contract checks for file/artifact deliverables in complex tasks.

## What Worked
- Added optional `deliverable` contract in scenario schema (artifact/attachment/manifest/evidence/filetype requirements).
- Added `deliverable_check` at case-result level to score contract signal coverage from top tool matches.
- Added dedicated `complex_artifact_delivery` collection (32 cases) for complex tasks with concrete file outputs.
- Extended suite markdown report with `Deliverable Sampling Check` and good/bad sampled case tables.

## Outcome
- Foundation suite expanded to `19/19` collections and `493/493` pass@5.
- New collection result: `32/32` pass@5 with deliverable quality split `23/32` good and `9/32` bad.
- Report now includes explicit `x/x` deliverable metrics and sampled good/bad delivery diagnostics.

## Reusable Rule
For high-score suites that stop being challenging, add a separate delivery-quality axis (contract coverage + sampled checks) rather than only adding more routing cases.
