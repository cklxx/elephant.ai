# 2026-02-09 — Pass@1 Systematic Optimization and Eval Skill

## Context
Foundation suite baseline was `398/505` on pass@1, with failures concentrated in repeated tool-conflict families rather than broad quality regressions.

## What Worked
- Used full miss inventory (`hit_rank > 1`, non-N/A) and clustered by `expected => top1`.
- Optimized routing with conflict-family convergence (Lark context/send vs upload, memory recall vs file search, approval gate vs clarify, shell vs execute, scheduler vs timer/artifact).
- Added report-level systematic section: Top1 conflict clusters with x/x counts and optimization actions.
- Added new hard collection `conflict-convergence-hard` (`24` cases), including sandbox-intent normalization onto executable tools (`shell_exec` / `execute_code`).
- Added reusable skill + report template for repeatable evaluation cycles.

## Outcome
- Suite scale increased to `529` cases across `20` collections.
- pass@1 improved to `446/529` (from `398/505` baseline).
- pass@5 remained full at `529/529`.
- Deliverable sampling stayed stable (`good 23/32`, `bad 9/32`) while hard coverage expanded.

## Reusable Rule
For pass@1 optimization, prioritize conflict-cluster convergence over single-case tuning; keep report structure fixed (`x/x`, conflict inventory, bad-case decomposition, good/bad samples) to preserve iteration quality.
