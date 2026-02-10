# Plan: Lark Session Persistent Reuse + IM 最近5轮召回

> Created: 2026-02-10
> Status: completed
> Trigger: 用户要求 Lark 聊天默认永久复用同一 session；压缩阈值改为 70%；压缩后注入最近聊天记录；仅 `/new` 才新建 session，并明确“最近5轮”是 IM 聊天轮次而非 agent message。

## Goal & Success Criteria
- **Goal**: 让 Lark 聊天在默认路径中稳定复用 chat 级 session，并在上下文压缩后通过 IM 历史补回最近 5 轮用户发起聊天。
- **Done when**:
  - 普通消息不会新建 session，而是复用该 chat 绑定的 session。
  - `/new` 会显式创建并切换到新 session。
  - 自动压缩触发阈值为 token 使用率 > 70%。
  - 执行请求时注入的“近期对话”来源于 IM 拉取，并按最近 5 轮用户发言截断。
  - 相关单测通过，`make test` 通过。
- **Non-goals**:
  - 不改动 agent 内部 message 存储模型。
  - 不新增复杂编排或跨系统消息总线。

## Current State
- `internal/delivery/channels/lark/gateway.go`：当前会话复用主要靠内存 slot 的 `lastSessionID`，重启后不可靠；存在 `/reset` 清空逻辑。
- `internal/delivery/channels/lark/chat_context.go`：当前按消息条数拉取聊天历史，不按“轮次”截断。
- `internal/app/context/manager.go`：默认压缩阈值为 `0.8`。
- `internal/delivery/server/bootstrap/lark_gateway.go`：已具备 Lark store wiring 模式，可扩展 chat→session 持久化绑定。

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | 引入 chat→session 持久化绑定存储并接入 gateway | `internal/delivery/channels/lark/chat_session_binding_*.go`, `internal/delivery/channels/lark/gateway.go`, `internal/delivery/server/bootstrap/lark_gateway.go` | M | — |
| 2 | 调整会话策略：默认复用，新增 `/new`，`/reset` 改为弃用提示 | `internal/delivery/channels/lark/gateway.go`, `internal/delivery/channels/lark/task_command_test.go`, `internal/delivery/channels/lark/gateway_test.go`, `internal/delivery/channels/lark/inject_message_test.go` | M | T1 |
| 3 | IM 最近5轮召回（按聊天轮次而非 agent message）并接入上下文注入 | `internal/delivery/channels/lark/chat_context.go`, `internal/delivery/channels/lark/gateway.go`, `internal/delivery/channels/lark/chat_context_test.go`, `internal/delivery/channels/lark/gateway_test.go` | M | — |
| 4 | 调整自动压缩阈值到 70% 并补测试 | `internal/app/context/manager.go`, `internal/app/context/manager_test.go` | S | — |
| 5 | 运行测试、执行 code review skill、修复问题、提交并合并 main | 多文件 | M | T1,T2,T3,T4 |

## Technical Design
- **Approach**:
  - 在 Lark 层新增 `ChatSessionBindingStore`（Postgres 实现），持久化 `(channel, chat_id) -> session_id` 映射，gateway 在启动后可恢复 chat 对应 session。
  - 会话决策优先级：`slot.awaiting session` > `slot.lastSessionID` > `persisted chat binding` > `newSessionID`。`EnsureSession` 成功后回写绑定，确保持久复用。
  - 增加 `/new` 命令直接生成新 session 并切换绑定；保留 `/reset` 但仅提示“已弃用，请使用 /new”。
  - IM 召回按 API 返回消息（倒序）转正序后，基于“用户消息切分轮次”保留最近 5 轮，再格式化注入 `[近期对话]`。
  - 将 `defaultThreshold` 从 `0.8` 改为 `0.7`。
- **Alternatives rejected**:
  - 仅依赖内存 `lastSessionID`：进程重启丢失，无法满足“永久复用”。
  - 压缩后回注 agent 内部最近 message：与用户要求“IM 最近5轮”不一致。
- **Key decisions**:
  - 以 chat 为主键而非 user 维度，避免群聊内发送者变化导致 session 漂移。
  - “5轮”定义为最近 5 次用户发言开启的轮次，包含后续 assistant/bot 响应，符合聊天语义。

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| IM API 返回消息不足，无法凑满5轮 | M | L | 自动降级为现有可得轮次，不阻塞主流程 |
| `/reset` 语义变化引起旧习惯冲突 | M | M | 明确中文提示，指向 `/new` |
| 新增持久化表初始化失败 | L | M | 启动时 `EnsureSchema`，失败则降级仅内存复用并记录告警 |

## Verification
- 单测：
  - gateway 会话选择优先级与 `/new`、`/reset` 行为。
  - chat_context 最近5轮裁剪与当前消息排除逻辑。
  - chat-session 绑定 store CRUD/scan 流程。
- 集成：`go test ./internal/delivery/channels/lark/...`、`go test ./internal/app/context/...`。
- 全量：`make test`。
- 回滚：按提交粒度回退（先回退 chat binding，再回退 IM 轮次召回，再回退阈值变更）。
