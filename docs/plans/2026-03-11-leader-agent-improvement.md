# Leader Agent 信息面与控制面改进方案

Date: 2026-03-11

---

## 1. 当前架构概览

### 1.1 信息面（状态感知、事件采集、上下文聚合）

| 组件 | 路径 | 职责 |
|------|------|------|
| **Event Bus** | `runtime/hooks/bus.go` | 进程内 pub/sub，8 种事件类型：heartbeat, started, completed, failed, stalled, needs_input, handoff_required, child_completed |
| **StallDetector** | `runtime/hooks/stall_detector.go` | 周期扫描 `ScanStalled(threshold)`，基于 heartbeat 间隔判定卡死 |
| **Session Snapshot** | `runtime/runtime.go:GetSession` | 提供 SessionData：member type, goal, startedAt, state, paneID |
| **Blocker Radar** | `app/blocker/radar.go` | 扫描 task store 的 5 类 blocker：stale_progress, has_error, waiting_input, dependency_blocked, git_review_blocked |
| **ScopeWatch** | `app/scopewatch/detector.go` | 对比 work item 快照，检测 description/points/assignee/deadline 变更 |
| **Milestone Checkin** | `app/milestone/checkin.go` | 聚合 lookback window 内的 active/completed/failed tasks + token/cost |
| **Weekly Pulse** | `app/pulse/weekly.go` | 7 天窗口的 task 统计 + Git 活动指标（PR merged, review count, commits, avg review time） |
| **Attention Gate** | 配置 `config/leader_config.go` | 基于 score 阈值（40/60/80/90）做消息路由：summarize/queue/notify_now/escalate |
| **Leader Metrics** | `infra/observability/metrics_leader.go` | blocker scans, pulse generations, attention decisions, focus suppressions, alert outcomes/latency |
| **Lark Delivery Layer** | `delivery/channels/lark/` | 消息解析、plan/clarify 事件监听、慢任务进度摘要、handoff 通知、session slot 管理 |

### 1.2 控制面（决策引擎、任务派发、反馈循环）

| 组件 | 路径 | 职责 |
|------|------|------|
| **LeaderAgent** | `runtime/leader/leader.go` | 核心决策循环：订阅 EventStalled/EventNeedsInput/EventChildCompleted → LLM 决策 → INJECT/FAIL/ESCALATE |
| **Handoff** | `runtime/leader/handoff.go` | 构造 HandoffContext（session/member/goal/stallCount/elapsed/recommendedAction），发布 EventHandoffRequired |
| **Scheduler** | `app/scheduler/scheduler.go` | Cron 调度 6 类 leader job：BlockerRadar, WeeklyPulse, MilestoneCheckin, PrepBrief, ScopeWatch, DailySummary + OKR sync |
| **LeaderLock** | `scheduler.LeaderLock` | 跨进程单 leader 协调 |
| **AgentCoordinator** | `app/agent/coordinator/` | session lifecycle, ReAct engine delegation, BackgroundTaskRegistry |
| **BackgroundTaskManager** | `domain/agent/react/background*.go` | 子任务派发、状态查询、结果合并 |
| **Team Dispatch** | `domain/agent/taskfile/dispatch.go` | 结构化多角色 team workflow |
| **HandoffNotifier** | `delivery/channels/lark/handoff_notifier.go` | EventHandoffRequired → Lark 消息 + 操作建议 |
| **Dashboard API** | `delivery/server/http/leader_dashboard_handler.go` | GET /api/leader/dashboard (task counts, blockers, daily summary, scheduled jobs) |

---

## 2. 信息面盲区

### 2.1 缺失信号

| # | 盲区 | 当前现状 | 影响 |
|---|------|----------|------|
| I-1 | **LLM 调用质量信号** | EventDispatcher 仅有 SLA enrichment（工具调用延迟），但不采集 LLM response quality（token usage per turn、拒绝/幻觉检测、retry 次数） | Leader 无法识别"LLM 能力不足导致的低效循环"，只能等到 stall 才介入 |
| I-2 | **工具执行失败模式** | `tool_failure_guard_listener.go` 存在但仅做 listener；blocker radar 不区分工具失败类型（权限错误 vs 网络超时 vs 逻辑错误） | 无法做针对性恢复（如权限问题应 escalate，超时应 retry） |
| I-3 | **用户行为信号** | message_handler 仅解析文本内容，不采集用户 response latency、消息情绪倾向、连续修正次数 | Attention gate 无法动态调整 score 权重；leader 不知用户是否焦虑/满意 |
| I-4 | **子任务拓扑感知** | EventChildCompleted 只携带 child_id/goal/answer/error，不携带子任务链路深度、同级任务完成进度 | leader handleChildCompleted 无法判断"3/5 子任务已完成"这类全局进度 |
| I-5 | **跨 session 上下文** | stallSessionID 是 `leader-stall-{runtimeSessionID}` 的简单拼接，stall prompt 是 self-contained 的，禁用了 session history | 多次 stall 之间缺少累积上下文，leader 可能重复发同样的 INJECT 消息 |
| I-6 | **资源利用率信号** | Pool 有 Acquire/Release 但没有 utilization metrics 暴露给 leader | Leader 不知道 pane pool 是否饱和，无法做 capacity-aware 的任务调度决策 |
| I-7 | **Calendar/Schedule 上下文** | Scheduler 仅做静态 cron 触发，不感知当前用户日历状态（是否在会议中、focus time） | PrepBrief/DailySummary 可能在用户最忙时打断 |
| I-8 | **Git 实时信号** | ScopeWatch 和 Pulse 都是批量扫描，没有 webhook-driven 的实时 git 事件 | PR merge/review 事件延迟感知，blocker radar 的 git_review_blocked 检测有时间窗口盲区 |

### 2.2 丢失上下文

| # | 盲区 | 描述 |
|---|------|------|
| C-1 | **Stall 决策历史** | handleStall 用 stallCounts 跟踪次数但不记录每次 LLM 给出的决策内容；第 N 次 stall 时 leader 不知前 N-1 次做了什么 |
| C-2 | **任务交接上下文** | HandoffContext 有 reason/goal/elapsed 但缺少 last_tool_call、last_error_detail、session_messages_tail 等诊断信息 |
| C-3 | **Alert 反馈闭环** | Blocker radar 发出 alert 后，没有追踪 alert 是否被用户 acted_on（metrics 有 outcome counter 但缺少 alert→action 关联） |
| C-4 | **跨 job 关联** | Scheduler 各 job 独立运行，WeeklyPulse 不知道 BlockerRadar 这周检测到了多少次相同 blocker |

---

## 3. 控制面瓶颈

### 3.1 决策延迟

| # | 瓶颈 | 当前机制 | 问题 |
|---|------|----------|------|
| D-1 | **Stall 检测到介入的延迟** | StallDetector 按 `interval` 轮询（默认等于 threshold），发现 stall 后 leader 异步 LLM 调用 | 最坏情况 = threshold + LLM latency（可能 > 60s），对实时任务过慢 |
| D-2 | **LLM 决策单点串行** | handleStall 虽用 goroutine 但 inflight map 限制同一 session 只能有一个 LLM 调用 | 当多个 session 同时 stall 时，每个 session 的 LLM 调用是并行的，但单个 session 的连续 stall 必须等前一次完成 |
| D-3 | **Scheduler 触发粒度粗** | BlockerRadar 每 10 分钟扫一次，Milestone 每小时一次 | 对于紧急 blocker（如 PR review 超 SLA），最长需等 10 分钟才能检测到 |

### 3.2 派发精度不足

| # | 瓶颈 | 描述 |
|---|------|------|
| P-1 | **Stall prompt 缺乏上下文** | `buildStallPrompt` 只包含 sessionID/member/goal/elapsed/attempt 5 个字段，不包含 last tool call、last error、session 内容摘要 | LLM 只能给出通用的"请继续"而非针对性指令 |
| P-2 | **INJECT 消息无差异化** | parseDecision 只解析 INJECT/FAIL/ESCALATE 三种动作，没有 RETRY_TOOL、SWITCH_STRATEGY、REDUCE_SCOPE 等细粒度控制 | 对于"某工具频繁超时"的场景，leader 无法指示"跳过该工具" |
| P-3 | **子任务编排无全局规划** | handleChildCompleted 每次只看一个 child 的结果，依赖 LLM 做"接下来该派什么"的决策 | 没有预置的 DAG/plan 来保证任务依赖顺序和完整性 |
| P-4 | **Handoff 缺少 actionable 操作** | HandoffNotifier 发消息建议 provide_input/retry/abort，但用户只能通过 Lark 自然语言回复 | 没有 inline button/card 让用户一键操作（如"点击重试"/"点击终止"） |
| P-5 | **Blocker auto-remediation 缺失** | BlockerRadar 只做检测和通知，不触发自动修复（如自动 retry failed task、自动 nudge reviewer） | 对已知模式的 blocker（如 transient failure），仍需人工介入 |

### 3.3 反馈循环不完整

| # | 瓶颈 | 描述 |
|---|------|------|
| F-1 | **Stall recovery 无效果评估** | leader INJECT 后只等下一个 heartbeat/completed 来重置 stallCount，不评估 INJECT 是否真正解决了问题 |
| F-2 | **Alert fatigue 无自适应** | notifyCooldown 固定 24 小时，不根据用户的 acted_on 率自动调整 |
| F-3 | **Dashboard 只读无操控** | /api/leader/dashboard 只提供 GET 查询，/unblock 只做 escalated 响应，没有真正的任务控制能力 |

---

## 4. Lark 渠道用户反馈分析

基于 `delivery/channels/lark/` 的消息处理逻辑，识别到以下潜在用户需求：

| # | 信号来源 | 潜在需求 | 当前处理 |
|---|----------|----------|----------|
| U-1 | `gateway_handlers.go:isNaturalTaskStatusQuery` | 用户频繁主动查询任务状态 → 说明**进度透明度不够** | 仅被动响应查询，不主动推送 |
| U-2 | `gateway_handlers.go:isStopCommand` | 用户发 /stop → 可能是**任务跑偏或耗时过长** | 只做 cancel，不记录 stop 原因和频率 |
| U-3 | `plan_clarify_listener.go:ask_user` | agent 频繁 ask_user → **任务描述不够清晰或 agent 能力不足** | 透传到 Lark，不做频率统计或模式分析 |
| U-4 | `slow_progress_summary_listener.go` | 任务慢时才推送摘要 → 但**用户可能希望所有任务都有定期进度更新** | 仅慢任务触发，且间隔递增（30s→更长） |
| U-5 | `handoff_notifier.go` | 用户收到 handoff 后的响应行为未追踪 | 发完消息就结束，不知用户是否、何时、如何响应 |
| U-6 | `card_handler.go` + `plan_mode.go` | 计划确认流程要求用户回复 OK/修改意见 → **交互摩擦大** | 纯文本交互，无 card button |
| U-7 | `attention_gate` 在 `gateway_handlers.go` | 低分消息直接 auto-ack → 用户可能感到**被忽略** | auto-ack 消息模板固定，不解释为什么不立即处理 |

---

## 5. 改进方案

### Phase 1: 信息面增强（低风险，高收益）

#### 5.1.1 Stall 决策历史持久化

**目标**: Leader 在第 N 次 stall 时能看到前 N-1 次的决策记录。

- 在 `leader.Agent` 中增加 `stallHistory map[string][]stallRecord`
- stallRecord: `{attempt int, decision string, at time.Time, outcome string}`
- `handleStall` 完成后记录决策；EventCompleted/EventHeartbeat 时记录 outcome="recovered"
- 将 history 摘要注入 `buildStallPrompt` 的 context

**文件**: `runtime/leader/leader.go`
**复杂度**: 低

#### 5.1.2 Handoff 上下文增强

**目标**: 操作员收到 handoff 时能快速诊断。

- HandoffContext 新增字段：`LastToolCall string`, `LastError string`, `SessionTail []string`（最后 3 条消息摘要）
- `buildHandoffContext` 从 RuntimeReader 获取 session event log 尾部
- RuntimeReader interface 新增 `GetRecentEvents(id string, n int) []store.EventEntry`

**文件**: `runtime/leader/handoff.go`, `runtime/runtime.go`
**复杂度**: 中

#### 5.1.3 子任务拓扑感知

**目标**: Leader 知道"3/5 子任务已完成"。

- EventChildCompleted payload 新增 `sibling_total int`, `sibling_completed int`
- `runtime.markTerminal` 计算 parent 的所有 children 完成情况后注入 payload
- leader `handleChildCompleted` prompt 包含全局进度

**文件**: `runtime/runtime.go`, `runtime/leader/leader.go`
**复杂度**: 中

### Phase 2: 控制面精度提升（中风险，高收益）

#### 5.2.1 富 Stall Prompt

**目标**: LLM 有足够上下文做精准决策。

- `buildStallPrompt` 增加：last_tool_call, last_error, iteration_count, token_usage
- 新增决策类型：`RETRY_TOOL <tool_name>`, `SWITCH_STRATEGY <hint>`, `REDUCE_SCOPE <hint>`
- `parseDecision` 扩展支持新动作
- `applyDecision` 中 RETRY_TOOL 映射为特定 inject 指令

**文件**: `runtime/leader/leader.go`
**复杂度**: 中

#### 5.2.2 Blocker Auto-Remediation

**目标**: 对已知模式的 blocker 自动修复，减少人工介入。

- Radar.Scan 返回的 Alert 新增 `AutoRemediable bool` + `RemediationAction string`
- `ReasonHasError` + transient error pattern → auto retry
- `ReasonWaitingInput` + timeout > 2× threshold → auto escalate
- `ReasonGitReviewBlock` + wait > SLA → auto nudge reviewer via Lark
- NotifyBlockedTasks 中对 AutoRemediable alerts 调用 remediation handler

**文件**: `app/blocker/radar.go`, 新增 `app/blocker/remediation.go`
**复杂度**: 高

#### 5.2.3 Handoff Interactive Card

**目标**: 用户一键操作 retry/abort/provide_input。

- `FormatHandoffMessage` 改为返回 Lark interactive card JSON
- Card 包含 3 个 action button：重试 / 终止 / 提供输入
- `card_handler.go` 新增 handoff action callback 处理
- callback 触发对应 runtime 操作（InjectText / StopSession / 切换到 slotAwaitingInput）

**文件**: `delivery/channels/lark/handoff_notifier.go`, `delivery/channels/lark/card_handler.go`
**复杂度**: 中

### Phase 3: 反馈循环闭合（中风险，中收益）

#### 5.3.1 Stall Recovery 效果评估

**目标**: 量化 INJECT 成功率，淘汰无效策略。

- INJECT 后启动一个 evaluation timer（30s）
- timer 到期时检查 session 状态：running+heartbeat = success，still stalled = failure
- 记录到 stallHistory，metrics 新增 `alex.leader.stall.recovery.success_rate`
- 当某类 inject 的成功率 < 30% 时，leader 自动跳过 inject 直接 escalate

**文件**: `runtime/leader/leader.go`, `infra/observability/metrics_leader.go`
**复杂度**: 中

#### 5.3.2 Alert Feedback Tracking

**目标**: 建立 alert→user_action 关联。

- HandoffNotifier 发消息后记录 `{alertID, sentAt, chatID}`
- 当同一 chatID 在 alert 后 5 分钟内有消息（非 auto-ack），标记为 acted_on
- 基于 acted_on 率动态调整 notifyCooldown（高响应率 → 降低 cooldown，低响应率 → 增加 cooldown）
- Dashboard API 新增 alert response rate 指标

**文件**: `delivery/channels/lark/handoff_notifier.go`, `app/blocker/radar.go`
**复杂度**: 中

#### 5.3.3 用户行为信号采集

**目标**: 建立用户交互画像，优化 attention gate。

- gateway_handlers 记录：user_response_latency（从 ask_user 到回复）、stop_command_frequency、natural_status_query_frequency
- 新增 `domain/signal/user_signal.go`：UserSignalEvent{Kind, UserID, Latency, At}
- Attention gate 的 score 计算引入 user_urgency_factor（基于 response latency 分位数）

**文件**: `delivery/channels/lark/gateway_handlers.go`, 新增 `domain/signal/user_signal.go`
**复杂度**: 中

---

## 6. 优先级排序

| 优先级 | 改进项 | 理由 |
|--------|--------|------|
| **P0** | 5.1.1 Stall 决策历史 | 最小改动，直接提升 stall recovery 精度 |
| **P0** | 5.2.1 富 Stall Prompt | 和 5.1.1 同批交付，协同效应强 |
| **P1** | 5.1.2 Handoff 上下文增强 | 减少人工诊断时间 |
| **P1** | 5.2.3 Handoff Interactive Card | 降低用户操作摩擦，和 5.1.2 协同 |
| **P1** | 5.3.1 Stall Recovery 效果评估 | 量化改进效果的基础设施 |
| **P2** | 5.1.3 子任务拓扑感知 | 多子任务编排场景才需要 |
| **P2** | 5.2.2 Blocker Auto-Remediation | 高复杂度，需要逐个 blocker pattern 验证 |
| **P2** | 5.3.2 Alert Feedback Tracking | 需要足够的 alert 数据量才有统计意义 |
| **P3** | 5.3.3 用户行为信号采集 | 长期投资，需要数据积累 |

---

## 7. 风险与约束

1. **StallPrompt 膨胀**: 增加 context 后 prompt token 增大 → 用 truncation + 摘要控制在 500 token 内
2. **LLM 决策不可控**: 新增决策类型（RETRY_TOOL 等）依赖 LLM 正确输出 → 添加 fuzzy parsing + fallback to ESCALATE
3. **Lark card 兼容性**: interactive card 需要 bot 有 card 权限 → 检查 app 配置
4. **Auto-remediation 安全性**: 自动 retry 可能导致循环 → 设置 max_auto_retries=2 + cooldown
5. **架构边界**: `agent/ports` 不依赖 memory/RAG（long-term rule）→ 新增信号采集走 domain/signal ports

---

## 8. 度量指标

| 指标 | 目标 |
|------|------|
| Stall recovery success rate | 从当前未知 → 可度量，目标 > 60% |
| Mean time to stall intervention | 从 threshold + LLM latency → < 30s |
| Handoff → user action latency | 从不追踪 → 可度量 |
| Auto-remediation coverage | blocker alerts 中自动修复占比 > 30% |
| User stop command frequency | 下降 > 20%（因更好的进度透明度） |
