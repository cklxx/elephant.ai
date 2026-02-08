# 2026-02-08 Foundation Eval Suite Expansion

## Goal
- 继续优化剩余失败 case，并把 foundation 离线评测扩展为分层评测集合：
  - 基础工具覆盖（tool coverage）
  - 提示词有效性覆盖（prompt effectiveness）
  - 主动性测试（proactivity）
  - 复杂高价值任务（complex valuable tasks）

## Scope
- 增加 suite 级评测能力（一次运行多个 case set + 汇总报告）。
- 新增 4 份 foundation case set（YAML）。
- 基于新 case set 跑评测并对失败 case 做排序/可发现性优化。
- 增加/更新测试覆盖 suite loader、runner、报告与关键排序回归。

## Non-Goals
- 不改线上推理执行链路。
- 不引入模型调用作为 judge。

## Plan
1. 盘点现有 foundation 评测入口与可扩展点。
2. 设计 suite YAML schema + runner + aggregate report。
3. 新增 4 组评测集合 YAML（基础工具/提示词有效性/主动性/复杂任务）。
4. 首轮运行 suite，定位失败 case。
5. 定向优化失败 case（ranking heuristics + 必要的 tool metadata）。
6. 回归测试与多轮复跑，确保失败收敛。
7. 输出详细报告并更新经验记录。

## Progress
- [x] 创建 worktree 与分支并复制 `.env`
- [x] 加载工程规范与最小记忆
- [x] 实现 suite 级评测能力
- [x] 新增 4 组评测集合 YAML
- [x] 跑首轮 suite 并定位失败 case
- [x] 优化失败 case 并回归
- [x] 补齐报告与经验记录

## Validation Targets
- `go test ./evaluation/agent_eval ./cmd/alex`
- `go run ./cmd/alex eval foundation ...`（单集合）
- `go run ./cmd/alex eval foundation-suite ...`（全集合）
- `./dev.sh lint`
- `./dev.sh test`

## Execution Notes
- 新增 `alex eval foundation-suite` 子命令，支持 suite YAML 一次执行多个集合并产出 aggregate 报告。
- 新增 suite schema/runner/report：
  - `evaluation/agent_eval/foundation_suite.go`
- 新增 4 个分层 case set（总计 68 cases）：
  - `foundation_eval_cases_tool_coverage.yaml`
  - `foundation_eval_cases_prompt_effectiveness.yaml`
  - `foundation_eval_cases_proactivity.yaml`
  - `foundation_eval_cases_complex_tasks.yaml`
- 首轮 `r1` 失败 5 个 case（Top-K 92.5%），经启发式与 token alias 定向优化后 `r3` 达到 68/68（Top-K 100%，availability errors=0）。
- 已生成详细优化报告：
  - `tmp/eval-foundation-suite-20260208-r3/foundation_suite_optimization_report_20260208.md`

## Validation Status
- 已通过：
  - `go test ./evaluation/agent_eval ./cmd/alex`
  - `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml --output tmp/eval-foundation-suite-20260208-r3 --format markdown`
- 存量失败（非本次改动引入）：
  - `./dev.sh lint`：`internal/devops/*` 与 `cmd/alex/dev*.go` 的 errcheck/staticcheck 问题
  - `./dev.sh test`：`internal/delivery/server/bootstrap` 的 race 测试失败、`internal/shared/config` 的 env guard 失败
