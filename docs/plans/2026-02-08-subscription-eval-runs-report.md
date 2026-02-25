# 2026-02-08 Subscription Eval Runs Report

## Goal
- Verify local subscription-backed execution by running multiple complete evaluation jobs end-to-end.
- Produce a reproducible report with run configs, outcomes, artifacts, and stability conclusions.

## Plan
- [x] Gather required context (engineering practices, recent memory/experience entries, eval entrypoints).
- [x] Prepare clean eval workspace/output directory and fixed run configuration.
- [x] Execute run #1 (complete).
- [x] Execute run #2 (complete).
- [x] Execute run #3 (complete).
- [x] Collect artifacts (`summary/results/report`), aggregate metrics, and analyze variance/failures.
- [x] Deliver full report to cklxx.

## Progress Log
- 2026-02-08 12:12: Initialized isolated worktree `eval/subscription-benchmark-20260208` from `main` and copied `.env`.
- 2026-02-08 12:14: Reviewed evaluation entrypoints and confirmed `agent_eval` CLI manager path for local runs.
- 2026-02-08 12:16: Found blocker: evaluation pipeline built batch config without model, causing `invalid config: model name is required`.
- 2026-02-08 12:18: Implemented minimal fix in `evaluation/agent_eval/evaluation_manager.go` to construct batch config from swe-bench config manager defaults + runtime overrides; added regression test `TestBuildBatchConfigAppliesDefaultsAndOverrides`.
- 2026-02-08 12:18: Ran `go test ./evaluation/agent_eval/...` (pass).
- 2026-02-08 12:20: Completed eval run #1 (`fixed-run1`) with full artifacts.
- 2026-02-08 12:22: Completed eval run #2 (`fixed-run2`) with full artifacts.
- 2026-02-08 12:24: Completed eval run #3 (`fixed-run3`) with full artifacts.
- 2026-02-08 12:27: Ran `./dev.sh lint`; blocked by pre-existing web warnings in `web/app/dev/log-analyzer/page.tsx` (`react-hooks/incompatible-library`).
- 2026-02-08 12:28: Ran `./dev.sh test`; blocked by pre-existing race failures in `internal/delivery/server/bootstrap` (`TestRunLark_FailsWhenLarkDisabled`, `TestRunLark_FailsWhenCredentialsMissing`).
- 2026-02-08 12:30: Synced raw eval artifacts into main repo path `tmp/eval-subscription-20260208` for easier access outside temporary worktree.

## Fixed Run Configuration
- Command:
  - `go run ./cmd/alex eval --dataset evaluation/agent_eval/datasets/general_agent_eval.json --output tmp/eval-subscription-20260208/<fixed-runN> --limit 5 --workers 2 --timeout 300s --format markdown -v`
- Data: `evaluation/agent_eval/datasets/general_agent_eval.json`
- Workers: `2`
- Per-task timeout: `300s`
- Runs: `3` (`fixed-run1`, `fixed-run2`, `fixed-run3`)

## Run Results
| Run | Job ID | Success Rate | Completed / Failed | Overall Score | Grade | Batch Duration | Tokens | Cost (USD) | Model |
|---|---|---:|---:|---:|---|---:|---:|---:|---|
| fixed-run1 | eval_1770524304_1 | 40.0% | 2 / 3 | 57.2% | F | 95.04s | 26731 | 0.0133655 | kimi-for-coding |
| fixed-run2 | eval_1770524420_1 | 20.0% | 1 / 4 | 59.1% | F | 103.80s | 500 | 0.00025 | kimi-for-coding |
| fixed-run3 | eval_1770524567_1 | 20.0% | 1 / 4 | 59.1% | F | 54.40s | 7096 | 0.003548 | kimi-for-coding |

## Aggregate Metrics (3 Runs)
- Total tasks: `15`
- Completed: `4`
- Failed: `11`
- Average success rate: `26.67%` (min `20%`, max `40%`)
- Average overall score: `58.45%` (all grades `F`, all risk `high`)
- Total tokens: `34327`
- Total estimated cost: `$0.0171635`

## Failure Analysis
- Dominant failure type: `execution_error`
- Dominant error message:
  - `task execution failed: think step failed: LLM call failed: llm rate limit exceeded for user`
- Frequency:
  - `fixed-run1`: `3/5` failed by rate limit
  - `fixed-run2`: `4/5` failed by rate limit
  - `fixed-run3`: `4/5` failed by rate limit

## Notable Anomalies
- In run2/run3, one completed entry appears as `task_<n>_retry_1` with empty `instance_id` in `detailed_results.json`.
- `alex config` showed `gpt-5.2-codex`, but batch summaries recorded `model_name: kimi-for-coding`; effective model resolution in eval runtime should be re-verified.

## Artifacts
- Run roots:
  - `tmp/eval-subscription-20260208/fixed-run1`
  - `tmp/eval-subscription-20260208/fixed-run2`
  - `tmp/eval-subscription-20260208/fixed-run3`
- Per-run key files:
  - `results.json/summary.json`
  - `results.json/detailed_results.json`
  - `report_eval_<job_id>.md`
  - `agents/default-agent/eval_<job_id>.json`
  - `console.log`
