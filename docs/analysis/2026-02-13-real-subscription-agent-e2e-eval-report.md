# Real Subscription Agent E2E Evaluation Report (2026-02-13)

## Context
- Date: 2026-02-13
- Workspace: `elephant.ai.worktrees/agent-real-subscription-e2e-20260213`
- Runtime config (`go run ./cmd/alex config show`):
  - Provider: `openai`
  - Model: `kimi-for-coding`
  - Base URL: `https://api.kimi.com/coding/v1`
  - API key: set (from real local config/.env)
- Execution mode: real subscription runs (`go run ./cmd/alex eval ...`), no `sk-test` override.

## Commands and Artifacts

### A) Initial batch run (pre-fix, real subscription)
```bash
go run ./cmd/alex eval \
  --dataset evaluation/swe_bench/real_instances.json \
  --output tmp/agent-e2e-real-subscription-20260213-102122 \
  --workers 1 \
  --timeout 900s \
  --format markdown \
  -v
```
- Observation: batch progress stalled; aborted and switched to per-case serial runs.
- Artifact: `tmp/agent-e2e-real-subscription-20260213-102122`

### B) Per-case serial runs (pre-fix, real subscription)
```bash
go run ./cmd/alex eval --dataset tmp/real_instances_case1.json --output tmp/agent-e2e-real-subscription-case1-20260213-102616 --workers 1 --timeout 600s --format markdown -v
go run ./cmd/alex eval --dataset tmp/real_instances_case2.json --output tmp/agent-e2e-real-subscription-case2b-20260213-103226 --workers 1 --timeout 120s --format markdown -v
go run ./cmd/alex eval --dataset tmp/real_instances_case3.json --output tmp/agent-e2e-real-subscription-case3-20260213-103444 --workers 1 --timeout 120s --format markdown -v
```

Aggregate summary (pre-fix):
- Total: `3`
- Completed: `1`
- Failed/Timeout: `2`
- Success rate: `33.3%`
- Total tokens: `20264`
- Total cost: `$0.010132`

Case-level summary:
- `case1`: completed, `53.05s`, `20264 tokens`, `$0.010132`
- `case2`: timeout, `120.00s`
- `case3`: timeout, `120.00s`

Key anomaly (pre-fix):
- Output `instance_id` was empty (`""`) across preds/results.
- Case1 prompt content showed `Repository: general-agent-benchmark` and empty problem statement, not the actual SWE-Bench instance.

Evidence files:
- `tmp/agent-e2e-real-subscription-case1-20260213-102616/results.json/preds.json`
- `tmp/agent-e2e-real-subscription-case2b-20260213-103226/results.json/preds.json`
- `tmp/agent-e2e-real-subscription-case3-20260213-103444/results.json/preds.json`

## Root Cause
`cmd/alex eval` used `agent_eval.DefaultEvaluationOptions()`, whose default `DatasetType` is `general_agent`, and CLI did not expose dataset-type for override. Therefore SWE-Bench JSON path was parsed under wrong dataset type.

## Fix Implemented
### Code changes
- `cmd/alex/eval.go`
  - Added `--dataset-type` flag.
  - Added `resolveEvalDatasetType(flagValue, datasetPath)`:
    - explicit flag (`swe_bench|general_agent|eval_set|file|huggingface`) wins
    - dataset path containing `general_agent_eval` -> `general_agent`
    - empty dataset path -> `general_agent`
    - dataset path containing `swe_bench` or `real_instances` -> `swe_bench`
    - fallback -> `file`
  - Set `options.DatasetType = resolveEvalDatasetType(...)`.

- `cmd/alex/eval_dataset_type_test.go`
  - Added table-driven tests for resolution behavior.

### Test validation
```bash
OPENAI_BASE_URL=https://api.kimi.com/coding/v1 go test ./cmd/alex -run 'TestResolveEvalDatasetType'
OPENAI_BASE_URL=https://api.kimi.com/coding/v1 go test ./cmd/alex
```
- Result: pass.

## Post-fix real subscription regression
```bash
go run ./cmd/alex eval \
  --dataset tmp/real_instances_case1.json \
  --output tmp/agent-e2e-real-subscription-case1-fixed-20260213-103933 \
  --workers 1 \
  --timeout 120s \
  --format markdown \
  -v
```
Result:
- `instance_id`: `astropy__astropy-12907` (correctly injected)
- status: timeout (`120.00s`)

Evidence:
- `tmp/agent-e2e-real-subscription-case1-fixed-20260213-103933/results.json/batch_results.json`

## Conclusion
- 真实订阅链路已确认在跑（非 mock）。
- 关键评测正确性缺陷已修复：SWE-Bench dataset 不再被误路由到 `general_agent`。
- 修复后实例 ID 注入正确，后续性能优化应聚焦真实任务执行效率（timeout/工具调用路径），而不是数据集解释错误。
