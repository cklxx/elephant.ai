# Plan: R12 Systematic Hard Benchmark Expansion

Owner: cklxx
Date: 2026-02-09
Worktree: `/Users/bytedance/code/elephant.ai-wt-r12-hardbench-20260209-233600`
Branch: `feat/eval-hardbench-r12-20260209-233600`

## Goal
Systematically expand the hard-only foundation suite using industry hardest benchmark families, with explicit dimension taxonomy and reproducible scoring.

## Steps
- [x] Load engineering practices and active memory.
- [x] Research hardest benchmark families and map them to routing-evaluable dimensions.
- [x] Add new benchmark-transfer case collections with specific and stable naming.
- [x] Integrate new collections into `foundation_eval_suite.yaml`.
- [x] Run full suite and produce x/x scoreboard + failure cluster summary.
- [ ] Update research/analysis docs with systematic classification and latest results.
- [ ] Run full lint + full tests.
- [ ] Commit incrementally and merge back to `main`.

## Benchmark Families Selected (R12)
- Terminal-Bench (terminal/ops agent reliability)
- MLE-Bench (ML engineering lifecycle)
- SWE-PolyBench (cross-language software engineering)
- GitTaskBench (real-repo maintenance workflows)
- OSWorld-G (grounded multimodal computer use)
- Humanity's Last Exam + FrontierMath (extreme deep reasoning with verification)

## Progress Log
- 2026-02-09 23:37: Created fresh worktree from main, copied `.env`.
- 2026-02-09 23:40: Loaded practices + long-term memory + latest summaries.
- 2026-02-09 23:43: Completed benchmark research shortlist and dimension mapping.
- 2026-02-09 23:50: Added 6 hard benchmark-transfer collections and integrated into `foundation_eval_suite.yaml`.
- 2026-02-09 23:52: Ran full suite (`25` collections / `387` cases): pass@1 `330/387`, pass@5 `380/387`; new collections introduce `5` failed cases by design.
