# Plan: R13 SOTA Frontier Layered Expansion

Owner: cklxx
Date: 2026-02-10
Worktree: `/Users/bytedance/code/elephant.ai-wt-r13-sota-frontier-20260210-000500`
Branch: `feat/eval-sota-frontier-r13-20260210-000500`

## Goal
Push evaluation hardness to SOTA-difficult level with layered taxonomy and benchmark-backed collections.

## Layers
- L1 Core-Hard: SWE/conflict/orchestration/artifact stress.
- L2 Frontier-Hard: Terminal-Bench, MLE-Bench, SWE-PolyBench, GitTaskBench, OSWorld-G.
- L3 Research-Frontier-Hard: FrontierMath/HLE, RE-Bench, EXP-Bench, ARC-AGI-2.

## Steps
- [x] Research SOTA-hard benchmark families and map to routing-evaluable dimensions.
- [x] Add RE-Bench / EXP-Bench / ARC-AGI-2 collections.
- [x] Add PaperBench / MLRC-Bench / ALE-Bench collections.
- [x] Integrate into `foundation_eval_suite.yaml` with layered description.
- [x] Run full suite and capture x/x scoreboard and failure clusters.
- [x] Update analysis/research docs with layer taxonomy and benchmark sources.
- [x] Run full lint + tests.
- [ ] Commit and merge back to `main`.

## Progress Log
- 2026-02-10 00:05: Created fresh worktree from main and copied `.env`.
- 2026-02-10 00:09: Completed benchmark scan and selected frontier families.
- 2026-02-10 00:15: Added three research-frontier collections and executed full suite.
- 2026-02-10 00:19: Updated analysis/research docs with layered taxonomy and R13 x/x scoreboard.
- 2026-02-10 00:23: Completed full lint and full test runs (green).
- 2026-02-10 00:27: Added additional research-frontier collections (PaperBench/MLRC-Bench/ALE-Bench) and reran full suite (`31` collections, `457` cases).
