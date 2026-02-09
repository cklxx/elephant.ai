# 2026-02-09 Pass@1 Failure Systematic Optimization and Eval Skill

## Context
- User requirement: fully inventory all remaining pass@1 failures, optimize them systematically (not case-by-case hacks), expand harder evaluation coverage, and standardize a reusable report format.
- Additional requirement: treat unavailable tools as `N/A` (excluded from fail), but for available tools, prioritize improving real tool usability/routeability instead of excluding failures.
- Repo conventions: non-trivial task requires plan tracking, full lint/tests before delivery, and report/template accumulation as reusable skill.

## Goals
1. Reduce foundation-suite pass@1 misses by conflict-cluster-driven routing improvements.
2. Expand datasets with harder, conflict-heavy/systematic cases across coverage layers.
3. Add stable report sections: x/x ratios, top1 failure cluster inventory, optimization actions, and good/bad sampled deliverable checks.
4. Persist the report workflow into a reusable local skill under `skills/`.

## Scope
- In scope:
  - `evaluation/agent_eval/*` routing heuristics, report generation, tests.
  - `evaluation/agent_eval/datasets/*.yaml` collection expansion.
  - `evaluation/agent_eval/README.md` report format and run instructions.
  - New evaluation skill docs under `skills/`.
- Out of scope:
  - Runtime/tool executor behavior outside eval framework.
  - Unrelated product features.

## Implementation Plan
1. Baseline and failure clustering
   - Run full foundation suite and export pass@1 misses.
   - Cluster by `expected_tool <- top1_tool`, category, and failure reason signature.
   - Identify top conflict families for systematic fixes.
2. Heuristic and token normalization upgrades
   - Introduce grouped conflict penalties/boosts for top families.
   - Add token aliases for high-frequency ambiguity patterns.
   - Add explicit convergence behavior for sandbox-like intents (route to execution tools; avoid separate sandbox pseudo-target).
3. Report format hardening
   - Add “Top1 failure cluster breakdown” section with count x/x and rate.
   - Add per-cluster optimization recommendations.
   - Keep and strengthen sampled good/bad deliverable inspection section.
4. Dataset expansion and hardening
   - Add new harder cases to targeted collections (especially where pass@1 remains weak).
   - Preserve N/A semantics for unavailable-tool expectations.
5. Verification
   - Unit tests for new routing conflict families and report sections.
   - Full lint + tests.
   - Full foundation-suite run; capture before/after metrics.
6. Knowledge accumulation
   - Add/refresh reusable evaluation skill with report template and workflow.
   - Update long-term memory timestamp and add durable rule if needed.

## Progress
- [x] Loaded engineering practices and long-term memory.
- [x] Loaded latest error/good summaries.
- [x] Confirmed existing worktree and baseline failure artifact location.
- [x] Run fresh baseline in this worktree and build conflict cluster table.
- [x] Implement routing/report/test updates.
- [x] Expand datasets with harder cases.
- [x] Run lint/tests and suite; collect final report.
- [x] Add evaluation skill and memory update.
