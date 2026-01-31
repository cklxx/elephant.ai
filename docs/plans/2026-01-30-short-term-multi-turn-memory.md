# Short-Term Multi-Turn Memory Design

> **Status:** Draft
> **Author:** cklxx
> **Created:** 2026-01-30
> **Updated:** 2026-01-30

> **⚠️ 执行顺序（更新）：** 源码验证表明 `larkMemoryManager` 在 gateway 中已 **dormant**。
> 本方案与 `2026-01-30-memory-architecture-improvement.md` Phase 1 **可并行开发**，
> 但推荐部署顺序仍为：Plan 2（稳定 session）→ Phase 1（ConversationCaptureHook + 清理 dormant 代码）→ Phase 2-4。

## 1. Problem

Lark 频道每条消息创建一个随机 sessionID（`lark-<UUID>`），session.Messages 永远为空。
LLM 看不到上一轮对话，无法做多轮追问、上下文引用、指代消解。

当前靠 `AutoChatContext` 调 Lark API 拉最近 20 条原始聊天记录注入，但这有三个问题：
1. **依赖外部 API** — 受 Lark API 限速和延迟影响
2. **原始格式** — `[timestamp] sender: text` 纯文本，LLM 无法区分用户消息、助手回复、工具调用
3. **不可复用** — 仅 Lark 可用，Web/CLI 不走这条路

## 2. 各频道现状对比

```
                    Session ID          History 累积        多轮机制
┌──────────┬───────────────────────┬──────────────────┬──────────────────────┐
│ Lark     │ lark-<随机UUID>       │ ❌ 每次从零开始   │ AutoChatContext      │
│          │ 每条消息一个新session   │                  │ (Lark API拉20条)     │
├──────────┼───────────────────────┼──────────────────┼──────────────────────┤
│ CLI      │ <UUID> 整个会话复用    │ ✅ session直接累积│ 完整上下文窗口        │
├──────────┼───────────────────────┼──────────────────┼──────────────────────┤
│ Web/API  │ 客户端提供或新建       │ ✅ 客户端复用时累积│ 完整上下文窗口        │
└──────────┴───────────────────────┴──────────────────┴──────────────────────┘
```

CLI/Web 已经有完整的多轮支持。**唯一缺失的是 Lark。**

## 3. 方案对比

### 方案 A: Lark 改用稳定 Session ID（推荐）

将 Lark 的 sessionID 从随机 UUID 改为稳定 hash：

```go
// 现在 (每条消息新session)
sessionID := fmt.Sprintf("%s-%s", g.cfg.SessionPrefix, id.NewLogID())

// 改为 (同一个chat复用session)
sessionID := g.memoryIDForChat(chatID)
```

**无需新建任何组件。** 现有基础设施自动生效：

```
第1条消息 → EnsureSession("lark-a1b2c3") → 创建空session
            → ExecuteTask() → 生成回复
            → SaveSessionAfterExecution()
               → historyMgr.AppendTurn(Turn 1: user + assistant)
               → sessionStore.Save(session)

第2条消息 → EnsureSession("lark-a1b2c3") → 拿到已有session
            → loadSessionHistory()
               → historyMgr.Replay() → 返回 Turn 1 的消息
            → recallUserHistory()
               → 如果history token < 70% MaxTokens → 原样注入
               → 如果超出 → LLM摘要压缩后注入
            → ExecuteTask() → LLM看到上轮对话 + 当前消息
            → SaveSessionAfterExecution()
               → historyMgr.AppendTurn(Turn 1 + Turn 2)
```

### 方案 B: 新建独立 Sliding Window Buffer

构建一个新的 `ConversationBuffer` 组件：
- 每个 chat 维护最近 N 个 turn pair
- 有独立的 TTL
- 注入方式类似 AutoChatContext

**不推荐。** 原因：
- 复制了 historyMgr + preparation/history.go 已有的全部功能
- 多一个组件维护
- 与 session.Messages 的语义重叠

### 方案 C: 保持现状，优化 AutoChatContext

改进 AutoChatContext 的注入格式，让 LLM 更好地理解。

**不推荐。** 治标不治本：
- 仍然依赖 Lark API（延迟、限速）
- 助手的回复在 Lark 聊天记录中是卡片/富文本，解析困难
- 工具调用过程完全不可见

### 结论：方案 A

**一行代码改动 + 零新组件 = 完整多轮支持。**

## 4. 前置验证（Preconditions Verification）

方案 A 的核心假设是：稳定 sessionID → `EnsureSession` 拿到已有 session → `loadSessionHistory()` 通过 historyMgr 恢复历史。
实施前必须验证以下代码路径在 Lark channel 下可用：

| 前置条件 | 验证方式 | 风险 |
|----------|----------|------|
| `historyMgr` 在 Lark 的 DI 路径中被正确初始化 | 检查 `container_builder.go` 中 historyMgr 是否依赖 channel type | 低 — historyMgr 是全局组件 |
| `SaveSessionAfterExecution()` 在 Lark 代码路径中被调用 | 搜索 coordinator 的 session save 逻辑 | 低 — coordinator 通用逻辑 |
| `historyMgr.AppendTurn()` 支持同一 sessionID 多次追加 | 编写单元测试：连续 3 次 AppendTurn 同一 sessionID | 中 — 当前 Lark 从未触发此场景 |
| **AppendTurn 前缀匹配不误清空** | 编写集成测试：连续两次 ExecuteTask，确保 history 未被清空 | 中 — system prompt/注入消息可能导致 prefix mismatch |
| `loadSessionHistory()` 能恢复多轮历史 | 编写集成测试：save 2 轮 → 新请求 load → 验证 Messages 包含 2 轮 | 中 — 同上 |
| session 持久化存储（file/postgres）支持更新已存在的 session | 检查 `sessionStore.Save()` 是 upsert 还是 insert-only | 低 — 已有通道在使用此路径 |
| `session.UpdatedAt` 行为可靠 | 验证 Save 是否自动更新 UpdatedAt；新建 session 是否非零 | 中 — 过期逻辑依赖 UpdatedAt |

**建议：在实施 Step 1 之前，先运行上述验证用例。如果任一失败，需先修复再改 session ID。**

## 5. 方案 A 详细设计

### 5.1 核心改动

```diff
 // internal/channels/lark/gateway.go handleMessage()

-// Each message gets a fresh session (zero history). Memory recall
-// injects relevant context instead of accumulating session messages.
 senderID := extractSenderID(event)
 memoryID := g.memoryIDForChat(chatID)
-sessionID := fmt.Sprintf("%s-%s", g.cfg.SessionPrefix, id.NewLogID())
+sessionID := memoryID  // 同一chat复用session, 多轮历史自动累积
```

改动后 `memoryID` 和 `sessionID` 是同一个值，锁逻辑不变。

**回滚开关：**

```yaml
# config.yaml
lark:
  session_mode: stable # stable | fresh
```

**同时修复 hash 长度问题：** 当前 `memoryIDForChat` 只取 SHA1 前 8 字节（64 bit）。
Plan 2 将 sessionID = memoryID，碰撞意味着**两个不同群聊共享 session（对话串台）**，后果严重。
应至少使用 12 字节（96 bit），碰撞概率降至 ~2^-48：

```diff
 // internal/channels/lark/gateway.go
 func (g *Gateway) memoryIDForChat(chatID string) string {
     hash := sha1.Sum([]byte(chatID))
-    return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:8])
+    return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:12])
 }
```

### 5.2 AutoChatContext 的去留

**保留，但降级为可选补充。**

| 场景 | AutoChatContext 的价值 |
|------|----------------------|
| P2P 私聊 | **低** — session history 已包含所有对话 |
| 群聊 | **中** — 其他用户的消息不在 session 中，AutoChatContext 提供群聊上下文 |
| 首次对话 | **无** — session 和 Lark 聊天记录都为空 |

建议：
- P2P 时关闭 AutoChatContext（session history 足够）
- 群聊时保留（补充非 bot 对话的群聊上下文）

```go
// 仅群聊时注入 AutoChatContext
if g.cfg.AutoChatContext && g.client != nil && isGroup {
    // ...fetch and inject
}
```

### 5.3 Session 生命周期管理

稳定 session 带来一个新问题：**session 可能无限增长。**

现有机制已经覆盖了大部分场景：

```
historyMgr.AppendTurn()  →  增量追加每轮对话
loadSessionHistory()     →  Replay 全部历史
recallUserHistory()      →  检查是否需要摘要
shouldSummarizeHistory() →  history tokens > 70% MaxTokens 时触发
composeHistorySummary()  →  LLM 压缩为 2-3 段摘要
```

**但需要补充一个机制：Session 过期重建。**

原因：用户两天没聊天后回来，旧 session 的上下文完全不相关。LLM 摘要会保留旧内容，
浪费 context window 且可能误导 LLM。

**关键设计决策：过期检测放在 preparation 层而非 gateway 层。**

理由：
- 其他稳定 session 的通道同样需要过期处理
- 放在 gateway 层意味着每个 channel 各写一份
- `preparation/service.go` 的 `loadSessionHistory()` 是所有 channel 加载历史的统一入口

```go
// internal/agent/app/preparation/service.go loadSessionHistory()
func (s *Service) loadSessionHistory(ctx context.Context, session *storage.Session) []ports.Message {
    // Session 过期检测 — 全局生效，所有 channel 受益
    if !session.UpdatedAt.IsZero() && time.Since(session.UpdatedAt) > s.cfg.SessionStaleAfter {
        s.logger.Info("Session %s stale (last updated %v), clearing history",
            session.ID, session.UpdatedAt)

        // 过期前保存关键信息到长期记忆（避免信息完全丢失）
        if s.memoryCaptureFunc != nil && len(session.Messages) > 0 {
            s.memoryCaptureFunc(ctx, session, appcontext.UserIDFromContext(ctx))
        }

        // 清空历史并持久化，避免重复过期捕获
        session.Messages = nil
        session.Metadata = nil // 若 historyMgr 使用 metadata 存摘要/指针，统一清理
        session.UpdatedAt = s.now()
        _ = s.sessionStore.Save(ctx, session)
        if s.historyMgr != nil {
            s.historyMgr.ClearSession(ctx, session.ID)
        }

        return nil
    }

    // ... existing history loading logic
}
```

**过期前保存长期记忆：** session 过期清空前，提取关键信息存入长期记忆。
这样即使 session history 被清空，用户仍可通过 memory recall 找到之前的关键决策。

```go
// memoryCaptureFunc 的实现（在 DI 层注入）
func captureSessionSummary(ctx context.Context, session *storage.Session, userID string) {
    // 提取最后 5 轮对话的关键信息
    lastMessages := lastN(session.Messages, 10) // 5 轮 = 10 条消息
    summary := buildSessionSummary(lastMessages)
    memoryService.Save(ctx, memory.Entry{
        UserID:  userID,
        Content: summary,
        Slots: map[string]string{
            "type":       "auto_capture",
            "subtype":    "session_expired",
            "scope":      "user",
            "session_id": session.ID,
        },
    })
}
```

**群聊处理：** 若当前为群聊（由 gateway 注入 `chat_id`/`channel`），可在 `captureSessionSummary` 中额外写一条 `scope=chat` 的摘要（`chat_id` + `channel`），受 `capture_group_memory` 配置控制，避免群聊噪声无节制增长。

**可配置参数（全局，非 channel-specific）：**

```yaml
# config.yaml
agent:
  session_stale_after: 48h  # 默认 48h（覆盖"昨天"的常见追问场景）
```

**为什么 48h 而非 24h：**
- 用户常说"昨天我们讨论的方案"，如果恰好超过 24h，session 已被清空
- 48h 覆盖"昨天"和"前天"的追问，且通过 LLM 摘要压缩控制 token 消耗
- 过期前的长期记忆保存进一步兜底

### 5.4 P2P vs 群聊的 Session Key

当前 `memoryIDForChat(chatID)` 用 chatID 做 hash。这意味着：

| 场景 | Session Key | 行为 |
|------|-------------|------|
| P2P | `lark-<hash(chatID)>` | 每个用户与 bot 的私聊是独立 session ✅ |
| 群聊 | `lark-<hash(groupChatID)>` | 全组共享一个 session |

群聊共享 session 的含义：
- Bot 能看到它在群里的所有对话历史（包括与不同用户的交互）
- 用户 A 问了一个问题，用户 B 可以追问"刚才那个问题的结果"
- **这是期望行为** — 群聊中 bot 应该有群级上下文

如果需要群聊中每个用户独立 session，可以改为：
```go
// 群聊: 按 user 隔离 session
if isGroup {
    sessionID = fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, sha1.Sum([]byte(chatID+":"+senderID))[:8])
} else {
    sessionID = memoryID
}
```

**推荐先不做用户隔离**，群聊共享 session 更自然。如果后续用户反馈需要隔离再加。

### 5.5 群聊并发性能 Trade-off

群聊共享 session + `sessionLock` 串行化 = 多人同时发消息时排队。
当前 Lark 用 fresh session 不存在这个问题（每条消息独立 session，无锁竞争）。

改成共享 session 后的影响：
- User A 消息等锁 → User B 消息等锁 → 串行处理
- 如果 ReAct loop 耗时较长（如 deep research），后续消息延迟显著

**评估：**
- 当前 Lark bot 的群聊使用场景以**低并发**为主（几人小群，偶尔@bot）
- sessionLock 的粒度是 per-sessionID，不同群聊之间不互相影响
- 对于高频群聊场景（>10人频繁@bot），可后续优化：
  - 读写分离：recall 不加锁，只在 save 时加锁
  - 或高频群聊 fallback 到 per-message session + AutoChatContext

**结论：当前设计可接受，作为已知 trade-off 记录。**

### 5.6 群聊 UserID 一致性

群聊共享 session 引入一个新问题：`session.Metadata["user_id"]` 只保存最后一个发言者的 ID。
coordinator 的 `resolveUserID()` 从 session metadata 取值时，可能拿到错误的 user ID。

**此问题在 `2026-01-30-memory-architecture-improvement.md` §4.1.4 中统一修复：**
coordinator 优先从 `ctx` 取 UserID（gateway 每次 request 设置的当前 senderID），而非 session metadata。

本方案确保 gateway 的 `handleMessage` 在每次请求中正确设置 `appcontext.WithUserID(ctx, senderID)`，
不依赖 session 持久化的旧值。

### 5.7 与长期记忆的关系

```
┌─────────────────────────────────────────────────────────────┐
│                     短期记忆 (Session)                       │
│  "你刚才说了什么" "上一步的结果" "我们正在讨论的方案"           │
│                                                             │
│  载体: session.Messages + historyMgr                        │
│  生命周期: 单次连续对话 (配置过期时间, 默认48h)                │
│  注入方式: preparation/history.go 自动加载                   │
│  压缩策略: > 70% MaxTokens 时 LLM 摘要                      │
├─────────────────────────────────────────────────────────────┤
│                     长期记忆 (Memory Store)                   │
│  "上周部署用了蓝绿策略" "用户偏好YAML格式" "上次报错的root cause"│
│                                                             │
│  载体: memory.Store (File/Postgres/Hybrid)                  │
│  生命周期: 永久 (或按类型 TTL)                                │
│  注入方式: MemoryRecallHook.OnTaskStart() 关键词+向量检索     │
│  写入时机: MemoryCaptureHook.OnTaskCompleted() 自动摘要       │
└─────────────────────────────────────────────────────────────┘

两者正交，不冲突:
- Session history = "这次对话聊了什么" (高保真, 有上下文)
- Memory recall  = "历史上学到了什么" (摘要化, 跨会话)
```

## 6. 实现清单

### Step 0: 前置验证（§4）

```
文件: 无代码改动
动作: 验证 historyMgr、SaveSessionAfterExecution、sessionStore.Save 在 Lark 路径可用
测试: 编写验证用例（§4 表格中列出的 5 项）
阻塞: 如果任一验证失败，先修复再进入 Step 1
```

### Step 1: 改 session ID 生成 + hash 长度（核心）

```
文件: internal/channels/lark/gateway.go
改动:
  - sessionID = memoryID (stable)
  - 支持 `lark.session_mode`（stable|fresh）便于回滚
  - memoryIDForChat hash[:8] → hash[:12] (1 行)
测试: gateway_test.go 新增/更新
```

### Step 2: 添加 session 过期检测（preparation 层）

```
文件: internal/agent/app/preparation/service.go
改动: ~20 行 (UpdatedAt 检查 + 过期前记忆保存 + 返回空历史)
配置: agent.session_stale_after (全局配置, 默认 48h)
测试:
  - preparation_test.go: session 过期后返回空历史
  - preparation_test.go: 过期前 memoryCaptureFunc 被调用
  - preparation_test.go: session 未过期时正常加载历史
  - preparation_test.go: session 过期后写回 store（Messages 被清空）
```

### Step 3: AutoChatContext 仅群聊启用

```
文件: internal/channels/lark/gateway.go
改动: 1 行 (加 `&& isGroup` 条件)
测试: gateway_test.go
```

### Step 4: 最小 /reset 命令

```
文件: internal/channels/lark/gateway.go
改动: ~10 行
```

稳定 session 上线后，用户没有任何手段清除错误上下文。最小实现：

```go
// handleMessage 中，在 ExecuteTask 之前检测
if strings.TrimSpace(content) == "/reset" {
    session, _ := g.sessionStore.Load(ctx, sessionID)
    if session != nil {
        session.Messages = nil
        session.Metadata = nil
        session.UpdatedAt = g.now()
        g.sessionStore.Save(ctx, session)
    }
    if g.historyMgr != nil {
        g.historyMgr.ClearSession(ctx, sessionID)
    }
    g.replyText(ctx, chatID, "已清空对话历史，下次对话将从零开始。")
    return
}
```

> 如果 `historyMgr` 没有 `ClearSession` 方法，需要在 `history_manager.go` 中新增。

### Step 5: 验证

```
- go test ./internal/channels/lark/...
- go test ./internal/agent/app/coordinator/...
- go test ./internal/agent/app/preparation/...
- 手动: Lark P2P 发2条消息，验证第2条看到第1条的上下文
- 手动: Lark 群聊发消息，验证 AutoChatContext 仍注入群聊记录
- 手动: Lark 发 /reset，验证下一条消息不包含之前的历史
- 手动: /reset 后 historyMgr.Replay 不再返回旧 turn
- 手动: 等待 48h（或临时调低阈值），验证过期后 session 自动清空
```

## 7. Rollback Plan

- **稳定 session 回滚**：新增 `lark.session_mode: stable|fresh`（默认 stable）；出现群聊并发或串台问题时切回 `fresh`。
- **过期逻辑回滚**：将 `agent.session_stale_after=0`（禁用），保留历史不清空。
- **/reset 回滚**：仅移除 `/reset` 处理分支，不影响主流程。

## 8. 参数推荐

| 参数 | 推荐值 | 说明 |
|------|--------|------|
| `session_stale_after` | **48h** | 超过 48h 未活动的 session 清空历史（覆盖"昨天"追问场景） |
| `session_mode` | **stable** | `stable`=复用 session；`fresh`=每条消息新 session（回滚开关） |
| `AutoChatContextSize` | **25** | 仅群聊时使用，20-30 条合理 |
| `MaxTokens` 历史占比阈值 | **70%** | 现有值，保持不变 |
| `historySummaryMaxTokens` | 现有值 | LLM 摘要的输出上限，保持不变 |
| `memoryIDForChat` hash 长度 | **12 字节** | 从 8 字节升级，降低碰撞概率 |

## 9. Edge Cases

| 场景 | 行为 | 备注 |
|------|------|------|
| Bot 重启后第一条消息 | EnsureSession 拿到持久化的旧 session | historyMgr 从 DB/文件恢复 ✅ |
| 超长对话 (100+ 轮) | shouldSummarizeHistory 触发 LLM 摘要 | 压缩为 2-3 段 ✅ |
| 并发消息 (同一 chat) | sessionLock 串行化 | 已有机制 ✅（群聊高并发见 §5.5） |
| AppendTurn 前缀不一致 | 历史被清空 | 需验证并修复（见 §4 前置验证） |
| 48h 无活动后发消息 | session 过期清空，关键信息存入长期记忆 | 新增机制（§5.3） |
| 用户说"忘掉之前的" | 发送 `/reset` 清空 session | 新增机制（§6 Step 4） |
| 群聊中多用户交替提问 | 共享 session，bot 看到全部历史 | 期望行为 |
| 群聊话题切换但仍在 48h 内 | 旧话题仍保留 | 已知 trade-off；后续可基于话题相关性清理 |
| 群聊中记忆 save/recall 的 UserID | 从 ctx 取当前 senderID，不从 session metadata | 修复（§5.6） |
| historyMgr 不可用 | 降级到 session.Messages (仅当前轮) | 已有 fallback ✅ |
| memoryIDForChat hash 碰撞 | 两个群聊共享 session（对话串台） | 12 字节 hash 后概率极低 ✅ |
| session 过期时保存长期记忆失败 | 忽略错误，仍然清空 session（降级可接受） | 日志告警 |

## 10. 不在本方案范围内

- **长期记忆优化**（检索排序、HybridStore 启用）→ 见 `2026-01-30-memory-architecture-improvement.md`
- **记忆路径统一**（去掉 larkMemoryManager 冗余）→ 同上 Phase 1（本方案完成后执行）
- **向量检索**（semantic search）→ 同上 Phase 2
- **群聊中按用户隔离 session** → 后续按需
- **群聊高并发优化**（读写分离锁）→ 后续按需（§5.5 已记录 trade-off）
