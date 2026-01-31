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
- `request_user` — 仅在缺少关键信息时批量提问

## 设计原则：智能填充，一次完成

**优先单次完成**：分析用户输入，提取尽可能多的信息，只在缺少关键信息时用一次 `request_user` 批量提问。

## 工作流一：创建 OKR

1. **分析用户输入**
   - 从用户消息中提取：Objective、KR（指标/基线/目标/数据来源）、时间范围
   - 如果在 Lark 上下文中，自动获取 `lark_chat_id`

2. **补全缺失信息（仅在必要时）**
   - 如果缺少关键信息（Objective 不明确、KR 缺少目标值），用一次 `request_user` 批量提问所有缺失项
   - 合理默认值：review_cadence 默认每周一 `0 9 * * 1`；progress_pct 初始值 0

3. **直接创建**
   - 调用 `okr_write` 写入目标文件
   - 目标 ID 从 Objective 自动生成（如 `q1-2026-revenue`）

4. **展示结果**
   ```
   OKR 已创建：{goal_id}

   Objective: {objective}
   Key Results:
     - KR1: {metric} ({baseline} → {target})
     - KR2: {metric} ({baseline} → {target})
   Review cadence: {cadence_description}
   Notifications: {channel} ({chat_id})
   ```

## 工作流二：回顾 OKR (Review Tick)

1. **读取目标**
   - 使用 `okr_read` 获取指定目标的完整内容
   - 如果未指定 goal_id，先列出所有目标让用户选择

2. **批量更新**
   - 如果用户已提供新的 KR 数值，直接更新
   - 如果缺少数值，用一次 `request_user` 批量询问所有 KR 的当前值

3. **计算进度**
   - 公式：`(current - baseline) / (target - baseline) * 100`
   - 下降型指标（如 churn rate）：`(baseline - current) / (baseline - target) * 100`

4. **写入并展示**
   - 调用 `okr_write` 保存，`updated` 字段自动更新
   - 展示状态面板：
   ```
   OKR Dashboard: {goal_id}

   KR1: {metric}
     {current}/{target} ({progress_pct}%)
     Source: {source} | Updated: {date}

   KR2: {metric}
     {current}/{target} ({progress_pct}%)
     Source: {source} | Updated: {date}

   Overall: {avg_progress}% | Next review: {next_date}
   ```

## 最终检查清单
- [ ] 所有 KR 都有明确的指标、基线和目标
- [ ] progress_pct 计算正确
- [ ] updated 日期已刷新
- [ ] review_cadence 是有效的 cron 表达式
- [ ] 通知配置已设置（如适用）
