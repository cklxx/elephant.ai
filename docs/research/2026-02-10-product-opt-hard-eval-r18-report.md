# Product Optimization + Hard Eval Expansion R18 (2026-02-10)

## Objective
- Continue improving real product routing quality (not eval-only boost).
- Increase industry-grade hard evaluation coverage.
- Retire/reduce already-saturated easy-pass collections.

## Baseline
- Suite: `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- Run: `tmp/foundation-suite-r18-baseline-20260210-090429`
- Score:
  - total: `257`
  - pass@1: `225/257`
  - pass@5: `257/257`
  - deliverable good: `22/25`

## Dataset Changes (Harder + Systematic)
- Added hard collections:
  - `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_swebench_verified_hard_plus.yaml`
  - `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_cybench_security_ops_hard.yaml`
  - `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_tau2_long_horizon_enterprise_hard.yaml`
- Retired saturated collections from main suite:
  - `multi-step-orchestration`
  - `intent-decomposition-constraint-matrix`
  - `challenge-hard-v2`
  - `industry-benchmark-multiturn-enterprise-assistantbench-tau2`
  - `swebench-verified-readiness`
  - `industry-benchmark-coding-workflow`
- Hard replacement inside existing collection:
  - `foundation_eval_cases_industry_benchmark_terminal_bench_ops_hard.yaml`: replaced 4 easier cases with harder conflict-heavy ones (`scheduler_delete_job`, `write_attachment`, `web_fetch`, `web_search` boundaries).

### Case Volume Accounting
- Removed case volume: `6 collections * 4 = 24`
- Added new hard case volume: `3 collections * 12 = 36`
- Net increase: `+12`
- Final total: `269` cases

## Product Changes (Routing Quality)
- File/memory boundary:
  - `internal/infra/tools/builtin/memory/memory_get.go`
  - `internal/infra/tools/builtin/aliases/read_file.go`
  - `internal/infra/tools/builtin/sandbox/sandbox_file.go`
- Semantic body search vs topology discovery:
  - `internal/infra/tools/builtin/aliases/search_file.go`
  - `internal/infra/tools/builtin/sandbox/sandbox_file.go`
- Artifact inventory vs creation:
  - `internal/infra/tools/builtin/artifacts/artifacts.go`
- Terminal evidence vs screenshot routing:
  - `internal/infra/tools/builtin/aliases/shell_exec.go`
  - `internal/infra/tools/builtin/browser/screenshot.go`
  - `internal/infra/tools/builtin/sandbox/sandbox_browser.go`
- Regression test alignment:
  - `internal/infra/tools/builtin/browser/routing_descriptions_test.go`

## Final Result
- Run: `tmp/foundation-suite-r18-opt3-20260210-091128`
- Score:
  - total: `269`
  - pass@1: `224/269`
  - pass@5: `269/269`
  - deliverable good: `24/30`
  - failed cases: `0`

## Interpretation
- Hardness increased materially:
  - suite became larger and tougher while removing several saturated small collections;
  - pass@1 ratio dropped from `0.875` (225/257) to `0.833` (224/269), consistent with intentional hardening.
- Reliability maintained:
  - pass@5 remains `100%` and failed cases remain `0`.
- Remaining top conflict clusters:
  - `read_file => memory_get`
  - `search_file => find`
  - `artifacts_list => artifacts_write`

## Next Optimization Focus
1. Add stronger router-side dependency signal: prefer `memory_get` only when a memory path is already established in-context.
2. Penalize `find` when prompt explicitly requests body/content evidence (not filename/path).
3. Gate `artifacts_write` behind mutation verbs when intent is inventory/selection-only.
