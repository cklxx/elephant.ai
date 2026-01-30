---
name: okr-management
description: 创建和管理 OKR（目标与关键结果），支持创建、回顾和进度更新工作流。
triggers:
  intent_patterns:
    - "OKR|okr|目标|关键结果|key result|季度目标|quarterly goal"
  tool_signals:
    - okr_read
    - okr_write
  context_signals:
    keywords: ["OKR", "okr", "目标", "KR", "关键结果", "进度"]
  confidence_threshold: 0.6
priority: 8
exclusive_group: planning
max_tokens: 2000
cooldown: 120
output:
  format: markdown
  artifacts: false
---

# OKR 管理（创建 & 回顾）

## When to use this skill
- 用户想创建新的 OKR（季度/月度目标）。
- 用户想回顾或更新现有 OKR 的进度。
- 用户想查看所有 OKR 的状态概览。

## 必备工具
- `okr_read` — 读取单个目标或列出所有目标
- `okr_write` — 创建或更新目标文件
- `request_user` — 多轮对齐，获取用户确认

## 工作流一：创建 OKR

1. **明确目标 (Objective)**
   - 使用 `request_user` 询问用户的 Objective。
   - 示例问题：「请描述你的目标（Objective），例如"提升月收入 30%"」

2. **定义关键结果 (Key Results)**
   - 逐个 KR 使用 `request_user` 询问：
     - 指标名称 (metric)
     - 基线值 (baseline)
     - 目标值 (target)
     - 数据来源 (source)
   - 问用户是否还需要更多 KR。

3. **设置回顾频率**
   - 使用 `request_user` 询问 review cadence：
     - 每周一 9:00 → `0 9 * * 1`
     - 每两周 → `0 9 1,15 * *`
     - 每月 → `0 9 1 * *`
   - 用户可以自定义 cron 表达式。

4. **自动捕获通知配置**
   - 如果在 Lark 上下文中，自动从 context 获取 `lark_chat_id`。
   - 如果不在 Lark 中，可选跳过或让用户手动提供。

5. **确认并写入**
   - 使用 `request_user` 展示完整 OKR 预览，请求确认。
   - 确认后调用 `okr_write` 写入目标文件。
   - 目标 ID 从 Objective 自动生成（如 `q1-2026-revenue`）。

## 工作流二：回顾 OKR (Review Tick)

1. **读取目标**
   - 使用 `okr_read` 获取指定目标的完整内容。
   - 如果未指定 goal_id，先列出所有目标让用户选择。

2. **逐个 KR 更新**
   - 对每个 KR 使用 `request_user`：
     - 展示当前值、目标值、进度百分比
     - 展示数据来源和上次更新时间
     - 询问新的 current 值（或确认不变）

3. **计算进度**
   - 根据 baseline、target、current 重新计算 progress_pct。
   - 公式：`(current - baseline) / (target - baseline) * 100`
   - 对于下降型指标（如 churn rate）：`(baseline - current) / (baseline - target) * 100`

4. **写入更新**
   - 调用 `okr_write` 保存更新后的目标文件。
   - `updated` 字段自动更新为今天的日期。

5. **生成状态面板**
   - 以结构化 Markdown 展示：
     - 每个 KR 的进度条或百分比
     - 风险等级标识（✓ on track / ⚠ at risk / ✗ off track）
     - 下一步行动建议

## 输出格式

### 创建完成后
```
✅ OKR 已创建：{goal_id}

📋 Objective: {objective}
📊 Key Results:
  - KR1: {metric} ({baseline} → {target})
  - KR2: {metric} ({baseline} → {target})
⏰ Review cadence: {cadence_description}
🔔 Notifications: {channel} ({chat_id})
```

### 回顾面板
```
📊 OKR Dashboard: {goal_id}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━

KR1: {metric}
  {current}/{target} ({progress_pct}%) {risk_icon}
  Source: {source} | Updated: {date}

KR2: {metric}
  {current}/{target} ({progress_pct}%) {risk_icon}
  Source: {source} | Updated: {date}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Overall: {avg_progress}% | Next review: {next_date}
```

## 最终检查清单
- [ ] 所有 KR 都有明确的指标、基线和目标
- [ ] progress_pct 计算正确
- [ ] updated 日期已刷新
- [ ] review_cadence 是有效的 cron 表达式
- [ ] 通知配置已设置（如适用）
