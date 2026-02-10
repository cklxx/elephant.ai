# 2026-02-10 — E2E Systematic Suite Rebuild and Delivery Review

Impact: Rebuilt a capability-layered end-to-end suite with harder industry benchmark transfer sets and produced a delivery-focused review that exposes real failure clusters instead of saturated easy-pass results.

## What changed
- Added 4 new hard benchmark-mapped datasets:
  - `foundation_eval_cases_industry_benchmark_webarena_verified_webops_hard.yaml`
  - `foundation_eval_cases_industry_benchmark_agentbench_multidomain_tooluse_hard.yaml`
  - `foundation_eval_cases_industry_benchmark_browsecomp_sparse_research_hard.yaml`
  - `foundation_eval_cases_industry_benchmark_agentlongbench_long_context_memory_hard.yaml`
- Added a new systematic E2E suite:
  - `foundation_eval_suite_e2e_systematic.yaml`
- Added research and analysis docs:
  - `docs/research/2026-02-10-e2e-systematic-benchmark-research.md`
  - `docs/analysis/2026-02-10-e2e-systematic-agent-delivery-review.md`

## Result
- Current baseline suite: `pass@1 224/269`, `pass@5 269/269`, `deliverable good 24/30`.
- New E2E suite: `pass@1 312/363`, `pass@5 361/363`, `failed 3`, `deliverable good 14/20`.
- The new suite successfully introduced hard pressure and surfaced real failure families:
  - `shell_exec => browser_screenshot`
  - `memory_search => browser_action`
  - `read_file => memory_get`

## Why this worked
- Benchmarks were selected by capability dimension and transferability, not by raw quantity.
- New cases emphasize implicit prompts and boundary conflicts to reduce lexical shortcut passing.
- Delivery review includes good/bad contract sampling, making output quality visible beyond pass rates.

## Validation
- `make fmt`
- `make test`
- `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml --output tmp/foundation-suite-r19-current-20260210-104937 --format markdown`
- `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite_e2e_systematic.yaml --output tmp/foundation-suite-r19-e2e-systematic-20260210-104937 --format markdown`
