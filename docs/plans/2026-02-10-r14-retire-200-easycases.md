# Plan: R14 Retire 200 Always-Pass Cases

Owner: cklxx
Date: 2026-02-10
Worktree: `/Users/bytedance/code/elephant.ai-wt-r14-retire-200-20260210-001800`
Branch: `feat/eval-retire-200-easycases-r14-20260210-001800`

## Goal
Reduce benchmark saturation by removing exactly 200 cases that are consistently 100% passed, while preserving hard stress coverage and suite diagnosability.

## Steps
- [x] Load practices and memory context.
- [x] Locate latest foundation-suite run and identify 100%-pass collections/cases.
- [x] Produce deterministic prune manifest (`200` case IDs) with per-collection budgets.
- [x] Apply dataset edits and validate YAML integrity.
- [x] Re-run full suite and report x/x deltas.
- [x] Update analysis/research/plan docs.
- [x] Run full lint + full tests and commit.

## Prune Policy
- Prefer retiring cases from collections with `pass@1=100%` and `pass@5=100%` first.
- Keep at least 4 cases per collection to preserve dimensional signal.
- Do not remove all deliverable-contract cases from any collection.
- Keep all currently failing cases intact.
- Target exactly `200` removed scenarios.

## Progress Log
- 2026-02-10 00:18: Created fresh worktree from main and copied `.env`.
- 2026-02-10 00:20: Loaded engineering practices and active memory.
- 2026-02-10 00:27: Ran baseline suite (`457` cases), generated deterministic prune manifest, removed exactly `200` passed cases.
- 2026-02-10 00:30: Re-ran suite after prune (`257` cases): failures preserved (`14`), hard conflict profile unchanged.
- 2026-02-10 00:36: Updated analysis/research docs with R14 retirement deltas and prune-manifest reference.
- 2026-02-10 00:42: Completed full lint + full test runs (green).
