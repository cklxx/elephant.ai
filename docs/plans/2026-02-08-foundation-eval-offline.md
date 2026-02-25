# 2026-02-08 Foundation Eval (Offline)

## Goal
构建并执行一个不依赖模型调用的“基础能力评测”，聚焦：
- 提示词质量（Prompt Quality）
- 工具可用性（Tool Usability）
- 工具可发现性（Tool Discoverability）
- 隐式工具使用准备度（Implicit Tool-Use Readiness）

## Scope
- 新增 `alex eval foundation` 子命令
- 新增离线评测实现与报告输出
- 新增隐式意图场景集（YAML）
- 新增单元测试并执行
- 产出本次评测报告（markdown）

## Non-Goals
- 不调用外部模型做 judge
- 不修改线上推理流程
- 不改动现有订阅相关改动

## Plan
1. 设计离线评测数据结构与评分规则
2. TDD：先补评分规则与加载器测试
3. 实现 `foundation` 评测逻辑与报告生成
4. 接入 `cmd/alex/eval.go`
5. 跑测试与离线评测命令，生成报告
6. 总结失败/成功 case 与修复建议

## Progress
- [x] 明确评测目标和实现边界
- [x] 完成基础评测实现
- [x] 完成测试与报告

## Execution Notes
- 新增 `alex eval foundation` 离线子命令（不依赖模型调用）
- 新增基础场景集 `evaluation/agent_eval/datasets/foundation_eval_cases.yaml`（47 个隐式意图 case）
- 新增多配置评测运行（web/cli + 多 preset）并输出 JSON + Markdown 报告
- 测试通过：
  - `go test ./evaluation/agent_eval ./cmd/alex`
- 全量校验状态：
  - `./scripts/run-golangci-lint.sh run ./...` 失败（已有仓库存量问题，非本次改动引入）
  - `go test ./...` 失败于 `internal/shared/config` 的 guard 测试（已有仓库存量问题）

## Validation
- `go test ./evaluation/agent_eval ./cmd/alex`
- `go run ./cmd/alex eval foundation --mode web --preset full --format markdown`

## Output
- 代码：`evaluation/agent_eval/*foundation*`, `cmd/alex/eval.go`
- 场景集：`evaluation/agent_eval/datasets/foundation_eval_cases.yaml`
- 报告：`tmp/eval-foundation-*/foundation_report_*.md`
