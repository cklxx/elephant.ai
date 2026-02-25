# 2026-02-25 Lark Agents Teams 消息通知展示与监控优化调研报告

## 1. Intent & Scope

### 1.1 目标
围绕 Agents Teams 在 Lark（飞书）场景的通知体验，设计一套“低打扰、可监控、可持续优化”的完整方案，重点解决：
- 通知展示如何降低用户心智负担
- 通知策略如何控制打扰与升级
- 监控体系如何判断“有用提醒”与“无效噪音”

### 1.2 范围
- 平台能力：飞书/Lark 官方消息与卡片能力（发送、编辑、回复、卡片更新、频控）
- 对照实践：Microsoft Teams、Slack、Google SRE
- 人因依据：中断恢复与认知负担研究
- 输出粒度：可直接落地到 `internal/agent` + `internal/delivery/channels/lark` 的设计与监控框架

### 1.3 非范围
- 本报告不直接改代码，仅提供设计与实施方案。
- 不覆盖 WeChat、Web 等其他渠道的通知细节实现。

---

## 2. Method（多轮检索）

### Round 1：飞书能力边界
确认“能做什么”和“不能做什么”：消息类型、更新窗口、限频、去重、卡片交互路径。

### Round 2：跨平台通知实践
对照 Teams/Slack 的“展示简洁、避免重复通知、用户可控”策略。

### Round 3：告警治理与认知依据
用 SRE 的告警精度/召回框架和中断研究结果，定义通知质量指标与降噪机制。

---

## 3. 证据表（核心事实）

| 主题 | 证据（摘要） | 设计含义 |
|---|---|---|
| 飞书发送消息 | `im/v1/messages` 支持文本、富文本、卡片等；同用户/同群限频均为 5 QPS；支持 `uuid` 1 小时去重 | 必须做幂等键与发送节流，避免重复和洪泛 |
| 飞书编辑消息 | 文本/富文本可编辑；单条最多编辑 20 次；卡片需走专门接口 | 不应“狂发新消息”，优先更新已有消息 |
| 飞书回复消息 | 回复接口支持多类型消息；同样有 5 QPS 与 `uuid` 去重 | 建议把增量进展放在线程回复而非新开通知 |
| 飞书更新卡片 | `PATCH /im/v1/messages/:message_id`：14 天内可更新，单消息更新 5 QPS；需 `update_multi:true`；不支持批量发送消息与仅特定人可见卡片更新 | 推荐“单主卡片持续更新”模型，但要有超窗/不支持场景回退策略 |
| 飞书卡片更新模式 | 官方文档给出全量更新、局部更新、流式更新三类路径 | 长流程适合“同卡片持续更新 + 流式文本”减少碎片消息 |
| 飞书自定义机器人 | 自定义机器人限频为 100 次/分钟、5 次/秒；建议避开整点/半点高峰；请求体 ≤20KB；且卡片不支持回调交互 | 高互动流程应优先应用机器人而非自定义 webhook 机器人 |
| Teams 活动通知设计 | 活动 Feed 是通知入口，官方强调卡片结构清晰（图标/标题/预览/时间） | 通知展示应极简结构化，避免大段文本 |
| Microsoft Graph 通知最佳实践 | 仅发送“相关且可执行”的通知；避免 activity feed 与 bot 重复通知；按用户节流 | 需要统一通知源与去重规则，禁止双通道重复推送 |
| Slack 通知设置 | 用户可配置关键词、时段、DND、线程与提及规则 | 必须提供“个体化降噪开关”，把控制权交给用户 |
| Google SRE 告警框架 | 评估告警要看 precision/recall/detection/reset；推荐 multi-window multi-burn-rate 以降低误报 | 可将 L2/L3 通知当作“告警”，建立质量指标闭环 |
| 中断认知研究 | 中断会引发额外压力与恢复成本；近年实验提示“恢复线索(cues)”可降低主观负担 | 通知需提供“上下文线索 + 下一步”，降低恢复成本 |

---

## 4. 设计原则（从证据推导）

1. 默认安静：除非需要人决策，否则不打断。
2. 一个任务一个主对象：优先更新已有卡片/线程，不重复开新通知。
3. 先可见后打断：先在时间线可见，再升级为需要动作的提醒。
4. 每条通知必须回答三件事：发生了什么、影响是什么、你现在要做什么。
5. 用户可覆写：随时降噪、静音、改频率，且可恢复。
6. 可观测先行：每次通知决策都必须有可审计 reason code 与 outcome。

---

## 5. 目标方案（展示 + 策略 + 审批）

### 5.1 展示层：单主卡片 + 线程更新

#### 结构
- 主卡片固定三段：
  - `状态`: 完成/阻塞/待审批
  - `下一步`: 系统将做什么或用户需要做什么
  - `主按钮`: 仅一个主动作（例如“批准”或“补充信息”）
- 历史细节与日志进入线程回复，默认折叠。

#### 更新策略
- 文本/富文本：走 `message/update`
- 卡片：走 `message-card/patch`
- 超过 14 天或不满足 `update_multi` 条件：回退到线程回复 + 新摘要卡片

### 5.2 策略层：四级通知分层

- `L0 静默记录`：仅事件流，不推送
- `L1 可见不打断`：更新主卡片/时间线
- `L2 需用户动作`：单条可操作通知
- `L3 高风险`：@用户 + 审批门（外发、不可逆、高成本）

升级条件建议：
- `blocked=true`（流程阻塞）
- `irreversible=true`（不可逆）
- `external_side_effect=true`（外发消息/外部写操作）
- `user_stop_signal=false`（未显式停止提醒）

### 5.3 幂等与去重

建议去重键：
`dedupe_key = user_id + team_id + task_id + event_type + state_version`

发送前检查：
- 1 小时内是否同键已发送（结合飞书 `uuid` 去重）
- 目标用户是否处于“降噪窗口/静音窗口”

### 5.4 审批与安全

- 所有外发或不可逆动作必须经 approval gate。
- L3 通知必须可追溯到：`reason_code + policy_version + actor + decision`。

---

## 6. 监控方案（3 层指标）

### 6.1 传输层（可靠性）
- `delivery_success_rate`
- `delivery_latency_p95`
- `throttle_rate`（如 429/11232）
- `duplicate_delivery_rate`
- `out_of_order_rate`

### 6.2 决策层（策略质量）
- `actionable_rate = 有动作通知数 / (L2+L3 通知数)`
- `false_interrupt_rate = 24h 无动作的 L2+L3 / (L2+L3)`
- `approval_reject_rate`
- `escalation_rate_by_reason`
- `auto_resolve_without_interrupt_rate`

将 SRE 的四指标映射到通知策略：
- precision：被打断后确实需要动作的比例
- recall：确实重要事件被提醒到的比例
- detection time：从事件发生到用户可见/可行动的时间
- reset time：问题已恢复后通知状态收敛时间

### 6.3 体验层（心智负担）
- `interruptions_per_user_per_day`
- `no_action_notification_ratio`
- `median_time_to_first_action`
- `snooze_or_mute_rate`
- `after_hours_interruptions`

### 6.4 Burden Score（建议）

```
BurdenScore (0-100)
= 30% * interruptions_z
+ 25% * no_action_ratio_z
+ 20% * after_hours_z
+ 15% * duplicate_z
+ 10% * escalation_z
```

自适应动作：
- `Score >= 70`：自动切到摘要模式（例如 10 分钟聚合）
- `Score >= 85`：仅保留 L3 + 显示“可一键恢复实时通知”

---

## 7. 低心智负担机制（产品策略）

1. 批处理窗口：默认 3~10 分钟聚合 L1/L2 同类事件。
2. 解释性通知：每条通知附“为什么收到这条”。
3. 一键降噪：`静音 4 小时` / `仅阻塞通知` / `每日摘要`。
4. 进度可见性优先：通过同一主卡片展示 checkpoint，而不是频繁新消息。
5. 恢复线索：在更新消息中保留上一步/下一步上下文，降低任务恢复成本。

---

## 8. 落地路线图（建议 4 周）

### Week 1：埋点与基线
- 打通 typed events：`notification_decision_made`、`notification_sent`、`notification_acted`、`notification_snoozed`
- 先出基线看板（不改策略）

### Week 2：展示改造
- 上线“单主卡片 + 线程更新”
- 建立回退路径（不满足 patch 条件时自动 fallback）

### Week 3：策略引擎 + A/B
- 上线 L0-L3 分层与去重键
- A/B：旧策略 vs 新策略（核心看 BurdenScore、actionable_rate）

### Week 4：个性化与自动调频
- 启用用户级偏好（静音窗口、摘要频率、关键词）
- 引入基于 BurdenScore 的自动降噪

---

## 9. 风险与缓解

1. 风险：过度聚合导致关键事件延迟
- 缓解：L3 永不聚合，直接通知

2. 风险：卡片更新失败（权限/版本/窗口）
- 缓解：统一 fallback（线程回复 + 新摘要卡）并记录失败 reason

3. 风险：多通道重复打扰
- 缓解：统一 notification orchestrator；同一 `dedupe_key` 跨通道去重

4. 风险：策略过拟合导致漏报
- 缓解：保留 recall 监控与人工抽样复核

---

## 10. 对 elephant.ai 的实现映射（建议）

- `internal/agent/domain/events/`：新增通知决策与结果事件（typed）
- `internal/agent/domain/`：实现 L0-L3 决策器 + reason code
- `internal/delivery/channels/lark/`：
  - 统一“创建/更新/回复”适配器
  - 封装 `uuid` 幂等与频控重试策略
- `internal/infra/observability/`：接入三层指标 + BurdenScore
- `internal/agent/domain/approval`：L3 与外发动作强制审批

---

## 11. 结论

在飞书能力边界下，最优路径不是“多发通知”，而是“单主卡片持续更新 + 分级打扰 + 可解释与可控 + 三层监控闭环”。

如果只做一件事，优先做：
**“单主卡片 + L0-L3 分级 + dedupe_key + actionable_rate/BurdenScore 看板”**。

这会最快降低噪音，并为后续个性化调频提供稳定数据基础。

---

## 12. Sources（调研来源）

### 飞书 / Lark 官方
- https://open.feishu.cn/document/server-docs/im-v1/message/create
- https://open.feishu.cn/document/server-docs/im-v1/message/update
- https://open.feishu.cn/document/server-docs/im-v1/message/reply
- https://open.feishu.cn/document/server-docs/im-v1/message-card/patch
- https://open.feishu.cn/document/feishu-cards/update-feishu-card
- https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot

### Microsoft 官方
- https://learn.microsoft.com/en-us/microsoftteams/platform/concepts/design/activity-feed-notifications
- https://learn.microsoft.com/en-us/graph/teams-activity-feed-notifications-best-practices

### Slack 官方
- https://slack.com/help/articles/201355156-Configure-your-Slack-notifications

### SRE / Reliability
- https://sre.google/workbook/alerting-on-slos/
- https://sre.google/sre-book/practical-alerting/

### 研究论文
- Mark, G. et al. (2008). *The Cost of Interrupted Work: More Speed and Stress*. DOI: https://doi.org/10.1145/1357054.1357072
- Lim et al. (2025). *Mitigating interruption-induced workload by displaying task context and transition cues*. Scientific Reports. https://www.nature.com/articles/s41598-025-04812-8
