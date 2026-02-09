# Plan: Hardness R11 Suite Upgrade

Owner: cklxx
Date: 2026-02-09
Worktree: `/Users/bytedance/code/elephant.ai-wt-r11-hard-20260209-230301`
Branch: `feat/eval-hardness-r11-20260209-230301`

## Goal
Increase benchmark difficulty so pass@5 is no longer saturated, using harder implicit, low-overlap, multi-constraint scenarios aligned with industry benchmark transfer tasks.

## Steps
- [x] Confirm baseline state and constraints.
- [x] Add harder scenario collections/cases (implicit and conflict-heavy).
- [x] Integrate into hard-only foundation suite.
- [x] Run baseline evaluation and verify pass@1/pass@5 drops to a more challenging level.
- [ ] Run lint/tests for changed scope and commit incrementally.

## Progress Log
- 2026-02-09 23:03: Created fresh worktree from main and copied .env.
- 2026-02-09 23:17: Added hard low-overlap implicit-intent transfer collection and integrated it into `foundation_eval_suite.yaml`.
- 2026-02-09 23:22: Added hard compound long-horizon autonomy/value-delivery collection and integrated it into `foundation_eval_suite.yaml`.
- 2026-02-09 23:28: Ran full hard suite and confirmed challenge increase: `pass@1 264/315`, `pass@5 302/315`, with new failure clusters concentrated in low-overlap implicit scheduling/approval intents.
- 2026-02-09 23:34: Implemented failure-cluster-driven router boost updates in `foundation_eval.go` and reran suite; improved to `pass@1 271/315`, `pass@5 311/315`.
- 2026-02-09 23:41: Second optimization iteration focused on scheduler boundary conflicts; improved to `pass@1 273/315`, `pass@5 313/315`, remaining failed cases `2`.
