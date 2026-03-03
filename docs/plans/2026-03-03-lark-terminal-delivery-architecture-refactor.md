# 2026-03-03 Lark Terminal Delivery 架构重构方案（事件化 + Outbox）

Updated: 2026-03-03  
Status: Proposed

## 1. 背景与问题定义

现状中，Lark 最终回复（含 final answer / failure / await 提示）在 `task_manager.runTask -> dispatchResult` 同步发送，且与任务执行上下文强耦合。  
当执行上下文超时或取消时（例如 `ReplyTimeout`），终态消息发送可能被连带取消，导致用户“任务结束但未收到最终消息”。

核心问题不是“是否能发一次”，而是**终态投递缺乏独立的可靠投递语义**：
1. 发送生命周期与执行生命周期耦合。
2. 缺少统一投递状态机（pending/sent/failed/retrying/dead）。
3. 缺少显式幂等键和可重放机制。

## 2. 目标架构（应有形态）

采用“**领域事件 + 投递意图 Outbox + 异步投递 Worker**”三段式：

1. Domain/Coordinator 产生终态事件（`workflow.result.final` 等）。
2. Delivery Intent Builder 将事件转为 `DeliveryIntent`（终态文本、附件、chat_id、幂等键、序号）。
3. Intent 与任务状态在同一事务边界内持久化到 Outbox（或同等可靠持久层）。
4. Delivery Worker 异步拉取 Intent，按重试策略投递到 Lark。
5. 投递结果回写状态；超过上限进入 dead-letter，并可人工/自动重放。

## 3. 关键设计点

### 3.1 可靠性边界

- 执行成功/失败与“消息已送达”解耦：执行完成只保证写入 Intent；是否投递成功由 Worker 负责。
- 不再依赖 task execution context；Worker 使用自己的超时与重试预算。

### 3.2 幂等与顺序

- 幂等键建议：`lark:{chat_id}:{run_id}:{event_type}:{sequence}`。
- 同一 `chat_id + run_id` 内要求单调序列；`final` 事件优先级最高。
- 重放时按幂等键去重，保证“至少一次投递 + 业务幂等”。

### 3.3 重试策略

- 指数退避 + jitter + 最大重试次数。
- 仅对可重试错误重试（429/5xx/网络抖动）；4xx 语义错误直接失败并告警。
- 维护 retry budget，避免故障放大。

### 3.4 可观测性

必须有：
1. `delivery_intent_pending_total`
2. `delivery_intent_retry_total`
3. `delivery_intent_dead_total`
4. `terminal_delivery_latency_ms`（事件产生到消息送达）
5. 按 `chat_id/run_id/intent_id` 可追踪日志

## 4. 与当前代码的映射（建议）

### 新增

1. `internal/delivery/channels/lark/delivery_intent.go`
   - Intent 模型与状态机。
2. `internal/delivery/channels/lark/delivery_outbox_store.go`
   - 持久化接口与实现（可先 file store，后续可切 DB）。
3. `internal/delivery/channels/lark/delivery_worker.go`
   - 轮询/拉取、投递、重试、回写。
4. `internal/delivery/channels/lark/delivery_worker_test.go`
   - 幂等、重试、死信、重放覆盖。

### 改造

1. `internal/delivery/channels/lark/task_manager.go`
   - `dispatchResult` 从“直接发送”改为“写 Intent”。
2. `internal/app/agent/coordinator/workflow_event_translator_react.go`
   - 保证终态 envelope 字段完整（answer/stop_reason/attachments/seq）。
3. `internal/delivery/server/app/event_broadcaster.go`（可选）
   - 若需要共享投递管线，可统一终态事件入口。

## 5. 分阶段重构计划

### Phase 0（已完成）
- 最小修复：终态发送使用 detached context（止血）。

### Phase 1（低风险）
- 引入 `DeliveryIntent` 模型与 Outbox Store。
- 终态路径“写 Intent + 继续直发”（shadow mode，对账不切流）。
- 验收：Intent 生成率与直发终态事件 1:1 对齐。

### Phase 2（切流）
- 终态路径切到 Worker 异步发送；直发仅作 fallback（feature flag）。
- 验收：`terminal_delivery_missing_rate` 显著下降；无用户可见退化。

### Phase 3（统一投递）
- 将进度编辑、附件发送也纳入 Intent 机制（多类型 Intent）。
- 引入 dead-letter replay 命令。

### Phase 4（收敛）
- 移除旧直发主路径，保留应急旁路。
- 文档、运维手册、告警策略同步。

## 6. 测试与验收

### 单测
1. Context 取消后 Intent 仍可被 Worker 投递。
2. 相同幂等键重复入队只发送一次。
3. 429/5xx 触发重试；4xx 不重试。
4. 达到最大重试进入 dead-letter。

### 集成
1. 人为注入 Lark API 间歇失败，验证最终可达。
2. 进程重启后未完成 Intent 可恢复发送。
3. 高并发 chat 场景下终态顺序正确。

### 运行指标
1. `terminal_delivery_latency_p95` < 5s（示例目标）
2. `terminal_delivery_missing_rate` 接近 0
3. dead-letter 可观测且可重放

## 7. 风险与回滚

风险：
1. 双写（直发+outbox）阶段可能重复通知。
2. Worker 故障导致投递积压。

缓解：
1. 幂等键 + 发送端去重。
2. 监控 pending backlog + 自动扩容。
3. 保留 feature flag：一键回退到直发。

## 8. 结论

这次故障暴露的是“终态投递语义”缺失，而不是单点代码 bug。  
推荐将终态消息升级为**可靠事件投递架构**：  
“事件生成（可追踪）→ 意图持久化（可恢复）→ 异步投递（可重试）→ 状态回写（可审计）”。
