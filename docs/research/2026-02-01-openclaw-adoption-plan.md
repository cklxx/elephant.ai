# OpenClaw 借鉴方案（面向 elephant.ai，细节版）

Updated: 2026-02-01

## 0. 目标与边界
- 目标：以最小入侵方式，引入“系统级主动性 + 可审计长期记忆 + 工具治理 + macOS 原生体验”。
- 约束：保持 `agent/ports` 不引入 memory/RAG 依赖；避免兼容性 shim；强调结构性分层。
- 假设：现有 Lark/CLI/Web 通道稳定，不更换主模型和主交互协议。

## 1. 总体策略（分层借鉴）
1. **Gateway 驱动主动性**：把调度与事件触发放到系统层而非模型层。
2. **文件优先长期记忆**：文件是唯一事实来源，索引仅用于检索加速。
3. **Typed Tools + allow/deny**：工具最小化 + 治理策略前置。
4. **平台原生集成优先**：macOS 通过 companion app/本地 node host 统一权限入口。

## 2. elephant.ai 架构映射（对照点）
- 现有主动性相关：`internal/scheduler`、`internal/agent/app/hooks`、`internal/workflow`。
- 记忆相关：`internal/memory/`、`internal/rag/`、`internal/tools/builtin/memory`。
- 工具相关：`internal/tools/builtin/`、`internal/toolregistry/`。
- 通道与执行：`internal/channels/`、`internal/agent/domain/react`、`internal/observability/`。

## 3. 主动性（Cron + Hooks）

### 3.1 Cron 设计细节
- 目标：将“时间驱动任务”从模型转移到系统调度器。
- 存储：新增 `scheduler_jobs`（或文件化 `automation/cron/*.yaml`）
  - 字段建议：
    - `id`、`schedule`（cron/interval）
    - `timezone`（显式）
    - `payload`（任务描述/输入）
    - `mode`（main|isolated）
    - `status`（active|paused|error）
    - `last_run_at`、`next_run_at`
    - `max_concurrency`、`cooldown_seconds`
    - `retry_policy`（max_retries/backoff）
- 调度策略：
  - **claim + heartbeat**：调度器只负责 claim，执行器负责心跳与最终状态。
  - **at-least-once**：若执行器崩溃，任务可在下个心跳窗口被重试。
  - **幂等保证**：为 job 生成 `run_id`，执行侧做幂等检查。
- 执行模式：
  - **main**：向主会话写入 system event，再触发 agent 执行。
  - **isolated**：创建新 session（或 run context），隔离副作用。
- 触发机制：
  - `tick`（定时器）+ `immediate`（手动唤醒）
  - 统一经 `internal/scheduler` 生成任务事件。

### 3.2 Hooks 设计细节
- 目标：把“事件驱动自动化”从 model 层移出。
- 事件分类（建议）：
  - `session.started` / `session.ended`
  - `command.*`（如 `/new`, `/reset`, `/stop`）
  - `tool.completed` / `tool.failed`
  - `memory.flush.requested`
  - `gateway.startup`
- Hook 结构：
  - `id`, `event`, `filters`, `action`, `timeout`, `priority`。
  - `filters` 支持 session_id/channel/tool_name 等维度。
- 执行机制：
  - `internal/agent/app/hooks` 作为 registry。
  - Hook 执行应有 **速率限制** 与 **并发上限**。
  - Hook 失败不能阻塞主流程，失败写入 observability。

## 4. 长期记忆（文件 + 索引）

### 4.1 文件化记忆结构
- 目录建议：`memory/`（workspace 或 `docs/memory/`）
- 结构：
  - `memory/YYYY-MM-DD.md`：每日事实与事件摘要
  - `MEMORY.md`：长期事实（非时序）
- 规则：
  - 记忆写入必须可 diff、可回滚。
  - 不允许模型仅在上下文内“记忆”，必须落盘。

### 4.2 记忆捕获与刷新
- **auto_capture**：在特定事件（session end、/reset）触发。
- **flush before compaction**：在上下文压缩前触发一次 “memory flush”。
- **策略参数**（示例）：
  - `auto_capture_ttl_days`
  - `chat_turn_ttl_days`
  - `workflow_trace_ttl_days`
- 记忆写入模板：
  - 标题 + 来源 + 置信度 + 生效范围。

### 4.3 Recall 分层
1. **短期**：当前 session 缓存
2. **长期**：`MEMORY.md`
3. **检索**：RAG/向量索引（延迟加载）

## 5. 工具治理（Typed Tools + allow/deny）

### 5.1 Typed Tools
- 为每个工具定义 schema：参数、风险级别、输出规范。
- 统一登记到 `internal/toolregistry`。

### 5.2 allow/deny 与 profile
- 通过 profile 定义工具策略：
  - `tools.allow` / `tools.deny`（组合策略）
- 针对高风险工具（system.run、exec）强制 approval。

### 5.3 工具分组
- 建议分组：runtime/fs/memory/web/ui/automation/messaging
- 业务侧只允许引用 group，不直接列出所有工具。

## 6. macOS 集成（本地 node host + 权限中心）

### 6.1 本地 node host
- 在 macOS 上运行本地 node host，暴露：
  - system.run
  - screen capture
  - audio record
  - UI automation
- Gateway 通过 token 调用本地 node host，避免权限散落。

### 6.2 权限与审批
- TCC 权限请求由 companion app 统一触发。
- approvals 存储在本地 keychain/secure storage。
- 失败回退策略：若权限拒绝，降级到无权限工具。

### 6.3 交互体验
- 在 CLI/Web 中暴露状态（权限是否就绪）。
- 提供明确的“权限缺失”错误指导。

## 7. 配置设计（YAML 示例）

### 7.1 Cron
`configs/automation/cron.yaml`
```
version: 1
jobs:
  - id: daily_summary
    schedule: "0 9 * * *"
    timezone: "Asia/Shanghai"
    mode: main
    payload:
      prompt: "Generate daily summary"
    max_concurrency: 1
    cooldown_seconds: 3600
    retry_policy:
      max_retries: 3
      backoff_seconds: 60
```

### 7.2 Hooks
`configs/automation/hooks.yaml`
```
version: 1
hooks:
  - id: memory_flush_on_reset
    event: command.reset
    action: memory.flush
    timeout_seconds: 10
    priority: 100
```

### 7.3 Tool allow/deny
`configs/tools/allowlist.yaml`
```
version: 1
profiles:
  default:
    allow:
      - group:fs
      - group:memory
      - group:web
      - group:automation
```

## 8. Observability
- 指标：
  - `cron.run.success_rate`
  - `hook.exec.latency_p95`
  - `memory.flush.count`
  - `tool.approval.denied_rate`
- 日志：
  - 按 `session_id`, `job_id`, `hook_id` 关联
- 告警：
  - Cron 连续失败
  - Hook 失败率过高
  - Memory flush 失败

## 9. 交付路线图（阶段 + 退出条件）

### Phase 1：最小 Cron + Hook registry
- 目标：可创建 job、执行单轮、可观察
- 退出条件：
  - 任务成功率 > 95%
  - Hook 触发稳定

### Phase 2：Memory flush + 文件化落盘
- 目标：session end / reset 触发 flush
- 退出条件：
  - flush 成功率 > 99%
  - 记忆文件可 diff

### Phase 3：工具治理（allow/deny）
- 目标：工具风险分级 + 策略生效
- 退出条件：
  - allow/deny 覆盖全部工具
  - 审批流程稳定

### Phase 4：macOS node host 集成
- 目标：本地权限与工具入口
- 退出条件：
  - system.run 成功率 > 98%
  - 权限降级策略有效

### Phase 5：语义检索与 recall 优化
- 目标：索引覆盖长期记忆
- 退出条件：
  - recall 命中率可测
  - 延迟在预算内

## 10. 风险与缓解
- 主动性误触发：冷却窗口 + 限流 + 失败自动暂停。
- 记忆污染：规则化模板 + 人工复核入口。
- 工具滥用：强制 allow/deny + approval gate。
- macOS 权限失败：明确降级 + 可恢复引导。

## 11. 测试策略
- 单元：scheduler、hook registry、memory flush。
- 集成：Cron → Hook → Memory。
- E2E：Lark/CLI/Web 全通道。
- 负载：cron 并发 + hook 事件风暴。

## 12. 与现有系统协作点
- `internal/agent/app/hooks`：扩展 hook registry。
- `internal/scheduler`：接入 job 持久化 + tick 机制。
- `internal/memory`：增加 file-based export/import。
- `internal/toolregistry`：实现 allow/deny policy。
- `internal/channels/*`：统一事件上报。

