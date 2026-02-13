# Real Subscription Agent E2E Eval + Dataset Type Fix Plan (2026-02-13)

## Goal
用真实订阅（Kimi API key + Kimi base URL）执行 agent 端到端评测，避免 mock 环境；同时修复 `alex eval` 将 SWE-Bench 数据错误按 `general_agent` 解释的问题，保证评测输入与真实问题一致。

## Scope
- 在真实订阅配置下运行 `go run ./cmd/alex eval ...`（非 foundation 离线评测）。
- 执行 `evaluation/swe_bench/real_instances.json` 的真实链路验证。
- 修复 `cmd/alex/eval.go` 的 dataset type 解析。
- 增加 dataset type 解析回归测试。
- 输出评测分析与 code review 记录。

## Steps
1. 校验真实运行配置（provider/model/base_url）并确认非 mock。 (completed)
2. 执行真实订阅 E2E 跑数并收集 artifacts。 (completed)
3. 识别异常输入链路（空 `instance_id` / 空 problem prompt）并定位原因。 (completed)
4. TDD 修复 `alex eval` dataset type 解析（新增 `--dataset-type` + 自动推断）。 (completed)
5. 在真实订阅链路下回归验证修复效果。 (completed)
6. 输出分析、审查、提交、合并。 (in_progress)

## Acceptance
- `alex eval` 对 `evaluation/swe_bench/real_instances.json` 默认走 `swe_bench`，不再误走 `general_agent`。
- 至少有一条修复后真实 run 证明 `instance_id` 正确注入。
- 提供真实订阅 run 产物路径、成功/失败/超时分布、token/cost 数据。
