# 2026-02-09 — Foundation Hard Problem Expansion + Full Optimization (R4)

## Context
The suite had strong pass@5 saturation. We needed harder benchmark-like problems to preserve challenge pressure while keeping diagnostics actionable.

## What Changed
- Added 3 concrete hard collections (16 cases each):
  - sparse clue retrieval stress
  - stateful commitment boundary stress
  - reproducibility trace evidence stress
- Expanded full suite from `22` to `25` collections and from `608` to `656` cases.
- Ran post-expand baseline and fixed dominant conflict families via focused heuristic updates:
  - scheduler list/create/delete boundary
  - path-first (`find`) vs content-first (`search_file`)
  - artifact trace tools (`artifacts_list`/`artifacts_delete`/`artifacts_write`) vs unrelated tools
- Added regression assertions in `foundation_eval_test.go` for new conflict semantics.

## Impact
- Post-expand baseline: `pass@1=556/656`, `pass@5=648/656` (challenge pressure up, failures exposed).
- Final optimized: `pass@1=562/656`, `pass@5=656/656`, failed cases=`0`.
- Deliverable quality: `39/50` good, `11/50` bad.

## Learnings
- Benchmark-inspired hard cases should be represented as explicit conflict dimensions, not generic "hard" buckets.
- For scheduler and stateful commitments, read-vs-mutate semantics must be encoded with high-priority lexical gates.
- For content/path retrieval conflicts, both positive and negative constraints are required; one-sided boosts are insufficient.
