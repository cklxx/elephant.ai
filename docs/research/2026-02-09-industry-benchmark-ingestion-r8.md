# 2026-02-09 Industry Benchmark Ingestion (R8)

## Objective
- Replace easy synthetic routing items with harder, industry benchmark-transferred tasks.
- Keep evaluation aligned with real agent benchmarks instead of non-LLM-basic tasks.

## Source Benchmarks (Primary Sources)
- SWE-bench / SWE-bench Verified:
  - https://www.swebench.com/
  - https://arxiv.org/abs/2310.06770
- τ-bench:
  - https://arxiv.org/abs/2406.12045
- AgentBench:
  - https://github.com/THUDM/AgentBench
- WebArena:
  - https://webarena.dev/
  - https://arxiv.org/abs/2307.13854
- BrowseComp:
  - https://openai.com/index/browsecomp/
- OSWorld:
  - https://os-world.github.io/
  - https://arxiv.org/abs/2404.07972
- LongBench v2:
  - https://github.com/THUDM/LongBench
  - https://arxiv.org/abs/2412.15204
- RULER:
  - https://arxiv.org/abs/2404.06654

## Suite Restructure
- Removed basic collections that are better covered by unit tests than LLM eval:
  - `tool-coverage`
  - `prompt-effectiveness`
  - `task-completion-speed`
- Added industry benchmark transfer collections:
  - `industry-benchmark-coding-workflow`
    - file: `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_coding_workflow.yaml`
    - mapped from: SWE-bench Verified, τ-bench, AgentBench
  - `industry-benchmark-web-and-computer-use`
    - file: `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_web_and_computer_use.yaml`
    - mapped from: WebArena, BrowseComp, OSWorld, WebChoreArena-style tasks
  - `industry-benchmark-long-context-reasoning`
    - file: `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_long_context_reasoning.yaml`
    - mapped from: LongBench v2, RULER, InfiniteBench-style long-context tasks

## Context Prompt Correction
- Added routing guardrails at context system prompt composition path:
  - `internal/app/context/manager_prompt.go` (`composeSystemPrompt`)
- Added/updated tests:
  - `internal/app/context/manager_test.go`

## Current R8 Evaluation Snapshot
- report: `tmp/foundation-suite-r8-industry-transfer/foundation_suite_report_foundation-suite-20260209-120754.md`
- collections: `20`
- pass@1: `291/326`
- pass@5: `325/326`
- failed: `1`
- top1 misses: `34/326`

## Notes
- This version intentionally increases difficulty by importing benchmark-style ambiguity and cross-tool conflict patterns.
- One hard-failure remains in long-context benchmark transfer (`industry-ruler-human-consent-critical-step`) and is preserved as optimization target.

