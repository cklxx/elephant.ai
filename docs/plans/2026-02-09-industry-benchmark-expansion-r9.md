# Plan: Industry Benchmark Expansion R9

Owner: cklxx
Date: 2026-02-09
Worktree: `/Users/bytedance/code/elephant.ai-wt-eval-industry-r9-20260209`
Branch: `feat/eval-industry-r9-20260209`

## Goal
Increase evaluation difficulty using real industry benchmark transfer cases, wire them into the foundation suite, and produce a scored report with clear `x/x` coverage.

## Scope
- Add harder transfer datasets with explicit benchmark provenance and concrete capability dimensions.
- Integrate new collections into `foundation_eval_suite.yaml`.
- Run full suite scoring, plus full lint/tests, then document results.

## Active Memory (Top)
1. Start from `main` in a fresh worktree, copy `.env`, then merge back and clean worktree.
2. Non-trivial work requires a plan file with progress updates.
3. Keep evaluation config in YAML files.
4. Full lint + tests are required before delivery.
5. Track evaluation with fixed report structure and explicit `x/x` scoreboard.

## Steps
- [x] Confirm branch/worktree and baseline suite status.
- [x] Research current benchmark sources (official pages + papers).
- [x] Add new hard benchmark transfer datasets.
- [x] Add collections to foundation suite.
- [x] Run full evaluation and collect pass@1/pass@5 and conflict clusters.
- [x] Update analysis/report docs with new coverage and outcomes.
- [ ] Run full lint/tests.
- [ ] Commit incrementally and merge back to `main`.

## Progress Log
- 2026-02-09 20:10: Validated worktree/branch and current dataset inventory.
- 2026-02-09 20:14: Completed benchmark-source research pass for latest public benchmarks and transfer targets.
- 2026-02-09 20:21: Added GAIA transfer dataset and integrated additional industry-benchmark collections into main suite.
- 2026-02-09 20:24: Added three hard benchmark-transfer datasets: LiveCodeBench/SWE-Lancer, AssistantBench/τ2, NoLiMa/LongMemEval/BABILong.
- 2026-02-09 20:24: Ran full suite baseline after expansion (`tmp/foundation-suite-r9-industry-20260209-202122`) and extracted top conflict clusters.
- 2026-02-09 20:24: Applied conflict-cluster heuristic router improvements and added regression tests.
- 2026-02-09 20:24: Converted main suite to hard-only by retiring saturated easy collections.
- 2026-02-09 20:24: Re-ran hard-only optimized suite (`tmp/foundation-suite-r9-hardonly-optimized-20260209-202433`) and updated report/analysis docs.
