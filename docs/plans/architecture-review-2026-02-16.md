# 架构梳理报告 — 2026-02-16

## 健康度概览

| 维度 | 状态 | 说明 |
|------|------|------|
| 层间隔离 | ✅ 通过 | 0 policy violations，无循环依赖 |
| 代码规模 | ⚠️ 警告 | 609 Go 源文件，react/ 达 51 文件 920KB |
| 复杂度 | ⚠️ 需关注 | 4 个 God Struct，6 个 1000+ LOC 文件 |
| 设计一致性 | ❌ 需改进 | 事件系统碎片化、存储抽象重叠、Domain 层泄漏 |
| 测试覆盖 | ❌ 缺口 | react/ 56% 未测试，coordinator/ 60% 未测试 |

---

## P0 — 架构性问题

### 1. Domain 层泄漏 Lark 概念

**位置：** `internal/domain/agent/presets/tools.go:26`, `prompts.go`

Domain 层硬编码了 `ToolPresetLarkLocal` 和 Lark 操作指令。Domain 应 channel-agnostic，Lark 定制应在 delivery 层注入。

**影响：** 添加新 channel 时必须改动 domain 层。

**修复方向：** presets 提供 channel-agnostic 基础，Lark 定制通过 app/delivery 层的 prompt overlay 注入。

---

### 2. 事件系统三重包装

**位置：**
- `domain/agent/react/` → `emitEvent()` (domain events)
- `domain/workflow/` → `wf.AddListener()` (workflow events)
- `app/agent/hooks/` → `ProactiveHook` (hooks)
- `app/agent/coordinator/event_listener_wrapper.go` → `SerializingEventListener`
- `app/agent/coordinator/workflow_event_translator.go` → 941 LOC 翻译层

**问题：** 一个事件从 domain 到 delivery 经过 4 层嵌套 decorator：
```
emitEvent → SerializingEventListener → WorkflowEnvelope → SLAEnrichment → planTitleRecorder → downstream
```

新增 event type 需改 3-4 处。

**修复方向：** 合并为单一 EventDispatcher，内部 pipeline 处理排序+翻译+enrichment。

---

### 3. Context 管理职责重叠

**位置：**
- `internal/app/context/` — 压缩、Memory 注入、Prompt 构建
- `internal/domain/agent/react/` — attachment snapshot、message 构建

**问题：** `app/agent/context/attachments.go` 和 `domain/agent/react/attachments.go` (830 LOC) 都操作同一个 `ports.Attachment` 类型，无清晰所有权契约。

**修复方向：** 明确 attachment 操作归属——domain 拥有运行态变换，app 拥有持久化和注入。

---

### 4. Background Task 的 Context 传播陷阱

**位置：** `internal/domain/agent/react/background.go:162`

```go
taskCtx, cancel := context.WithCancel(context.Background())
```

用 `context.Background()` 创建 detached context，父 context 取消时 background task 不级联取消。context value 需手动逐个复制。

**修复方向：** 改用 `context.WithoutCancel(runCtx)` 或显式 value 继承包装器。

---

## P1 — God Struct / 复杂度热点

### God Struct 清单

| Struct | 文件 (LOC) | Fields | Methods | 核心问题 |
|--------|-----------|--------|---------|---------|
| AgentCoordinator | `coordinator/coordinator.go` (704) | 17 | 5+ | 23 个 import，混合 Session/Event/Cost/Hooks/Credential/Attachment |
| ReactEngine | `react/engine.go` | 29 | 15+ | 混合 Think-Act-Observe/checkpoint/attachment/workflow/background |
| reactRuntime | `react/runtime.go` (1,078) | 15+ 状态变量 | 33+ | Plan/Clarify/ReAct/FinalReview 状态机嵌入执行循环 |
| Gateway | `lark/gateway.go` (888) | 24 | 31 | SDK 桥接/Session Slot/Dedup/Input Relay/Multi-bot/Plan Review |

### 大文件 Top 5

| 文件 | LOC | 问题 |
|------|-----|------|
| `react/background.go` | 1,227 | Task 生命周期 + 事件路由 + 外部 Agent 协调 |
| `react/runtime.go` | 1,078 | `runIteration()` 30+ 决策分支 |
| `server/app/task_execution_service.go` | 987 | 混合 admission/lease/orphan recovery |
| `server/app/event_broadcaster.go` | 955 | SSE 序列化 + broadcast fanout |
| `coordinator/workflow_event_translator.go` | 941 | 事件类型翻译 switch-case |

### 拆分方向

- reactRuntime → 提取 `UIOrchestrator`（Plan/Clarify/FinalReview 状态机）
- BackgroundTaskManager → 提取 `TaskStateMachine`（生命周期与事件路由分离）
- Gateway → 提取 `SessionSlotManager`（Slot 生命周期与 SDK 桥接分离）

---

## P1 — 错误处理碎片化

**位置：** `internal/shared/errors/types.go`

定义了 `TransientError`/`PermanentError`/`DegradedError`，但：
- Retry (`toolregistry/retry.go`) 有自己的 `RetryableError`
- Degradation (`toolregistry/degradation.go`) 有独立的 fallback 机制
- 两者不联动

**修复方向：** 创建 `ErrorClassifier` 自动串联 error type → retry → degradation 管道。

---

## P1 — 存储抽象增殖

8 种独立存储抽象，SessionStore 和 HistoryManager 语义重叠：

| Store | 位置 | 职责 |
|-------|------|------|
| SessionStore | `ports/storage/session.go` | 会话消息、元数据 |
| HistoryManager | `ports/storage/history.go` | 对话轮次重放 |
| StateStore | `infra/session/state_store.go` | 动态上下文状态 |
| CostTracker | `ports/storage/cost.go` | Token 用量和费用 |
| CheckpointStore | `react/checkpoint.go` | ReAct 迭代快照 |
| AttachmentPersister | `ports/attachment_store.go` | 附件持久化 |
| IndexStore | `infra/memory/index_store.go` | 向量+全文检索 |
| EventHistoryStore | `delivery/server/app/` | 事件重放 |

**修复方向：** 澄清 Session vs History 边界，考虑 Repository 模式统一生命周期管理。

---

## P1 — 测试覆盖缺口

| Package | 未测试率 | 关键未测试文件 |
|---------|---------|--------------|
| react/ | 56% | `attachments.go` (836 LOC) 无测试 |
| coordinator/ | 60% | `coordinator.go` (704 LOC) 无直接单测 |
| infra/llm/ | 57% | `factory.go` (238 LOC) 无测试 |
| lark/ | 39% | Card handlers 部分无测试 |

---

## P2 — 设计气味

| 问题 | 位置 | 描述 |
|------|------|------|
| 单实现接口 | `ToolExecutionLimiter`, `ContextManager` | 只有一个实现且无 mock |
| Tool 执行 4 层包装 | `toolregistry/registry.go:113-119` | base → policy → breaker → degradation → SLA |
| Config 碎片 | 159 个 Config struct | 83 处独立 `yaml.Unmarshal`，无统一 schema 校验 |
| 消息格式化重复 | `formatter.go` + `larktools/chat_history.go` | 两处独立 switch-on-tool-name |
| 命名约定模糊 | 全局 | Manager/Service/Coordinator/Store 使用边界不清 |
| TaskStore 接口膨胀 | `domain/task/store.go` | 21 methods，应拆分为 CRUD/Progress/Lease/Audit |

---

## 改进路线图

### Phase 1：解耦与边界清理
1. 抽离 Domain 中的 Lark 引用 → presets channel-agnostic，Lark 定制推到 delivery 层
2. 明确 Context 所有权 → Attachment 操作归属 domain 或 app，不要两层都做
3. 统一错误管道 → `ErrorClassifier` 串联 retry → degradation

### Phase 2：拆分 God Struct
4. 从 reactRuntime 提取 `UIOrchestrator`
5. 从 BackgroundTaskManager 提取 `TaskStateMachine`
6. 从 Gateway 提取 `SessionSlotManager`

### Phase 3：事件与存储统一
7. 合并事件管道 → 单一 EventDispatcher 替代 4 层 decorator chain
8. 澄清 Session vs History 边界
9. 统一 Config 加载

### Phase 4：测试补全
10. react/background.go、coordinator.go、llm/factory.go 单测 → 目标 80%+

---

## 架构优势（保持）

- ✅ 层间隔离纪律良好：0 循环依赖，异常有到期日管理
- ✅ Port/Adapter 模式清晰：domain 定义 ports，infra 实现 adapters
- ✅ 多 Provider LLM：Factory + 5 层 Client 包装（流式/重试/限流/解析）
- ✅ Arch Check 自动化：pre-push CI + `scripts/check-arch.sh`
- ✅ Memory 混合检索：Vector + BM25 hybrid search，自动 recall 注入
