# Plan: Hard-Only Eval R10 Product Optimization

Owner: cklxx
Date: 2026-02-09
Worktree: `/Users/bytedance/code/elephant.ai-wt-r10-opt-20260209-212241`
Branch: `feat/eval-hardonly-r10-20260209-212241`

## Goal
Improve real product routing quality against the hard-only foundation suite by fixing high-frequency failure clusters and raising pass@1 while preserving pass@5 and deliverable quality.

## Scope
- Run hard-only suite baseline and extract top1 conflict clusters.
- Implement targeted heuristic/router improvements in product code.
- Add regression tests for newly fixed conflict patterns.
- Re-run suite and report before/after metrics with x/x scoreboard.

## Steps
- [x] Load practices + active memory context.
- [x] Run hard-only baseline.
- [x] Implement conflict-driven routing improvements.
- [x] Add/adjust regression tests (TDD for modified logic).
- [x] Re-run hard-only suite and compare deltas.
- [x] Run lint/tests and commit incremental changes.

## Progress Log
- 2026-02-09 21:23: Created fresh worktree from `main`, copied `.env`, loaded engineering practices and long-term memory context.
- 2026-02-09 21:23: Baseline run complete: `tmp/foundation-suite-r10-baseline-20260209-212310` (`pass@1 224/269`, `pass@5 267/269`).
- 2026-02-09 21:28: Implemented conflict-cluster heuristic upgrades in `evaluation/agent_eval/foundation_eval.go` and expanded regression assertions in `evaluation/agent_eval/foundation_eval_test.go`.
- 2026-02-09 21:29: Optimized run #1: `tmp/foundation-suite-r10-optimized-20260209-212840` (`pass@1 232/269`, `pass@5 269/269`).
- 2026-02-09 21:29: Optimized run #2: `tmp/foundation-suite-r10-optimized2-20260209-212950` (`pass@1 235/269`, `pass@5 269/269`).
- 2026-02-09 21:30: `./dev.sh lint` passed; `./dev.sh test` hit existing race failure in `internal/delivery/channels/lark` (`TestHandleMessageReprocessesInFlightFollowUpWhenAwaitingInput`), unchanged by this scope.
