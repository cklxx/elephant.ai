---
name: self-test
description: 自主运行 Lark 场景测试套件，分析失败，按修复分级自动迭代修复。
triggers:
  intent_patterns:
    - "self.?test|自测|场景测试|scenario.?test|run.?test|测试套件"
  context_signals:
    keywords: ["test", "scenario", "lark", "测试", "自测"]
  confidence_threshold: 0.7
priority: 7
exclusive_group: testing
max_tokens: 2000
cooldown: 120
output:
  format: markdown
  artifacts: true
  artifact_type: document
---

# Lark 场景自测与自主迭代

## When to use this skill
- 运行 Lark Gateway 全场景测试套件并生成报告。
- 分析测试失败的根因，按修复分级自动或半自动修复。
- 扩展场景覆盖，从生产对话或事故中生成回归测试。

## 工作流

### Phase 1: 执行测试
```bash
CGO_ENABLED=0 go test ./internal/channels/lark/testing/ -v -json 2>&1
```
- 解析 JSON 输出，分类: pass / fail / skip。
- 生成 ScenarioReport（JSON 格式，见下方）。

### Phase 2: 失败分析（每个失败场景）
对每个失败的场景：
1. **读取场景 YAML**（`tests/scenarios/lark/<name>.yaml`）理解预期行为。
2. **读取测试输出**（错误消息、assertion 失败详情）理解实际行为。
3. **读取相关源码**（根据失败类型定位到具体文件）理解代码逻辑。

### Phase 3: 根因分类
将每个失败归类为以下之一：
- **test_drift**: 测试断言阈值漂移（如 mock 响应过期、时序抖动）
- **prompt_issue**: skill/prompt 表述不准确
- **tool_bug**: tool 实现 bug
- **gateway_logic**: gateway 逻辑问题
- **context_issue**: context 组装缺陷
- **llm_quality**: LLM 模型能力限制
- **architecture**: 架构层面问题

### Phase 4: 修复分级

| Tier | 范围 | 权限 | 安全门 |
|------|------|------|--------|
| **Tier 1** 自主修复 | `tests/scenarios/*.yaml`, `internal/channels/lark/testing/` | 无需审批 | 重跑场景验证 |
| **Tier 2** 轻量审批 | `skills/*/SKILL.md` or `skills/*/SKILL.mdx` (prompt/技能) | 自主修复 + diff 记录 | 修改前后 diff + 重跑全部相关场景 |
| **Tier 3** Plan Review | `internal/` (生产代码) | 需人工确认 | 完整 diff → plan review → 全量 lint+test |
| **Tier 4** 仅报告 | 架构变更、安全敏感、LLM 限制 | 人工决策 | 生成诊断报告 + 修复建议 |

### Phase 5: 自动迭代（最多 3 轮）
```
for round in 1..3:
    1. 读取失败列表
    2. 对 Tier 1/2 失败: 应用修复
    3. 重跑测试套件
    4. 如果全部通过 → 退出循环
    5. 如果仍有失败 → 继续下一轮
```

### Phase 6: 生成报告
输出结构化报告：
- **概要**: 通过/失败/跳过数量，总耗时。
- **失败详情**: 场景名、根因分类、修复 tier、修复操作（如有）。
- **修复历史**: 每轮迭代的 diff 和结果。
- **待处理项**: Tier 3/4 需要人工介入的问题。

## 输出格式

### 报告结构
```
## 测试报告 YYYY-MM-DD HH:MM

### 概要
- 通过: N / 失败: N / 跳过: N
- 耗时: Xs
- 迭代轮次: N

### 失败场景

| 场景 | 根因 | Tier | 状态 |
|------|------|------|------|
| xxx  | test_drift | 1 | 已修复 |
| yyy  | gateway_logic | 3 | 待人工 |

### 修复历史
（每轮修改的 diff 和结果）

### 待处理
（需要人工介入的 Tier 3/4 问题）
```

## 安全规则
- 最多 3 轮自动迭代，超过则停止并报告。
- 生产代码修改（Tier 3）走 plan review 流程。
- 架构变更（Tier 4）仅生成报告。
- 全量 `go vet ./... && go test ./internal/channels/lark/...` 通过后才可提交。
- 修复历史持久化到 `docs/error-experience/entries/` 供后续参考。

## 最终检查清单
- 所有场景 YAML 语法正确，可被 LoadScenariosFromDir 加载。
- 每个修复有 diff 记录和验证结果。
- Tier 3/4 问题有详细诊断报告。
- 报告已保存为 artifact。
