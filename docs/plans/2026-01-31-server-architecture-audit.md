# internal/server 架构审计与优化计划

**Created:** 2026-01-31
**Status:** Completed (Batches 1-6)
**Author:** cklxx + AI

---

## Execution Summary

| Batch | Phase | Description | Commit | Status |
|-------|-------|-------------|--------|--------|
| 1 | 4.2 | broadcasterMetrics → atomic | `b1dee5d6` | Done |
| 2 | 5.1 | Event type constants centralized | `f2f82936` | Done |
| 3 | 1.2 | Decouple EventBroadcaster ↔ TaskStore | `57ab32af` | Done |
| 4 | 3 | Structured startup (stages + SubsystemManager) | `aa2d8368` | Done |
| 5 | 1.1 | Split ServerCoordinator into 3 services | `7d0f2e07` | Done |
| 6 | 2 | Router refactor (RouterDeps/RouterConfig) | `ff3d09d3` | Done |

### Remaining items (not in scope for this round)
- Phase 4.1: InMemoryTaskStore TTL eviction
- Phase 5.2: LLM Health Probe actual detection
- Phase 5.3: WriteTimeout per-route handling
- Phase 5.4: Event drop observability & backpressure

---

## 1. 模块概览

`internal/server` 是 elephant.ai 的 HTTP API 服务层，约 18,900 行代码，分为 4 个子包：

| 子包 | 文件数 | 职责 |
|------|--------|------|
| `bootstrap/` | 18 | 服务器生命周期启动、配置加载、DI 组装 |
| `app/` | 22 | 核心应用逻辑：任务协调、事件广播、持久化 |
| `http/` | 50 | HTTP 路由、中间件、API Handler、SSE |
| `ports/` | 4 | 接口定义（Task、Health、Broadcaster、Session） |

---

## 2. 问题分类与深度分析

### 2.1 [严重] God Object: `ServerCoordinator`

**位置:** `app/server_coordinator.go:34-49` + 5 个扩展文件

**现状：** ServerCoordinator 持有 11 个外部依赖，横跨 6 个文件（~982 行），承担了以下所有职责：

- 异步任务执行与生命周期 (`server_coordinator_async.go`)
- Session CRUD 操作 (`server_coordinator_sessions.go`)
- 任务查询与取消 (`server_coordinator_tasks.go`)
- 快照与回放 (`server_coordinator_snapshots.go`)
- Share Token 管理 (`share.go`)
- Analytics 事件采集（散布在各文件中）
- Cancel function 管理（`cancelFuncs map + cancelMu`）

**问题本质：** 违反单一职责原则。每次新增功能都会增加 ServerCoordinator 的膨胀。测试需要 mock 11 个依赖。任务生命周期管理（cancel map）和 session 管理混在同一个 struct 中，无法独立演进。

**依赖拓扑：**
```
ServerCoordinator
├── AgentExecutor (agent 层)
├── EventBroadcaster (事件层)
├── SessionStore (存储层)
├── StateStore (状态层)
├── HistoryStore (历史层)
├── TaskStore (任务层)
├── Logger
├── Analytics (分析层)
├── JournalReader (日志层)
├── Observability (可观测层)
├── cancelFuncs map (运行时状态)
└── cancelMu (同步原语)
```

---

### 2.2 [严重] EventBroadcaster 与 TaskStore 的紧耦合

**位置:** `app/event_broadcaster.go:101-221`

**现状：** EventBroadcaster 在广播事件的同时同步更新 TaskStore 进度：

```go
// event_broadcaster.go:131-133
// 每个事件都触发 taskStore I/O
b.updateTaskProgress(baseEvent)  // 同步调用
```

`updateTaskProgress()` (L166-221) 对每个事件做 type switch，并对 `taskStore` 执行 `Get()` + `UpdateProgress()` 调用。在高频事件（如 streaming delta）下：

1. 每秒可能有 100+ 事件，每个都触发 TaskStore 的 mutex lock/unlock
2. `InMemoryTaskStore.UpdateProgress()` 持有全局 `sync.RWMutex`，所有 session 的任务更新都串行化
3. 如果 TaskStore 实现切换为数据库，广播路径将直接阻塞在 DB I/O 上

**设计违规：** EventBroadcaster 本应是纯事件分发器（fan-out），但它承担了副作用执行（task progress tracking）。这违反了关注点分离。

---

### 2.3 [严重] `RunServer()` 过程式启动函数

**位置:** `bootstrap/server.go:24-248`

**现状：** 248 行的单函数完成：
- 可观测性初始化
- 配置加载
- DI 容器构建
- 附件存储
- 事件历史存储
- EventBroadcaster + TaskStore 创建
- Analytics 客户端
- WeChat / Lark 网关
- Scheduler
- Session 迁移
- 健康检查
- Auth 服务
- HTTP 路由
- HTTP Server 启动

**问题：**

1. **静默失败（Fail-open）模式泛滥（共 9 处）：**
   - Container 关闭失败 → defer 中 `logger.Warn()` (L49-51)
   - `container.Start()` 失败 → `logger.Warn()`，继续运行 (L54-56)
   - 附件存储初始化失败 → `logger.Warn()`，跳过附件功能 (L64-65)
   - 事件历史 schema 初始化失败 → `logger.Warn()`，继续 (L85-87)
   - WeChat 网关启动失败 → `logger.Warn()`，服务禁用 (L128-131)
   - Lark 网关启动失败 → `logger.Warn()`，服务禁用 (L140-143)
   - Session 迁移失败 → `logger.Warn()` (L170-180)
   - 认证模块初始化失败 → `logger.Warn()`，鉴权禁用 (L187-193)
   - Evaluation 服务初始化失败 → `logger.Warn()`，服务禁用 (L203-206)

   这意味着服务器可以在半损坏状态下启动，且没有任何机制通知运维哪些子系统实际工作。

2. **defer 栈深度过大：** 函数中有 13 个 defer 语句，清理顺序隐含在书写位置中，难以追踪。

3. **Context 生命周期混乱：** 函数中有 6 个独立的 context/cancel 对，各自创建又各自取消，且 schedulerCancel 在 scheduler 未启用时也需要手动调用 (L164)。

---

### 2.4 [严重] `NewRouter()` 17 参数函数签名

**位置:** `http/router.go:20`

**函数签名：**
```go
func NewRouter(
    coordinator *app.ServerCoordinator,
    broadcaster *app.EventBroadcaster,
    healthChecker *app.HealthCheckerImpl,
    authHandler *AuthHandler,
    authService *authapp.Service,
    environment string,
    allowedOrigins []string,
    sandboxBaseURL string,
    configHandler *ConfigHandler,
    evaluationService *app.EvaluationService,
    obs *observability.Observability,
    maxTaskBodyBytes int64,
    streamGuard StreamGuardConfig,
    rateLimit RateLimitConfig,
    nonStreamTimeout time.Duration,
    attachmentCfg attachments.StoreConfig,
    memoryService memory.Service,
) http.Handler
```

**问题：**
- 17 个参数意味着路由组装了解所有实现细节
- 新增 handler 需要修改这个签名
- 无法部分初始化或做功能切换

---

### 2.5 [严重] 手工路由解析

**位置:** `http/router.go:234-322`

**现状：** 使用 `http.NewServeMux()` + 手工 `strings.TrimPrefix()` / `strings.HasSuffix()` 做路由匹配：

```go
// router.go:269-306
path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
if strings.HasSuffix(path, "/persona") { ... }
if strings.HasSuffix(path, "/snapshots") { ... }
if strings.Contains(path, "/turns/") { ... }
if strings.HasSuffix(path, "/replay") { ... }
if strings.HasSuffix(path, "/share") { ... }
if strings.HasSuffix(path, "/fork") { ... }
```

**问题：**
1. 每个新子路由都是一个 if 分支，容易遗漏或冲突
2. 路径参数提取靠 TrimPrefix 和位置推断，无类型安全
3. 方法路由（GET/POST/PUT/DELETE）靠手写 switch-case
4. Auth disabled 时注册 12 个 "service unavailable" 路由 (L147-162)，纯 boilerplate
5. `routeHandler()` 包装器和 `annotateRequestRoute()` 部分路由调用了两次

---

### 2.6 [中等] InMemoryTaskStore 无界增长

**位置:** `app/task_store.go:16-19`

```go
type InMemoryTaskStore struct {
    mu    sync.RWMutex
    tasks map[string]*ports.Task
}
```

**问题：**
- `tasks` map 只增不减（除非显式 Delete）
- `List()` 每次调用都对整个 map 做 copy + sort (L87-116)
- 长时间运行的服务器，任务记录会无限累积
- 没有 TTL、LRU、或最大容量限制

---

### 2.7 [中等] `broadcasterMetrics` 用 Mutex 而非 Atomic

**位置:** `app/event_broadcaster.go:55-62, 668-691`

```go
type broadcasterMetrics struct {
    mu sync.RWMutex
    totalEventsSent   int64
    droppedEvents     int64
    totalConnections  int64
    activeConnections int64
}

func (m *broadcasterMetrics) incrementEventsSent() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.totalEventsSent++
}
```

每次事件广播都要获取 mutex 来做简单的计数器递增。在高频事件场景（每秒数百事件）下，这是不必要的锁竞争。应使用 `atomic.Int64`。

---

### 2.8 [中等] `applyServerFileConfig()` 过程式配置合并

**位置:** `bootstrap/config.go:200-453`

253 行代码逐字段判空、trim、赋值。每次新增配置项都要新增 3-5 行 boilerplate。这个函数违反了 DRY 原则，且极易在新增字段时忘记处理。

---

### 2.9 [中等] LLMFactoryProbe 健康检查永远返回 Ready

**位置:** `app/health.go:122-134`

```go
func (p *LLMFactoryProbe) Check(ctx context.Context) ports.ComponentHealth {
    return ports.ComponentHealth{
        Name:    "llm_factory",
        Status:  ports.HealthStatusReady,
        Message: "LLM factory initialized",
    }
}
```

这个 probe 无论 LLM API 是否可达都返回 Ready，使得健康检查对 LLM 服务不可用的场景完全失效。至少应检测 container 是否持有有效的 LLM factory 引用，或做 lightweight ping。

---

### 2.10 [中等] 事件类型硬编码字符串

**位置:** 散布在多处

- `event_broadcaster.go:49`: `assistantMessageEventType = "workflow.node.output.delta"`
- `event_broadcaster.go:280`: `"workflow.result.final"`, `"workflow.result.cancelled"`
- `event_broadcaster.go:604`: `"workflow.node.output.delta"`, `"workflow.tool.progress"`
- SSE handler 中也有 allowlist map

没有单一来源定义事件类型常量。重构事件名时需要全局搜索替换。

---

### 2.11 [低] `ensureCriticalEventDelivery` 丢弃旧事件策略

**位置:** `app/event_broadcaster.go:240-273`

当 client buffer 满时，为了发送 critical 事件（如 `workflow.result.final`），从 channel 中 pop 一个旧事件丢弃。这可能导致：
- 前端丢失中间步骤的 tool output
- 无法确定哪个事件被丢弃
- 没有 backpressure 机制通知前端

---

### 2.12 [低] `WriteTimeout: 0` — 无限写超时

**位置:** `bootstrap/server.go:243`

```go
server := &http.Server{
    WriteTimeout: 0,
}
```

SSE stream 需要长连接，但 WriteTimeout=0 意味着非 SSE 路由也没有写超时保护。如果一个普通 API 请求的响应卡住，连接将永远不释放。

---

## 3. 架构优化方案

### Phase 1: 解耦核心对象（高收益，中风险）

#### 3.1.1 拆分 ServerCoordinator

将 ServerCoordinator 拆分为 3 个独立服务：

```
ServerCoordinator (现有)
    ↓ 拆分为
├── TaskExecutionService     — 异步任务执行、cancel 管理、analytics
├── SessionService           — Session CRUD、fork、persona、Share Token 管理
└── SnapshotService          — 快照列表、回放
```

每个服务只依赖它需要的存储层接口。APIHandler 注入各自独立的 service 而非一个巨型 coordinator。

**关键变更：**
- `cancelFuncs` map 移入 `TaskExecutionService`
- Analytics 采集通过 EventBus（或直接在 service 内）而非 coordinator 传递
- `executeTaskInBackground` 成为 `TaskExecutionService` 的私有方法

#### 3.1.2 EventBroadcaster 去除 TaskStore 副作用

将 `updateTaskProgress()` 从 EventBroadcaster 中移除。改为：

**方案 A（推荐）：独立 TaskProgressTracker**
```go
type TaskProgressTracker struct {
    taskStore serverPorts.TaskStore
    runMap    sync.Map  // sessionID -> taskID
}

// 作为独立 EventListener 注册
func (t *TaskProgressTracker) OnEvent(event agent.AgentEvent) {
    // 同步更新（首次实现保持行为不变，仅解耦职责）
    // 批量/去抖优化作为后续独立改进
}
```

在 `executeTaskInBackground` 中将 `TaskProgressTracker` 和 `EventBroadcaster` 组合为 `MultiEventListener`，两者并行接收事件但互不依赖。

**方案 B：事件管道**
EventBroadcaster 纯广播 → 下游 consumer 异步消费 progress 事件。但这增加了基础设施复杂度。

**推荐方案 A。**

**设计约束（补充）：**
- **一致性：** Task progress 只允许单调前进；对同一 task 使用 `lastSeq/lastAt` 防止乱序覆盖；最终事件（final/cancelled/error）必须同步落盘。
- **生命周期：** `runMap` 在任务完成/取消后立即删除；Session 删除或过期时批量清理；配合 TTL 兜底回收。
- **并发与背压：** Track 更新走独立队列，使用批处理/去抖窗口合并高频 delta；队列满时丢弃低价值事件并计数，避免阻塞广播。
- **失败策略：** taskStore 更新失败仅影响 progress，不阻断事件广播；记录 metrics + structured log，必要时触发一次性告警。

---

### Phase 2: 路由层重构（高收益，低风险）

#### 3.2.1 引入 RouterConfig 参数对象

将 17 个参数收拢为 config struct：

```go
type RouterDeps struct {
    TaskService       *TaskExecutionService
    SessionService    *SessionService
    SnapshotService   *SnapshotService
    Broadcaster       ports.SSEBroadcaster
    HealthChecker     ports.HealthChecker
    Auth              *AuthModule        // nil = disabled
    Config            *ConfigHandler
    Evaluation        *EvaluationService
    Observability     *observability.Observability
    Memory            memory.Service
    AttachmentCfg     attachments.StoreConfig
    SandboxBaseURL    string
}

type RouterConfig struct {
    Environment      string
    AllowedOrigins   []string
    MaxTaskBodyBytes int64
    StreamGuard      StreamGuardConfig
    RateLimit        RateLimitConfig
    NonStreamTimeout time.Duration
}

func NewRouter(deps RouterDeps, cfg RouterConfig) http.Handler
```

#### 3.2.2 路由注册表模式

用声明式路由注册替代手工 if/switch：

```go
type Route struct {
    Pattern string
    Method  string
    Handler http.HandlerFunc
    Auth    bool
}

func (r *Router) Register(routes []Route) {
    for _, route := range routes {
        handler := r.wrap(route)
        r.mux.Handle(route.Pattern, handler)
    }
}
```

项目使用 Go 1.24，`net/http.ServeMux` 已原生支持 `{param}` 路径参数和方法路由，无需引入外部路由库：

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /api/tasks/{id}", handler.GetTask)
mux.HandleFunc("DELETE /api/tasks/{id}", handler.CancelTask)
mux.HandleFunc("GET /api/sessions/{id}", handler.GetSession)
mux.HandleFunc("POST /api/sessions/{id}/replay", handler.ReplaySession)
```

- 支持 `{param}` 路径参数（`r.PathValue("id")`）
- 支持 `"METHOD /path"` 方法路由
- 零新增依赖，符合项目"避免不必要外部依赖"的风格
- 100% stdlib，无兼容性风险

#### 3.2.3 Auth disabled 路由简化

当前 auth disabled 时手动注册 12 个 503 handler (L147-162)。改为：

```go
if auth == nil {
    authMiddleware = func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            http.Error(w, "Authentication not configured", http.StatusServiceUnavailable)
        })
    }
}
```

一个 middleware 替代 12 行重复注册。

#### 3.2.4 兼容性策略与回归测试矩阵（补充）

**兼容性原则：**
- 所有现有路由与方法语义保持不变（含 404/405 行为、尾斜杠、query 忽略、path param 边界）。
- Auth disabled 行为统一为 503，响应体与现有 handler 对齐。
- SSE 与非 SSE 响应头/超时/压缩行为不回归。

**回归测试矩阵（最小集合）：**
- `/api/sessions/{id}`：GET/PUT/DELETE 正常 + 非法方法 405
- `/api/sessions/{id}/persona`：GET/PUT + 尾斜杠差异
- `/api/sessions/{id}/turns/{turnId}`：GET/DELETE + path param 校验
- `/api/sessions/{id}/replay|share|fork`：POST
- `/api/tasks/{taskId}`：GET/DELETE + rate limit 命中
- `/api/stream`：SSE 正常 + 非 SSE 请求 400/405
- `/api/evaluations/*`：GET/POST/DELETE 覆盖（http 层第二大 handler 文件）
- `/api/agents/{id}/*`：GET/PUT/DELETE + 子路由覆盖
- Auth disabled：抽样 3-5 个需鉴权的路由断言 503

建议补充一份现有路由清单（自动生成 or 手动）并做 golden 测试。

---

### Phase 3: 启动流程结构化（中收益，低风险）

#### 3.3.1 分阶段启动 + 显式降级

```go
type ServerBootstrapper struct {
    logger logging.Logger
    stages []BootstrapStage
    degraded map[string]string  // component -> reason
}

type BootstrapStage struct {
    Name     string
    Required bool  // true = fail fast; false = warn and continue
    Init     func(ctx context.Context) error
}
```

每个组件（event history、analytics、gateways、scheduler）是一个 stage。Required stage 失败则整个启动失败。Optional stage 失败记录到 `degraded` map，暴露在 /health 端点。
建议 /health 增加可选字段 `degraded`（map[string]string），并将整体 status 标记为 `degraded` 以便运维侧识别。

#### 3.3.2 子系统生命周期管理

将 gateway / scheduler 的 context 管理收拢为统一的 `SubsystemManager`：

```go
type Subsystem interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
}

type SubsystemManager struct {
    subsystems []Subsystem
    cancel     context.CancelFunc
}
```

替代 `RunServer()` 中 6 组独立的 `ctx, cancel` + `defer` 模式。

#### 3.3.3 `applyServerFileConfig()` 合并改造（补充）

目标：减少 253 行逐字段判空逻辑，降低新增配置项遗漏风险。

建议做法：
- 拆分为 `applyServerLoggingConfig`、`applyServerAuthConfig` 等子函数，按子结构聚合责任
- 引入轻量 helper（仅适用于标量和 slice），例如 `applyIfNonZero(dst *T, src T)` / `applyIfNotEmpty(dst *[]T, src []T)`
- 统一在 `config.LoadServerConfig()` 内完成 merge，保证逻辑单点

不建议引入反射式 deep-merge（可读性和可维护性差），除非已有依赖可复用。

---

### Phase 4: 数据层改进（中收益，中风险）

#### 3.4.1 InMemoryTaskStore 加入 TTL 淘汰

```go
type InMemoryTaskStore struct {
    mu       sync.RWMutex
    tasks    map[string]*taskEntry
    maxSize  int           // 最大任务数
    retention time.Duration // 已完成任务保留时间
}

type taskEntry struct {
    task      *ports.Task
    expiresAt time.Time  // 0 = 永不过期（running tasks）
}
```

后台 goroutine 定期清理过期任务。

#### 3.4.2 broadcasterMetrics 改用 atomic

```go
type broadcasterMetrics struct {
    totalEventsSent   atomic.Int64
    droppedEvents     atomic.Int64
    totalConnections  atomic.Int64
    activeConnections atomic.Int64
}

func (m *broadcasterMetrics) incrementEventsSent() {
    m.totalEventsSent.Add(1)
}
```

消除热路径上的 mutex 竞争。

---

### Phase 5: 质量提升（低收益，低风险）

#### 3.5.1 事件类型常量统一

建议在 `internal/agent/types/events.go` 定义所有事件类型常量，并由 server 层直接引用，避免 server->agent->server 反向依赖或 import cycle：

```go
const (
    EventWorkflowNodeOutputDelta   = "workflow.node.output.delta"
    EventWorkflowToolProgress      = "workflow.tool.progress"
    EventWorkflowResultFinal       = "workflow.result.final"
    EventWorkflowResultCancelled   = "workflow.result.cancelled"
    // ...
)
```

所有引用点改为使用常量。

#### 3.5.2 LLM Health Probe 增加实际检测

```go
func (p *LLMFactoryProbe) Check(ctx context.Context) ports.ComponentHealth {
    if p.container.LLMFactory == nil {
        return ports.ComponentHealth{
            Name:   "llm_factory",
            Status: ports.HealthStatusNotReady,
            Message: "LLM factory not initialized",
        }
    }
    // 可选：cached ping，避免每次 health check 调用 API
    return ports.ComponentHealth{...}
}
```

#### 3.5.3 WriteTimeout 分路由处理

对非 SSE 路由设置合理的 WriteTimeout（如 30s），SSE 路由通过 middleware 清理 write deadline（避免 Hijacker 破坏 HTTP/2）：
- Server 默认 `WriteTimeout=30s`
- SSE handler 中使用 `http.NewResponseController(w).SetWriteDeadline(time.Time{})` 清空 deadline
- 若无法使用 ResponseController，退化为拆分两套 server/listener（SSE 单独 server）

#### 3.5.4 事件丢弃可观测与背压策略（补充）

针对 `ensureCriticalEventDelivery` 的丢弃策略做显式化：
- 为每个连接维护 drop 计数与 lastDroppedAt，暴露 metrics
- 当发生 drop 时，发送一次 `workflow.stream.dropped` 事件（含 droppedCount、lastDroppedEventType）
- 对高频 delta 做合并/节流（合并窗口 50-100ms）
- 队列满时优先丢弃低价值事件（delta），保留 critical（final/cancelled）

---

## 4. 优先级排序与依赖关系

```
Phase 1.2 (解耦 EventBroadcaster ↔ TaskStore)
  ↓ 无依赖，可独立执行
Phase 1.1 (拆分 ServerCoordinator)
  ↓ 需要先完成 1.2
Phase 2 (路由层重构)
  ↓ 依赖 Phase 1.1 的新 service 接口
Phase 3 (启动流程结构化)
  ↓ 可与 Phase 2 并行
Phase 4 (数据层改进)
  ↓ 可与 Phase 2/3 并行
Phase 5 (质量提升)
  ↓ 可独立执行
```

**建议执行顺序：**

1. **Phase 1.2** — 风险最低、收益最高的独立变更
2. **Phase 4.2** — atomic metrics，5 分钟改动
3. **Phase 5.1** — 事件常量抽取
4. **Phase 1.1** — 核心解耦
5. **Phase 2** — 路由重构
6. **Phase 3** — 启动结构化（含 config 合并改造）
7. 其余 Phase 4/5 项（含丢弃策略补齐）

---

## 5. 风险与约束

| 风险 | 缓解策略 |
|------|---------|
| ServerCoordinator 拆分影响面大 | 分步进行：先提取 SessionService，验证无回归后再提取 TaskExecutionService |
| 路由层迁移至 Go 1.24 stdlib 增强路由 | 使用 `net/http.ServeMux` 原生 method+param 路由，零新增依赖；通过回归测试矩阵验证行为一致 |
| 启动流程变更可能影响部署 | 保持 `RunServer()` 签名不变，内部重构为调用 Bootstrapper |
| InMemoryTaskStore TTL 可能丢失运行中任务 | 仅对终态任务（completed/failed/cancelled）应用 TTL |
| 路由重构导致边界行为回归 | 补齐兼容性矩阵 + golden 路由测试 |
| 事件丢弃策略改变影响前端渲染 | 增加 dropped 事件 + metrics，前端容忍缺口 |

---

## 6. 指标定义

完成后应可观测到：

- ServerCoordinator 测试覆盖率从需 mock 11 依赖降至每个 service 3-4 个
- EventBroadcaster 不再直接依赖 TaskStore（接口边界清晰）
- NewRouter 参数数量从 17 降至 2（deps + config）
- RunServer 拆分为 <100 行的组装函数 + 结构化 stages
- 内存中任务记录数有上限
- Hot path（事件广播）无 mutex
- /health 输出包含 degraded 组件与原因
