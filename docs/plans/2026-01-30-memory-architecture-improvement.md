# Memory Architecture Improvement Plan

> **Status:** Draft
> **Author:** cklxx
> **Created:** 2026-01-30
> **Updated:** 2026-01-30

> **⚠️ 执行依赖（更新）：** 源码验证表明 `larkMemoryManager` 在 `gateway.handleMessage` 中已 **dormant**。
> Phase 1 不再依赖 Plan 2 才能“删除调用”，但**推荐部署顺序**仍为：
> ```
> Plan 2（稳定 session ID）→ Plan 1 Phase 1（ConversationCaptureHook + 清理 dormant 代码）
> → Plan 1 Phase 2-4
> ```
> 原因：Plan 2 先提供稳定短期上下文，Phase 1 再引入长期对话摘要更稳妥。

## 1. Problem Statement

elephant.ai 的记忆系统存在以下结构性问题：

| 问题 | 影响 | 严重度 |
|------|------|--------|
| Lark gateway 与 hooks 层存在**重复记忆路径** | 同一条消息被两套系统各存一次，检索时互相看不到 | 高 |
| 检索无相关性排序（keyword store 纯 `ORDER BY created_at DESC`） | 返回最新而非最相关的记忆，浪费 context slot | 高 |
| 存储无摘要压缩 — 原始对话全文直接写入 | 记忆膨胀快，检索噪声大 | 中 |
| 无记忆过期/衰减机制 | 旧记忆永远占位，新记忆被挤出 top-5 | 中 |
| HybridStore 已实现但未在生产环境启用 | RRF 排序能力闲置 | 中 |
| Lark 频道的 `larkMemoryManager` 与 coordinator hooks 是两套独立体系 | 维护成本高，行为不一致 | 中 |
| 无对话摘要窗口（每次全量 token 压缩） | 长对话时 token 消耗高 | 低 |

## 2. Current Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      Lark Gateway                       │
│  handleMessage()                                        │
│    ├─ agent.ExecuteTask(ctx, task, session, listener)   │
│    │    └─ Coordinator.ExecuteTask()                    │
│    │         ├─ MemoryRecallHook.OnTaskStart()       ①  │
│    │         ├─ ReactEngine.SolveTask()                 │
│    │         │    └─ memory_recall tool (ctx.UserID) ②  │
│    │         └─ MemoryCaptureHook.OnTaskCompleted()  ③  │
│    └─ (larkMemoryManager 已注入但未被调用)             │
└─────────────────────────────────────────────────────────┘

问题：当前记忆路径主要集中在 hooks + tools；`larkMemoryManager` 为 dormant 代码，
     但仍造成架构认知偏差与维护成本。
```

### 2.1 记忆条数现状

| 参数 | 位置 | 现值 | 说明 |
|------|------|------|------|
| `RecallForTask` Limit | `lark/memory.go` | 5 | channel 层检索上限 |
| `MaxRecalls` | `hooks/memory_recall.go` | 5 | hooks 层检索上限 |
| `AutoChatContextSize` | `lark/config.go` | 20 (max 50) | Lark 近期聊天消息数 |
| 历史摘要阈值 | `preparation/history.go` | 70% of MaxTokens | 触发压缩的 token 占比 |
| 记忆去重阈值 | `hooks/memory_capture.go` | 0.85 (Jaccard) | 自动捕获去重 |
| 记忆总量 | 无限制 | ∞ | 无 TTL、无上限 |

## 3. Target Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Channel Layer (Lark/WeChat/Web)            │
│  handleMessage()                                        │
│    ├─ id.WithUserID(ctx, senderID)                     │
│    ├─ AutoChatContext (last 20-30 raw msgs)         [A] │
│    └─ agent.ExecuteTask(ctx, task, session, listener)   │
│                                                         │
│              Coordinator Layer (unified)                │
│         ├─ MemoryRecallHook.OnTaskStart()           [B] │
│         │    └─ HybridStore.Search() with RRF          │
│         ├─ ReactEngine.SolveTask()                     │
│         │    └─ memory_recall / memory_save tools    [C] │
│         ├─ MemoryCaptureHook.OnTaskCompleted()      [D] │
│         │    └─ Summary extraction + dedup             │
│         └─ ConversationCaptureHook.OnTaskCompleted() [E] │
│              └─ Save condensed turn pair               │
└─────────────────────────────────────────────────────────┘

[A] Short-term: raw Lark messages, 纯时序，用于当前话题连贯性
[B] Long-term recall: HybridStore (keyword + vector), RRF 排序
[C] Explicit: 用户/LLM 主动调用的记忆读写
[D] Auto-capture: 有工具调用的任务自动摘要
[E] NEW: 对话轮次自动存摘要（替代 larkMemoryManager.SaveMessage）
```

### 3.1 Key Differences from Current

1. **去掉 `larkMemoryManager` 的 Save/Recall**，统一到 coordinator hooks
2. **启用 HybridStore**，所有 Recall 走 RRF 排序
3. **新增 ConversationCaptureHook**，替代 channel 层的原始消息存储
4. **记忆分层**：chat_turn / auto_capture / workflow_trace / user_explicit（配合 scope=user|chat）

## 4. Implementation Phases

### Phase 1: 统一记忆路径（消除冗余）

**目标：** 去掉 Lark gateway 中 `larkMemoryManager` 的 Save/Recall 调用，让所有记忆操作走 coordinator hooks。

**变更清单：**

#### 4.1.1 新增 ConversationCaptureHook

**设计原则：每轮对话存一条 turn pair entry**（而非 user/assistant 各存一条），原因：
- 检索时保留对话上下文（user 问了什么 → assistant 回了什么）
- `max_recalls=8` 的预算不被一轮对话占掉 2 条
- 去重逻辑只需对一条 entry 做 Jaccard 检查

**摘要策略：** 短文本直接存原文，长文本用"头+尾"保留关键信息（避免纯截断丢失结论/决策）。
LLM 摘要延后到 Phase 3，Phase 1 不引入额外 LLM 调用开销。

```go
// internal/agent/app/hooks/conversation_capture.go

type ConversationCaptureHook struct {
    memoryService memory.Service
    logger        logging.Logger
}

func (h *ConversationCaptureHook) OnTaskCompleted(ctx context.Context, result TaskResultInfo) error {
    userID := result.UserID
    if userID == "" {
        return nil
    }

    input := smartTruncate(result.TaskInput, 250)
    answer := smartTruncate(result.Answer, 400)
    if input == "" && answer == "" {
        return nil
    }

    // 合并为单条 turn pair entry
    content := fmt.Sprintf("User: %s\nAssistant: %s", input, answer)
    h.memoryService.Save(ctx, memory.Entry{
        UserID:  userID,
        Content: content,
        Slots: map[string]string{
            "type":       "chat_turn",
            "scope":      "user", // 群聊可额外写一条 scope=chat
            "channel":    result.Channel,
            "chat_id":    result.ChatID,
            "session_id": result.SessionID,
            "sender_id":  result.SenderID,
            "thread_id":  result.ThreadID, // optional
            "source":     "conversation_capture",
        },
    })
    return nil
}

// smartTruncate 保留头部和尾部信息，避免纯截断丢失结论。
// 短于 maxLen 时返回原文；长于时取前 60% + " ... " + 后 40%。
func smartTruncate(s string, maxLen int) string {
    s = strings.TrimSpace(s)
    if len([]rune(s)) <= maxLen {
        return s
    }
    runes := []rune(s)
    headLen := maxLen * 6 / 10
    tailLen := maxLen - headLen - 5 // 5 for " ... "
    return string(runes[:headLen]) + " ... " + string(runes[len(runes)-tailLen:])
}
```

**Slots schema（统一约束，便于过滤与 TTL）：**

| Slot | 说明 | 备注 |
|------|------|------|
| `type` | 记忆类型（`chat_turn`/`auto_capture`/`workflow_trace`/`user_explicit`） | 必填 |
| `scope` | 记忆归属（`user`/`chat`） | 必填 |
| `channel` | `lark`/`wechat`/`cli`/`web` | 必填 |
| `chat_id` | 群聊/私聊 ID | 可空 |
| `session_id` | 短期 session ID | 可空 |
| `sender_id` | 当前发言者 | `scope=user` 时必填 |
| `thread_id` | 线程/话题 ID | 可空 |
| `source` | 产生来源（`conversation_capture`/`memory_capture`/`tool`） | 必填 |

**Scope policy（与“主动性 AI”一致）：**
- **默认 user-scope**：用户偏好/个人上下文只归属到该用户。
- **群聊追加 chat-scope**：在群聊中，同一条 turn pair 额外写一条 `scope=chat`（受 `capture_group_memory` 开关控制），用于共享决策与群级上下文。
- **召回顺序**：先 user-scope，再 chat-scope，小额度补充，防止群聊噪声淹没个人偏好。

#### 4.1.2 精简 Lark gateway `handleMessage`

```diff
 // gateway.go handleMessage()

-// Save user message to memory for long-term recall.
-if g.memoryMgr != nil {
-    g.memoryMgr.SaveMessage(execCtx, senderID, "user", content, senderID)
-}

 // Memory recall 移除 — 由 coordinator hooks 统一处理
-if g.memoryMgr != nil {
-    if recalled := g.memoryMgr.RecallForTask(execCtx, senderID, content); recalled != "" {
-        taskContent = recalled + "\n\n" + taskContent
-    }
-}

 result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

-// Memory save 移除 — 由 coordinator hooks 统一处理
-if g.memoryMgr != nil {
-    g.memoryMgr.SaveFromResult(execCtx, senderID, result)
-}
-if g.memoryMgr != nil && result != nil && strings.TrimSpace(result.Answer) != "" {
-    g.memoryMgr.SaveMessage(execCtx, senderID, "assistant", result.Answer, "bot")
-}
```

**保留：** `AutoChatContext` 不变 — 它获取的是 Lark 原始聊天记录（短期上下文），与记忆系统（长期）不重叠。

#### 4.1.3 灰度切换 — Path Mode

直接删除 `larkMemoryManager` 调用是不可逆的。如果 coordinator hooks 路径有 bug，Lark 完全失去记忆能力。

**新增 `proactive.memory.path_mode`（显式三态，避免歧义）：**

```yaml
# config.yaml
proactive:
  memory:
    path_mode: hooks_only # hooks_only | legacy_only | dual_write
```

```diff
 // gateway.go handleMessage()
+if g.cfg.Proactive.Memory.PathMode != "hooks_only" {
+    // Legacy path for Lark only (guarded by path_mode)
     if g.memoryMgr != nil {
         g.memoryMgr.SaveMessage(execCtx, senderID, "user", content, senderID)
     }
     if g.memoryMgr != nil {
         if recalled := g.memoryMgr.RecallForTask(execCtx, senderID, content); recalled != "" {
             taskContent = recalled + "\n\n" + taskContent
         }
     }
+}

 result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)

+if g.cfg.Proactive.Memory.PathMode == "legacy_only" || g.cfg.Proactive.Memory.PathMode == "dual_write" {
     if g.memoryMgr != nil {
         g.memoryMgr.SaveFromResult(execCtx, senderID, result)
     }
     // ...
+}
```

**切换流程：**
1. 测试环境 `path_mode: hooks_only` → 验证 hooks-only 路径
2. 生产灰度 `path_mode: dual_write`（legacy 写入 + hooks 读写）→ 观察 1 周 recall 命中率
3. 确认无退化后切回 `hooks_only`，再删除 legacy 代码与 flag

**语义说明：**
- `hooks_only`：禁用 Lark legacy 路径（推荐默认）。
- `legacy_only`：仅 Lark 走 legacy，hooks 在 Lark 上禁用（用于紧急回滚）。
- `dual_write`：Lark legacy + hooks 同时写入，但**只用 hooks 召回**，避免重复 recall。

#### 4.1.4 UserID 解析修复

当前 `coordinator.resolveUserID()` 从 `session.Metadata["user_id"]` 取值，fallback 到 `session.ID`。
在群聊稳定 session 下（Plan 2），`Metadata["user_id"]` 可能是上一个发言者而非当前发言者。

**修复：coordinator 优先从 ctx 取 UserID，而非 session metadata。**

```diff
 // coordinator.go resolveUserID()
-func (c *AgentCoordinator) resolveUserID(session *storage.Session) string {
-    if session == nil || session.Metadata == nil {
-        return ""
-    }
-    if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
-        return uid
-    }
+func (c *AgentCoordinator) resolveUserID(ctx context.Context, session *storage.Session) string {
+    // 优先从 ctx 取（gateway 每次 request 设置的当前 senderID）
+    if uid := appcontext.UserIDFromContext(ctx); uid != "" {
+        return uid
+    }
+    // Fallback: session metadata（兼容非 channel 场景）
+    if session != nil && session.Metadata != nil {
+        if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
+            return uid
+        }
+    }
     if strings.HasPrefix(session.ID, "lark-") || strings.HasPrefix(session.ID, "wechat-") {
         return session.ID
     }
     return ""
 }
```

此修复确保群聊中每条消息的记忆 save/recall 使用正确的 senderID，而非 session 持久化的旧值。

#### 4.1.5 注册新 hook

```go
// internal/di/container_builder.go buildHookRegistry()
if memoryService != nil && b.config.Proactive.Memory.Enabled {
    // 已有: recallHook, captureHook
    // 新增:
    if b.config.Proactive.Memory.CaptureMessages {
        convHook := hooks.NewConversationCaptureHook(memoryService, b.logger)
        registry.Register(convHook)
    }
}
```

**验证：**
- `go test ./internal/channels/lark/...`
- `go test ./internal/agent/app/hooks/...`
- 集成测试：Lark 发消息 → 验证记忆只存一份 → 下次对话能 recall

---

### Phase 2: 启用 HybridStore（相关性排序）

**目标：** 让所有 Recall 走 HybridStore 的 RRF 排序，而非纯时间序。

**前置条件：** embedder 配置可用（OpenAI text-embedding-3-small 或本地模型）

#### 4.2.1 配置激活

```yaml
# config.yaml
proactive:
  memory:
    enabled: true
    store: hybrid   # 从 "auto" 改为 "hybrid"
    max_recalls: 8  # 从 5 提升到 8（RRF 有排序，可以多取一些）
    hybrid:
      alpha: 0.4          # 偏向关键词（0.4 vector + 0.6 keyword）
      min_similarity: 0.3  # 降低门槛让更多候选进入 RRF
      persist_dir: data/memory/vector
      collection: elephant_memory
      embedder_model: text-embedding-3-small
```

#### 4.2.2 Alpha 调参指南

| alpha | 行为 | 适用场景 |
|-------|------|----------|
| 0.0 | 纯关键词 | 精确查找，如"上次部署的版本号" |
| 0.3-0.4 | 关键词主导，向量辅助 | **推荐起始值** — 兼顾精确匹配和语义扩展 |
| 0.5 | 均衡 | 记忆条目已有丰富语义描述时 |
| 0.7-1.0 | 向量主导 | 记忆全是自然语言摘要（无结构化标签） |

**推荐 alpha=0.4：**
- 当前记忆条目含结构化 slots（type, role, tool_sequence），关键词匹配有效
- 向量作为补充，捕获同义词和语义相近的记忆
- 随着记忆质量提升（Phase 3 摘要化），可逐步提高到 0.5

#### 4.2.3 DI 层改造

当前 `container_builder.go` 已有 HybridStore 的构建逻辑（通过 `store: hybrid` 配置），只需确认：
1. Embedder 初始化路径可用
2. VectorStore persist dir 自动创建
3. 降级策略：embedder 不可用时自动 fallback 到纯 keyword store

**验证：**
- 单元测试：`go test ./internal/memory/...` — 特别是 `hybrid_store_test.go`
- 对比测试：相同 query，keyword-only vs hybrid 的返回结果排序差异
- 延迟测试：hybrid recall 的 p95 延迟 < 200ms
- **降级测试（必须覆盖）：**
  - mock embedder 返回 error → 确认 `HybridStore.Search` 降级到 keyword-only，不报错
  - mock embedder 返回 nil/空 embedding → 同上
  - 不配置 embedder（`embedder_model` 为空）→ 确认系统正常启动，recall 走 keyword store
  - embedder 超时（mock 3s delay）→ 确认不阻塞 recall，降级返回 keyword 结果

---

### Phase 3: 记忆质量优化

**目标：** 提升存入记忆的信息密度，降低检索噪声。

#### 4.3.1 摘要压缩（升级 Phase 1 的 smartTruncate）

Phase 1 的 `ConversationCaptureHook` 使用 `smartTruncate`（头+尾保留），已优于纯截断。
Phase 3 进一步升级为 **LLM 摘要提取**，从原文中提取关键事实/决策/偏好。

触发条件：
- 短对话（< 200 chars）：直接存原文（不调 LLM）
- 长对话（≥ 200 chars）：调用小模型提取摘要

```go
type ConversationCaptureConfig struct {
    Enabled         bool
    UseLLMSummary   bool   // Phase 3 开启; Phase 1 为 false，走 smartTruncate
    MaxRawLength    int    // 超过此长度才做 LLM 摘要，默认 200
    SummaryModel    string // 用小模型做摘要（如 gpt-4o-mini）
}
```

升级后 `ConversationCaptureHook.OnTaskCompleted` 的摘要路径：
```
input/answer 长度 < MaxRawLength → 原文存储
input/answer 长度 ≥ MaxRawLength && UseLLMSummary → LLM 提取关键事实
input/answer 长度 ≥ MaxRawLength && !UseLLMSummary → smartTruncate（Phase 1 行为，作为 fallback）
```

#### 4.3.2 记忆去重增强

当前去重仅在 `MemoryCaptureHook` 中，且只用 Jaccard 系数。改进：

1. **存储时去重**：`ConversationCaptureHook` 也做 Jaccard 去重（threshold=0.8）
2. **合并更新**：相同话题的记忆不新增条目，而是更新已有条目的 content
3. **按 slot 类型分区去重**：`chat_turn` 类型按时间窗口去重（同一小时内相似度 > 0.7 的合并）

#### 4.3.3 记忆过期/衰减

新增 TTL 或衰减机制，避免旧记忆永远占位：

| 记忆类型 | 建议 TTL | 理由 |
|----------|----------|------|
| 记忆类型 | Scope | 建议 TTL | 理由 |
|----------|-------|----------|------|
| `chat_turn` | `user` | 30 天 | 对话级记忆时效性强 |
| `chat_turn` | `chat` | 14 天 | 群聊噪声更高，保留周期更短 |
| `auto_capture` | `user`/`chat` | 90 天 | 任务摘要有中期参考价值 |
| `workflow_trace` | `user` | 180 天 | 工作流模式识别需要更长窗口 |
| `user_explicit` | `user` | 永不过期 | 用户主动要求记住的信息 |

**实现方式：Store 层自动过滤（支持 per-type + per-scope）**

选择理由：
- 无需额外 goroutine/cron 组件
- 保证每次检索都只看到有效记忆
- 物理保留数据（不删除），逻辑过滤，支持未来"显式恢复"
- Cron 清理作为可选补充（控制存储空间增长，P3 优先级）

在 `memory.Query` 上新增 `Slots` + `MinCreatedAt` 字段，由 hooks 按 type+scope 计算：

```go
// memory/types.go
type Query struct {
    // ... existing fields
    Slots        map[string]string // 精确匹配（type/scope/channel/...）
    MinCreatedAt time.Time          // 仅返回此时间之后的记忆，零值表示不过滤
}

// hooks/memory_recall.go OnTaskStart()
ttlByTypeScope := map[string]time.Duration{
    "chat_turn:user":      30 * 24 * time.Hour,
    "chat_turn:chat":      14 * 24 * time.Hour,
    "auto_capture:user":   90 * 24 * time.Hour,
    "auto_capture:chat":   90 * 24 * time.Hour,
    "workflow_trace:user": 180 * 24 * time.Hour,
    // "user_explicit:user": 不设 TTL
}

// 采用“按类型/Scope 分批查询 + RRF 合并”的方式避免 TTL 冲突
// 每个 query 带 Slots + MinCreatedAt
queries := []memory.Query{
    {Slots: map[string]string{"type": "user_explicit", "scope": "user"}, Limit: 2},
    {Slots: map[string]string{"type": "chat_turn", "scope": "user"}, Limit: 2, MinCreatedAt: now.Add(-30 * 24 * time.Hour)},
    {Slots: map[string]string{"type": "chat_turn", "scope": "chat"}, Limit: 2, MinCreatedAt: now.Add(-14 * 24 * time.Hour)},
    {Slots: map[string]string{"type": "auto_capture", "scope": "user"}, Limit: 3, MinCreatedAt: now.Add(-90 * 24 * time.Hour)},
    {Slots: map[string]string{"type": "auto_capture", "scope": "chat"}, Limit: 1, MinCreatedAt: now.Add(-90 * 24 * time.Hour)},
    {Slots: map[string]string{"type": "workflow_trace", "scope": "user"}, Limit: 1, MinCreatedAt: now.Add(-180 * 24 * time.Hour)},
}

// Store 层 Search 实现中加 WHERE 条件（Slots + created_at）
// keyword_store.go: WHERE created_at >= $minCreatedAt AND slots @> $slots
// hybrid_store.go:
//   - keyword: 同上
//   - vector: 若支持 metadata filter 则下推，否则 post-filter + 过采样补齐
```

可选的存储清理（低优先级；用于控制存储增长）：
```go
// 后台 goroutine，每天执行一次
func (s *Store) PurgeExpired(ctx context.Context, ttlByType map[string]time.Duration) (int, error)
```

---

### Phase 4: 参数调优与可观测性

#### 4.4.1 推荐参数总表

| 参数 | 推荐值 | 现值 | 变更理由 |
|------|--------|------|----------|
| `max_recalls` | **8** | 5 | HybridStore 有排序，可多取 |
| `AutoChatContextSize` | **25** | 20 | 多覆盖一些上下文，不超过 30 |
| `dedupe_threshold` | **0.80** | 0.85 | 适当降低，减少近似重复 |
| `hybrid.alpha` | **0.4** | 0.6 | 偏向关键词，向量辅助 |
| `hybrid.min_similarity` | **0.3** | 0.7 | 降低向量门槛，让 RRF 排序 |
| `capture_messages` | **true** | false | 开启对话摘要存储 |

#### 4.4.2 可观测性指标

在 hooks 层添加 structured log / metrics：

```go
// 每次 recall 记录
logger.Info("memory_recall",
    "user_id", userID,
    "keywords", keywords,
    "results_count", len(entries),
    "latency_ms", elapsed.Milliseconds(),
    "store_type", storeType,  // "keyword" | "hybrid"
)

// 每次 capture 记录
logger.Info("memory_capture",
    "user_id", userID,
    "entry_key", saved.Key,
    "content_len", len(summary),
    "dedup_skipped", isDuplicate,
)
```

#### 4.4.3 调参建议流程

```
1. 启用 hybrid store，alpha=0.4
2. 收集 1 周的 recall 日志
3. 分析：
   - recall 命中率（返回结果 > 0 的比例）
   - 用户实际使用了哪些 recalled 记忆（通过 LLM 的引用行为判断）
   - p95 延迟
4. 根据数据调整 alpha 和 max_recalls
```

## 5. Unified Config Schema

两个方案（本方案 + short-term-multi-turn-memory）合计新增多个配置字段。统一设计避免后续 Phase 间不兼容：

```yaml
# config.yaml — 完整的记忆相关配置
proactive:
  memory:
    enabled: true
    store: hybrid                    # file | postgres | hybrid | auto
    path_mode: hooks_only            # hooks_only | legacy_only | dual_write
    max_recalls: 8                   # Phase 2: HybridStore 有排序，可多取
    auto_recall: true
    auto_capture: true
    capture_messages: true           # Phase 1: 开启 ConversationCaptureHook
    capture_group_memory: true       # 群聊写入 scope=chat 记忆（小额度召回）
    dedupe_threshold: 0.80           # 降低去重门槛，减少近似重复
    hybrid:
      alpha: 0.4                     # Phase 2: 偏向关键词，向量辅助
      min_similarity: 0.3            # Phase 2: 降低向量门槛，让 RRF 排序
      persist_dir: data/memory/vector
      collection: elephant_memory
      embedder_model: text-embedding-3-small
    capture:
      use_llm_summary: false         # Phase 3 开启
      max_raw_length: 200            # Phase 3: 超过此长度触发 LLM 摘要
      summary_model: ""              # Phase 3: 小模型做摘要
    ttl:                              # Phase 3: 记忆过期（按 type + scope）
      chat_turn_user: 720h           # 30 天
      chat_turn_chat: 336h           # 14 天
      auto_capture_user: 2160h       # 90 天
      auto_capture_chat: 2160h       # 90 天
      workflow_trace_user: 4320h     # 180 天
      # user_explicit: 不配置 = 永不过期
    recall_quota:                    # 按类型 + scope 限额（避免噪声淹没）
      user_explicit: 2
      chat_turn_user: 2
      chat_turn_chat: 2
      auto_capture_user: 3
      auto_capture_chat: 1
      workflow_trace_user: 1

agent:
  session_stale_after: 48h           # Plan 2: session 过期时间（全局，非 channel-specific）
```

## 6. Execution Priority

| 阶段 | 优先级 | 依赖 | 风险 |
|------|--------|------|------|
| **Plan 2: 稳定 Session ID** | **P0** | 无 | 低 — 一行改动 |
| Phase 1: 统一记忆路径 | **P0** | Plan 2 完成 | 低 — 删除冗余代码（有 feature flag 灰度） |
| Phase 2: 启用 HybridStore | **P1** | embedder 配置 | 中 — 需要 embedding API |
| Phase 3: 记忆质量优化 | **P2** | Phase 1 | 中 — LLM 摘要有成本 |
| Phase 4: 参数调优 | **P2** | Phase 2 | 低 — 纯配置变更 |

## 7. 关键设计决策

### Q1: 为什么不用滑动窗口而用 token 压缩？

elephant.ai 的每条 Lark 消息都创建一个 fresh session（零历史），不存在传统意义上的"多轮对话窗口"。短期上下文通过 `AutoChatContext` 注入 Lark 原始聊天记录，长期上下文通过 memory recall 注入。这种架构下滑动窗口无意义。

### Q2: max_recalls 为什么选 8 而不是更多？

- "Lost in the Middle" 效应：LLM 对中间内容关注度低，超过 8 条 recalled memories 命中率边际递减
- token 预算：8 条记忆 × 平均 200 tokens = ~1600 tokens，占 128K 窗口的 ~1.2%，合理
- HybridStore RRF 排序保证 top-8 的质量，优于无排序时的 top-5

### Q3: 为什么保留 AutoChatContext？

AutoChatContext 和 memory recall 解决不同问题：
- **AutoChatContext**：当前会话的原始聊天流（"刚才说了什么"），短期、高保真
- **Memory recall**：跨会话的知识和偏好（"上周部署用了什么策略"），长期、摘要化

两者互补，不应合并。

### Q4: Lark 的 larkMemoryManager 应该完全删除吗？

Phase 1 后，`larkMemoryManager` 的 SaveMessage / RecallForTask / SaveFromResult 全部由 coordinator hooks 替代。但 `larkMemoryManager` 类型本身可以保留（作为未来 channel-specific 记忆逻辑的扩展点），只是不再在 gateway.handleMessage 中调用。

## 8. Risk Assessment

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| Embedding API 不可用 | 中 | HybridStore 降级为 keyword-only | `HybridStore.vectorSearch` 已有 nil 检查，自动降级；**新增降级测试用例** |
| 记忆迁移：旧 memoryID 的数据丢失 | 高 | 已知 — 旧 hash-based memoryID 的数据不可恢复 | 接受，因为旧系统本身就是坏的（user_id bug） |
| ConversationCaptureHook 增加延迟 | 低 | 每次 task 完成多 1 次 memory.Save（合并 turn pair） | Save 是异步的，不阻塞响应 |
| LLM 摘要增加成本（Phase 3） | 中 | 每次对话多一次小模型调用 | 仅长文本触发，可配置关闭 |
| **Phase 1 删除 larkMemoryManager 导致记忆中断** | 中 | Lark 短期无记忆能力 | **path_mode 灰度切换**（§4.1.3） |
| **群聊 UserID 交叉污染** | 高 | 不同用户的记忆互相覆盖 | **coordinator 从 ctx 取 UserID**（§4.1.4） |
| **群聊 chat-scope 噪声过大** | 中 | recall 被群聊噪声污染 | **scope=chat TTL 更短 + recall quota 限额**（§4.3.3） |

## 9. Success Metrics

分阶段设目标，避免早期数据不足时过度调参：

### Phase 1-2 上线第 1 周（基线建立）

| 指标 | 当前 | 目标 | 衡量方式 |
|------|------|------|----------|
| Memory recall 命中率 | 未知 | > **40%** | recall 返回 > 0 结果的比例 |
| Recall 结果被 LLM 实际引用 | 未知 | > **25%** | 检查 LLM 回复中是否引用了 recalled 内容 |
| Memory save 去重率 | ~15% | > **25%** | 被 Jaccard 去重跳过的比例 |
| Recall p95 延迟 | ~50ms | < **200ms** | 包含 hybrid search 的端到端延迟 |
| 记忆路径数量 | 3（channel + hooks + tool） | **1**（hooks only） | 代码路径计数 |

### Phase 3 摘要优化后（稳态目标）

| 指标 | 目标 | 说明 |
|------|------|------|
| Memory recall 命中率 | > **60%** | 摘要化记忆提升检索质量 |
| Recall 结果被 LLM 实际引用 | > **40%** | 信息密度提升后 LLM 更可能引用 |
| Memory save 去重率 | > **35%** | 增强去重 + 按时间窗口合并 |
| 记忆信息密度 | 平均 < **200 tokens/entry** | LLM 摘要压缩效果 |
