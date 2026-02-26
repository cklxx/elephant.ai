# Lark Agent E2E 评测报告固定格式

本文档定义 `alex lark scenario run` 的固定评测输出模板，目标是保证每次报告都有高信息密度、可追溯输入输出、可比较评分。

## 1. 报告目标

- 展示真实端到端执行结果（HTTP 注入链路），不依赖 mock 判定主结论。
- 同时回答四个问题：
  - 通过了哪些真实 case（含输入输出样例）
  - 题目难度分布是否合理
  - Lark agent 完成任务情况如何
  - 测试任务设计是否完整合理

## 2. Markdown 固定章节

`ToMarkdown()` 必须输出以下章节（顺序固定）：

1. `总评分（固定格式）`
2. `题目难度与覆盖情况`
3. `通过样例（真实输入输出）`
4. `失败场景（可复现）`
5. `场景完成度与任务设计合理性`
6. `场景详细执行记录`
7. `评分方法`

## 3. 评分定义

- `Agent 完成度`:
  - 按难度加权通过率计算（L1..L5 权重递增）。
- `任务设计质量`:
  - 基于 `eval` 元数据完整度计算：
    - objective
    - difficulty
    - category
    - capabilities
    - completion_criteria
    - design_rationale
- `综合得分`:
  - `overall = round(0.70*agent_completion + 0.30*test_design_quality)`

## 4. 场景元数据约束

每个真实 HTTP 场景应填写 `eval`：

```yaml
eval:
  objective: "..."
  difficulty: "L2"
  category: "task-command"
  capabilities: ["参数校验", "错误提示"]
  completion_criteria:
    - "ReplyMessage 包含 ..."
  design_rationale: "..."
```

缺失 `eval` 字段会拉低 `任务设计质量`，但不会阻止执行。

## 5. JSON 报告关键字段

- `summary`: 总体通过率、turn 粒度通过率、平均耗时
- `score`: 综合得分、agent 完成度、任务设计质量、公式
- `coverage`: 难度/类别/能力覆盖
- `passed_examples`: 通过样例（场景、输入、输出、验收达成）
- `failures`: 失败样例（场景、输入、输出、错误）
- `scenarios[*]`: 场景级完成度、验收达成估算、设计评分、turn 明细

## 6. 推荐执行方式

非 manual：

```bash
go run ./cmd/alex lark scenario run \
  --mode http \
  --port 9090 \
  --dir tests/scenarios/lark_http \
  --json-out tmp/eval/lark_http_non_manual.json \
  --md-out tmp/eval/lark_http_non_manual.md
```

manual（高难度推理 case）：

```bash
go run ./cmd/alex lark scenario run \
  --mode http \
  --port 9090 \
  --dir tests/scenarios/lark_http \
  --tag manual \
  --json-out tmp/eval/lark_http_manual.json \
  --md-out tmp/eval/lark_http_manual.md
```
