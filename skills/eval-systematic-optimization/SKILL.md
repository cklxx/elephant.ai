---
name: eval-systematic-optimization
description: 系统化梳理 foundation-suite 失败 case，按冲突簇优化 pass@1，并输出标准化 x/x 报告与 good/bad 抽样检查。
triggers:
  intent_patterns:
    - "评测.*优化|pass@1|pass@5|失败case|failure case|系统性评测|benchmark.*optimi"
  context_signals:
    keywords: ["foundation-suite", "pass@1", "pass@5", "x/x", "goodcase", "badcase", "冲突"]
  confidence_threshold: 0.7
priority: 8
exclusive_group: evaluation
max_tokens: 2600
cooldown: 90
output:
  format: markdown
  artifacts: true
  artifact_type: document
---

# Foundation 评测系统优化技能

## 目标
- 先全量梳理失败，后做冲突簇级别优化，避免单点“刷题”修复。
- 将报告固定为可复用结构：`x/x`、冲突簇、bad case 拆解、good/bad 抽样、修复动作闭环。

## 工作流

### Phase 1: 跑基线
```bash
go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml \
  --output tmp/foundation-suite-<tag>-baseline \
  --format markdown
```

记录基线指标：
- collections `x/x`
- cases `x/x`
- pass@1 `x/x`
- pass@5 `x/x`
- deliverable good/bad `x/x`

### Phase 2: 失败簇归因（必须）
从 `foundation_suite_result_*.json` 提取 `hit_rank > 1` 且非 `N/A` 的 case，聚类键：
- `expected_tools => top1_tool`
- collection
- failure reason signature

优先修复 Top conflict families（按频次排序）。

### Phase 3: 系统优化（非单点）
- 优先改规则层（token alias、冲突惩罚、意图增益），不要只改单条 case 文案。
- 对 “不可用工具” 标记 `N/A`，不计失败。
- 对可用工具冲突必须给出收敛策略，不可“排除失败”。
- sandbox 语义不作为独立目标工具，收敛到执行工具（如 `shell_exec` / `execute_code`）。

### Phase 4: 扩充更难集合
- 对连续多轮 pass@1=100% 的易题集做淘汰或降权。
- 新增高冲突、多约束、长链路、文件产物要求更高的 hard cases。

### Phase 5: 回归验证
```bash
make fmt
go test ./...
go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml \
  --output tmp/foundation-suite-<tag>-after \
  --format markdown
```

### Phase 6: 报告产出
- 报告模板：`evaluation/agent_eval/report_templates/foundation_systematic_report_template.md`
- 必须包含：
  - aggregate x/x
  - dimension x/x
  - top1 conflict clusters
  - bad case 分解
  - good/bad deliverable 抽样检查
  - 优化动作与下一轮目标

## 验收标准
- pass@1 有可量化提升，或在新增 hard cases 后仍稳定。
- pass@5 不回退。
- 新增 hard collection 已接入 suite 并可复跑。
- 报告可直接用于下一轮优化（决策完整）。
