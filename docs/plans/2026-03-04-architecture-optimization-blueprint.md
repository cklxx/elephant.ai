# Architecture Optimization Blueprint

Date: 2026-03-04
Owner: ckl

## Status Quo

当前架构是**枚举驱动拼接**：每个扩展点（provider、channel、tool policy、状态映射）都用 switch/if 枚举实现，分散在 4-6 个文件里。分层名义上存在，实际上 123 条 arch exception（全部同日过期）说明约束从未真正生效。

**核心矛盾**：系统已经长成多 provider、多 channel、多 agent 的复杂度，但抽象还停在单 provider、单 channel 的时代。

---

## Target Architecture Principles

1. **Capability-driven, not enumeration-driven.** 新 provider/channel/tool 通过注册描述符接入，不通过 switch 分支。
2. **Strict layer boundaries, enforced by tooling.** `make check-arch` 零 exception 是常态，不是目标。
3. **Domain is portable.** Domain 层不 import `os`/`net/http`/`os/exec`/`path/filepath`，全部通过 port 注入。
4. **Faithful contract propagation.** 语义不在层间塌缩（`waiting_input` 不折叠，causality 不截断）。
5. **Interface Segregation.** 调用方只依赖所需的窄接口，fat interface 拆分到 ≤ 4 方法。
6. **Single source of truth.** 同一关注点（provider 能力、model listing、channel 格式化）只有一个实现。

---

## System Panorama — Target State

### Runtime Surfaces

```
                                   ┌──────────────────────────────────────┐
                                   │          External World              │
                                   └──┬──────┬──────┬──────┬──────┬──────┘
                                      │      │      │      │      │
                                   Lark   Telegram WeChat  Web   CLI
                                   (WS)   (poll)  (hook)  (HTTP) (TTY)
                                      │      │      │      │      │
                               ┌──────▼──────▼──────▼──────▼──────▼──────┐
                               │     Channel Plugin Registry              │
                               │  ┌────────┐ ┌────────┐ ┌────────┐       │
                               │  │LarkPlug│ │TGPlug  │ │WXPlug  │  ...  │
                               │  │Gateway │ │Gateway │ │Gateway │       │
                               │  │Notify  │ │Notify  │ │Notify  │       │
                               │  │Hints   │ │Hints   │ │Hints   │       │
                               │  └────────┘ └────────┘ └────────┘       │
                               └──────────────────┬──────────────────────┘
                                                  │ AgentExecutor + EventListener
┌─────────────┐                ┌──────────────────▼──────────────────────┐
│  alex (CLI)  │──────────────▶│           Application Layer             │
│  alex-web    │──HTTP/SSE────▶│                                         │
│  alex-server │──────────────▶│  ┌─────────────┐  ┌──────────────────┐  │
│  eval-server │──────────────▶│  │ Coordinator  │  │  Context Mgmt    │  │
└─────────────┘                │  │ (orchestrate)│  │  ┌────────────┐  │  │
                               │  └──────┬───────┘  │  │WindowBuild │  │  │
                               │         │          │  │Compressor  │  │  │
                               │  ┌──────▼───────┐  │  │TurnRecord  │  │  │
                               │  │ Tool Registry │  │  └────────────┘  │  │
                               │  │ (wrap chain)  │  ├──────────────────┤  │
                               │  └──────┬───────┘  │  Scheduler       │  │
                               │         │          │  Subscription    │  │
                               │         │          │  Notification    │  │
                               │         │          │  DI Container    │  │
                               │         │          └──────────────────┘  │
                               └─────────┼───────────────────────────────┘
                                         │ Ports (interfaces only)
                               ┌─────────▼───────────────────────────────┐
                               │           Domain Layer                   │
                               │                                          │
                               │  ┌──────────┐  ┌──────────┐  ┌───────┐  │
                               │  │ ReAct    │  │ Workflow  │  │ Task  │  │
                               │  │ Engine   │  │ Model    │  │ Store │  │
                               │  └────┬─────┘  └────┬─────┘  └───┬───┘  │
                               │       │             │             │      │
                               │  ┌────▼─────────────▼─────────────▼───┐  │
                               │  │            Domain Ports            │  │
                               │  │  LLMClient  ToolExec  SessionStore│  │
                               │  │  ProcessRun  ArtifactStore        │  │
                               │  │  HTTPFetcher EventListener        │  │
                               │  │  HistoryMgr  CostTracker          │  │
                               │  └────────────────┬───────────────────┘  │
                               └───────────────────┼──────────────────────┘
                                                   │ Implementations
                               ┌───────────────────▼──────────────────────┐
                               │         Infrastructure Layer              │
                               │                                           │
                               │  ┌─────────────────────────────────────┐  │
                               │  │     Provider Capability Registry     │  │
                               │  │  ┌────────┐┌────────┐┌──────────┐  │  │
                               │  │  │Anthropic││OpenAI  ││Kimi      │  │  │
                               │  │  │DescPtr ││DescPtr ││DescPtr   │  │  │
                               │  │  └────────┘└────────┘└──────────┘  │  │
                               │  └─────────────────────────────────────┘  │
                               │                                           │
                               │  ┌────────┐ ┌────────┐ ┌──────────────┐  │
                               │  │Memory  │ │Session │ │ Process      │  │
                               │  │(SQLite)│ │(File)  │ │ (exec/tmux)  │  │
                               │  └────────┘ └────────┘ └──────────────┘  │
                               │  ┌────────┐ ┌────────┐ ┌──────────────┐  │
                               │  │Skills  │ │Coding  │ │ Observability│  │
                               │  │(cache) │ │(sandbox│ │ (metrics/    │  │
                               │  │        │ │ /exec) │ │  traces/logs)│  │
                               │  └────────┘ └────────┘ └──────────────┘  │
                               └───────────────────────────────────────────┘
                                                   │
                               ┌───────────────────▼──────────────────────┐
                               │           Shared Layer                    │
                               │  Config │ Logging │ JSON │ Token │ Utils │
                               └──────────────────────────────────────────┘
```

### Data Flow: Request → Response

```
User Input (Lark msg / HTTP POST / CLI stdin)
    │
    ▼
[1] Channel Plugin / HTTP Router / CLI Parser
    │  normalize → unified task request
    ▼
[2] Coordinator.ExecuteTask(ctx, task, sessionID, listener)
    │  load session, resolve model, build tool preset
    ▼
[3] Preparation Service
    │  session replay → context window → system prompt → model selection
    │  prompt hints injected via ChannelHintProvider port (app port, delivery adapter)
    ▼
[4] ReAct Engine Loop
    │  ┌─────────────────────────────────────────┐
    │  │ Think → Plan Tools → Execute → Observe  │ ←── checkpoint each iteration
    │  │    ▲                              │      │
    │  │    └──────────────────────────────┘      │
    │  └─────────────────────────────────────────┘
    │  tool calls via ToolRegistry (wrap chain: validate → approve → retry → SLA)
    │  LLM calls via Provider Registry (capability-driven, no switch)
    ▼
[5] Domain Events emitted (typed, with correlationID + causationID)
    │
    ├──▶ [6a] Workflow Event Translator → WorkflowEventEnvelope
    │         │  faithful field propagation (no semantic collapse)
    │         ▼
    │    [6b] Event Broadcaster (SSE) / File Event History Store
    │         │  causality fields persisted end-to-end
    │         ▼
    │    [6c] Web Frontend (useSSE) / Lark Progress Listener / CLI Output
    │
    └──▶ [7] Session Persistence + Cost Logging + Turn Recording
```

### Event Causality Chain (Target: End-to-End)

```
Domain Event (events.go)
    correlationID: root run_id
    causationID:   parent call_id
         │
         ▼
Workflow Envelope (envelope.go:98-99)
    SetCorrelationID ✓  ──── preserved
    SetCausationID   ✓  ──── preserved
         │
         ▼
Event Translator (workflow_event_translator.go)
    ErrorStr     ✓  ──── preserved
    PhaseLabel   ✓  ──── preserved
    Recoverable  ✓  ──── preserved
         │
         ▼
File Event History Store (file_event_history_store.go)
    correlationID  ✓  ──── MUST persist (currently dropped at :247)
    causationID    ✓  ──── MUST persist (currently dropped at :247)
         │
         ▼
Event Replay / Debug / Audit
    full causality chain available for cross-layer tracing
```

### Task Status Contract (Target: Faithful Propagation)

```
Domain (store.go)          Server Ports (task.go)       Frontend (task.ts)
─────────────────          ──────────────────────       ─────────────────
pending            ──────▶ TaskStatusPending      ────▶ "pending"
running            ──────▶ TaskStatusRunning      ────▶ "running"
waiting_input      ──────▶ TaskStatusWaitingInput ────▶ "waiting_input"  ← NEW
completed          ──────▶ TaskStatusCompleted    ────▶ "completed"
failed             ──────▶ TaskStatusFailed       ────▶ "failed"
cancelled          ──────▶ TaskStatusCancelled    ────▶ "cancelled"

No folding. No semantic loss. Frontend renders distinct UX per status.
```

### Subsystem Boundary Contracts

| Boundary | Contract Type | Current | Target |
|----------|--------------|---------|--------|
| Channel → App | `AgentExecutor` interface + `EventListener` | Working, but channel-specific Notifier | Unified: channel plugin provides `Notifier` via `Send(ctx, target, content)` + `ChannelHintProvider` port (app port, delivery adapter) |
| App → Domain | Port interfaces (`ports/agent/`, `ports/llm/`, `ports/tools/`) | Working, but ContextManager is fat (8 methods) | ISP: `WindowBuilder` + `TokenCompressor` + `TurnRecorder` |
| Domain → Infra | Port interfaces (no direct import allowed) | **Violated**: `os`, `os/exec`, `net/http` in domain | **Clean**: `ProcessRunner`, `ArtifactStore`, `HTTPFetcher` ports |
| App → Delivery | Should not exist | **Violated**: `app/scheduler` → `delivery/schedulerapi` | **Clean**: scheduler ports in `app/scheduler/ports/` |
| Infra → External | Provider/service adapters | Hardcoded switch per provider | Provider Capability Registry, single registration point |
| Delivery → Config | YAML config loading | Hardcoded `ChannelsConfig{Lark, Telegram}` fields | Dynamic `map[string]yaml.Node`, parsed by channel plugin |

### Extension Points (After Optimization)

| To Add... | Minimum Change Set | You Don't Touch... |
|-----------|-------------------|-------------------|
| New LLM provider (e.g., Gemini) | 1 provider file: `infra/llm/providers/gemini.go`（实现 `ProviderDescriptor` + `init()` 注册）+ 1 import line: `cmd/*/main.go` 加 side-effect import + YAML config entry | factory, DI builder, config normalizer, subscription catalog, runtime_models |
| New channel (e.g., Slack) | 1 plugin file: `delivery/channels/slack/plugin.go`（实现 `ChannelPlugin` + `init()` 注册）+ 1 import line: `cmd/*/main.go` 加 side-effect import + YAML config entry | bootstrap, prompt manager, scheduler notifier, config structs |
| New tool | 1 tool file: `infra/tools/builtin/newtool/`（实现 `ToolExecutor`）+ 注册调用 | tool registry core, policy engine |
| New task status | 1 domain enum + 1 server port const + 1 frontend type + 审计消费方 | No folding logic, no semantic mapping |
| New memory backend | 1 实现文件: implement `Engine` interface（无 `RootDir()`）+ DI 注入替换 | context manager, API handler |

> **注**：上述"最少改动集合"假设显式 import 注册机制（非反射自动发现）。每个新插件需要在对应 cmd 包的 import 块中加一行 side-effect import（`_ "alex/..."`) 以保证链接。这比"1 文件"稍多，但链接路径可审计、编译时可检查。

### Bootstrap Lifecycle (Target)

```
[1] Load Config (YAML → RuntimeConfig)
[2] Init Observability (logger, metrics, tracer)
[3] Build DI Container
    ├── Provider Registry (explicit import of provider packages; see §Discovery below)
    ├── Channel Registry (explicit import of channel packages; see §Discovery below)
    ├── Tool Registry (builtins + dynamic + MCP)
    ├── Memory Engine
    ├── Session/History/Cost Stores
    └── Coordinator(s)
[4] Start Required Stages (fail = abort)
    ├── Container.Start()
    ├── HTTP Server (if web mode)
    └── Channel validation: every configured channel MUST have a registered plugin (fail-fast)
[5] Start Optional Stages (fail = degrade, unless it's the only entry point)
    ├── Channel Plugins (iterate config, start enabled ones)
    ├── Scheduler + Timer
    ├── Kernel Daemon (if enabled)
    └── Analytics
[6] Ready Gate
    ├── At least one entry point (HTTP server OR ≥1 channel) is serving
    ├── Healthcheck endpoint (/healthz) returns 200
    └── If no entry point available → abort with clear error
[7] Shutdown
    ├── Drain (graceful, per-subsystem timeout)
    └── Stop channel plugins, close stores
```

**Discovery Mechanism**: Provider 和 channel 插件**不**依赖 `init()` 自动发现（`init()` 链接顺序不可控，且 side-effect import 路径难以审计）。采用**显式注册**：

```go
// cmd/alex-server/main.go (或 bootstrap/registry.go)
import (
    _ "alex/internal/infra/llm/providers/openai"    // 注册 openai provider
    _ "alex/internal/infra/llm/providers/anthropic"  // 注册 anthropic provider
    _ "alex/internal/infra/llm/providers/kimi"       // 注册 kimi provider
    _ "alex/internal/delivery/channels/lark"          // 注册 lark channel
    _ "alex/internal/delivery/channels/telegram"      // 注册 telegram channel
)
```

每个 `init()` 内部调用 `llm.Register()` / `channels.RegisterChannel()`。链接入口（cmd 包）决定哪些插件被编译进二进制。这比纯反射/文件扫描更可控。

### Web Frontend Architecture (Target)

```
Next.js App
├── Pages (app/)
│   ├── conversation/  ← main chat UI
│   ├── sessions/      ← session management
│   ├── evaluation/    ← eval/benchmark
│   └── share/         ← artifact sharing
│
├── Event Pipeline (hooks/useSSE/)
│   ├── SSE Connection → Event Buffer (50 cap)
│   │                  → Event History (1000 cap)
│   │                  → Delta Merge (10K char cap)
│   └── Status: includes "waiting_input" with distinct UX
│
├── State Management (hooks/)
│   ├── useSessionStore   ← session CRUD
│   ├── useTaskExecution  ← task lifecycle (6 statuses)
│   └── useAgentEventStream ← event subscription
│
└── Types (lib/types/)
    ├── api/task.ts     ← 6 statuses (including waiting_input)
    └── events/*        ← typed event payloads
```

---

## Phase 0: Foundation — 消除层违规 + 契约修复

**目标**：让分层约束真正生效，修复数据语义。Phase 0 仅处理本 phase 涉及的切片（A01-A05, A14），不要求清零全部 exception。gate: `make check-arch` exception < 80。

**对应切片**：A01, A02, A03, A04, A05, A14

### 0.1 Domain 净化

| 污染点 | 当前 | 目标 | 新 Port |
|--------|------|------|---------|
| `react/background.go:8-10,:961` | 直接 `os`/`os/exec`/`path/filepath`（tmux 进程管理） | 通过 port 注入 | `ProcessRunner` — `Start(ctx, cmd, args, env) (Process, error)` + `Kill(Process) error` |
| `react/context_artifact_compaction.go:9-10,:251` | 直接 `os`/`path/filepath`（文件读写） | 通过 port 注入 | `ArtifactStore` — `Read(ctx, path) ([]byte, error)` + `Write(ctx, path, data) error` + `Remove(ctx, path) error` |
| `materialregistry/attachment_migrator.go:10,:170` | 直接 `net/http` | 通过 port 注入 | `HTTPFetcher` — `Fetch(ctx, url) (io.ReadCloser, error)` |

Port 定义位置：`internal/domain/agent/ports/infra/`（新建，与 `ports/agent/`、`ports/llm/`、`ports/tools/` 平级）。
Infra adapter 位置：`internal/infra/adapters/`。

### 0.2 反向依赖消除

**A01**: `schedulerapi.Job` DTO 和 `schedulerapi.Service` 接口下沉到 `internal/app/scheduler/ports/`。`internal/delivery/schedulerapi/` 变为纯 adapter，反向 import 清零。`exceptions.yaml` 对应条目删除。

**A02**: `Notifier` 接口从 `SendLark(ctx, chatID, content)`/`SendMoltbook(ctx, content)` 改为：

```go
// internal/app/scheduler/ports/notifier.go
type Notifier interface {
    Send(ctx context.Context, target NotificationTarget, content string) error
}

type NotificationTarget struct {
    Channel string // "lark", "telegram", "web", ...
    Address string // chatID / userID / webhook URL
}
```

`Trigger` 结构体中 `Channel`/`UserID`/`ChatID` 合并为 `NotificationTarget`，消除 channel-specific 字段语义。

### 0.3 契约修复

**A05 — 状态语义**：

> **注意**：引入 `waiting_input` 为新外显状态，属于行为变更。虽然 scope 写"行为不变"，但状态塌缩本身就是 bug，修复它不应被 scope 限制。需要兼容迁移策略。

1. `internal/delivery/server/ports/task.go` 加入 `TaskStatusWaitingInput`。
2. `server_adapter.go:230-231` 取消折叠，`waiting_input` 直通。
3. 前端 `web/lib/types/api/task.ts` 显式定义 `waiting_input` 状态。
4. 审计 `web/components/*` 所有消费 `task.status` 的组件，确保 UI 正确渲染 `waiting_input`（如显示"等待输入"提示 + 输入框）。

**兼容迁移策略**：
- **Phase 1 — 后端先行**：后端开始返回 `waiting_input`。老前端因 `status: string` 泛型不会 crash，但可能显示为 unknown/默认态。
- **Phase 2 — 前端跟进**：前端明确处理 `waiting_input`，渲染对应 UX。
- **回滚条件**：如果前端未就绪，后端通过 feature flag `TASK_STATUS_V2=false` 恢复旧折叠行为。Feature flag 在前端部署确认后删除。
- **API 版本窗口**：至少保留一个版本周期（2 周）的 flag 可回滚窗口。

**A14 — 因果链**：
1. `file_event_history_store.go:247` 修复：持久化 `correlationID` 和 `causationID`。
2. `eventFileRecord` 新增这两个字段。
3. 恢复路径 `agentEventFromRecord` 正确回填。

### 0.4 验证门

```bash
# 层级违规收敛
make check-arch  # exception < 80（Phase 0 清理本 phase 涉及的条目）

# Domain 无 infra import
grep -rn '"os"\|"os/exec"\|"path/filepath"\|"net/http"' internal/domain/ | grep -v _test.go  # 空

# 状态语义直通（辅助告警）
grep -n "StatusWaitingInput.*StatusRunning" internal/delivery/  # 空

# 因果链持久化（辅助告警）
grep -n "not persisted" internal/delivery/server/app/  # 空

# 行为验证（A14 因果链 round-trip）
go test ./internal/delivery/server/app/ -run TestEventCausalityRoundTrip  # pass
# 测试逻辑：写入带 correlationID+causationID 的事件 → 重启 store → replay → 断言字段一致

# 行为验证（A05 状态直通）
go test ./internal/delivery/taskadapters/ -run TestWaitingInputPreserved  # pass
# 测试逻辑：domain StatusWaitingInput → domainStatusToServer → 断言 != TaskStatusRunning

# 回归
alex dev lint && alex dev test
```

---

## Phase 1: Capability Registry — Provider + Channel 插件化

**目标**：新 provider / channel 接入只需"注册一次"，不需"全仓补 switch"。

**对应切片**：A06, A07, A08, A09a, A09b, A10

### 1.1 Provider Capability Registry

**当前问题**：provider 信息分散在 5 处 switch + 2 处重复 `fetchProviderModels`。

**目标架构**：

```go
// internal/infra/llm/registry.go
type ProviderDescriptor struct {
    Name           string                // "openai", "anthropic", "kimi", ...
    Family         string                // "openai-compat", "anthropic", "llamacpp"
    Capabilities   ProviderCapabilities  // streaming, thinking, vision, function_calling, ...
    ClientFactory  func(model string, cfg ClientConfig) (LLMClient, error)
    CredentialResolver func(creds CLICredentials) (apiKey, baseURL string, ok bool)
    ModelLister    func(ctx context.Context, apiKey, baseURL string) ([]string, error)
    RateLimiter    *rate.Limiter         // nil = no limit
    RequestAdapter func(req *CompletionRequest)  // provider-specific request mutations
    Headers        func(apiKey string) http.Header  // provider-specific auth headers
}

type ProviderCapabilities struct {
    Streaming       bool
    Thinking        bool   // reasoning/chain-of-thought
    Vision          bool
    FunctionCalling bool
    MaxContextWindow int
}

// Registration
var registry = map[string]*ProviderDescriptor{}

func Register(desc *ProviderDescriptor) { registry[desc.Name] = desc }
func Get(name string) (*ProviderDescriptor, bool) { ... }
func List() []*ProviderDescriptor { ... }
```

**消除的 switch**：
- `factory.go:193` — 改为 `registry.Get(provider).ClientFactory(model, cfg)`
- `builder_llm.go:30` — 改为 `registry.Get(provider).CredentialResolver(creds)`
- `llm_profile.go:72` — 改为 `registry.Get(provider).Family`
- `provider_resolver.go:164` — 统一通过 registry 遍历匹配
- `catalog.go:414` + `runtime_models.go:83` — 合并为 `registry.Get(provider).ModelLister(...)`

**消除的特判**：
- `openai_client.go:461-478,516,544`（Kimi 检测） — Kimi 注册自己的 `RequestAdapter`
- `base_client.go:101-105`（Kimi UA） — Kimi 注册自己的 `Headers`
- `thinking.go:97-181`（reasoning 判定） — 改为 `Capabilities.Thinking` 字段查询

**Provider 注册示例**：

```go
// internal/infra/llm/providers/kimi.go
func init() {
    llm.Register(&llm.ProviderDescriptor{
        Name:   "kimi",
        Family: "openai-compat",
        Capabilities: llm.ProviderCapabilities{
            Streaming: true, FunctionCalling: true,
        },
        ClientFactory: func(model string, cfg llm.ClientConfig) (llm.LLMClient, error) {
            return llm.NewOpenAIClient(model, cfg)  // reuse OpenAI-compat client
        },
        RequestAdapter: func(req *llm.CompletionRequest) {
            // Kimi-specific: empty content → placeholder
            for i := range req.Messages {
                if isEmptyContent(req.Messages[i].Content) {
                    req.Messages[i].Content = " "
                }
            }
        },
        Headers: func(apiKey string) http.Header {
            return http.Header{"User-Agent": {"KimiCLI/1.3"}, "Authorization": {"Bearer " + apiKey}}
        },
        // ...
    })
}
```

### 1.2 Channel Plugin Interface

**当前问题**：channel 注册硬编码在 `ChannelsConfig{Lark, Telegram}`；bootstrap 有 channel-specific 文件；prompt 对 Lark 硬编码。

**目标架构**：

```go
// internal/delivery/channels/plugin.go
type ChannelPlugin interface {
    Name() string
    Capabilities() ChannelCapabilities
    NewGateway(deps GatewayDeps) (Gateway, error)
    PromptHints() []PromptHint  // channel-specific formatting rules for system prompt
}

type ChannelCapabilities struct {
    SupportsMarkdown  bool
    SupportsRichText  bool
    SupportsButtons   bool
    SupportsFileUpload bool
    MaxMessageLength  int
}

type PromptHint struct {
    Section string  // section name
    Content string  // prompt content
}

type Gateway interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

// Registration
var channelRegistry = map[string]ChannelPlugin{}

func RegisterChannel(p ChannelPlugin) { channelRegistry[p.Name()] = p }
```

**Bootstrap 改造**：

```go
// internal/delivery/server/bootstrap/channels.go (replaces lark_gateway.go, telegram_gateway.go)
func (f *Foundation) ChannelStages(subsystems *Subsystems) ([]BootstrapStage, error) {
    var stages []BootstrapStage
    for name, cfg := range f.config.Channels {
        plugin, ok := channels.Get(name)
        if !ok {
            // 配置了但没插件 → fail-fast，不静默跳过
            return nil, fmt.Errorf("channel %q configured but no plugin registered", name)
        }
        stages = append(stages, f.channelStage(plugin, cfg, subsystems))
    }
    return stages, nil
}
```

**启动就绪条件**：
- Required stages 全部成功。
- 至少一个入口可用（HTTP server 或至少一个 channel gateway 启动成功）。
- 配置了的 channel 必须有对应插件注册（fail-fast），**未配置**的 channel 静默跳过。
- Optional stages（analytics、kernel daemon）失败记录 degraded，不阻塞 Ready。
- Ready 信号由显式 healthcheck 端点暴露，不仅仅是"启动完成"。

**Config 改造**：

```yaml
# 现在
channels:
  lark:
    app_id: "..."
    app_secret: "..."
  telegram:
    bot_token: "..."

# 目标：同样的 YAML，但解析不再依赖 Go 结构体字段名
channels:
  lark:
    # ... plugin-specific config, parsed by plugin
  telegram:
    # ...
  wechat:  # 新 channel 零代码改动
    # ...
```

**Prompt 注入改造**：

> **层级约束**：app 层（`manager_prompt*.go`）不能直接 import delivery 层的 channel registry。Prompt hint 能力必须通过 app/domain port 注入，delivery channel 插件作为 adapter 注册。

```go
// internal/app/context/ports.go (新增 — app 层 port)
type ChannelHintProvider interface {
    PromptHints(channel string) []PromptHint
}

type PromptHint struct {
    Section string
    Content string
}
```

```go
// internal/delivery/channels/hint_adapter.go (delivery 层 adapter，实现 app port)
type channelHintAdapter struct{}

func (a *channelHintAdapter) PromptHints(channel string) []PromptHint {
    plugin, ok := channelRegistry[channel]
    if !ok { return nil }
    return plugin.PromptHints()
}
```

```go
// internal/app/context/manager_prompt_context.go (app 层消费 port，不 import delivery)
func buildChannelFormattingSection(provider ChannelHintProvider, channel string) string {
    hints := provider.PromptHints(channel)
    if len(hints) == 0 { return "" }
    // format hints into prompt section
}
```

DI 在 bootstrap 层将 `channelHintAdapter` 注入到 context manager。App 层只看到 `ChannelHintProvider` 接口，不知道 delivery 细节。

### 1.3 验证门

```bash
# === 辅助告警（grep，快速信号，不作为唯一判定） ===
grep -n "switch.*provider\|case.*openai.*anthropic" internal/infra/llm/factory.go  # 空
grep -n '"lark"\|"telegram"' internal/delivery/server/bootstrap/server.go  # 空
grep -n 'channel.*lark\|"lark"' internal/app/context/manager_prompt*.go  # 空
grep -rn "func.*fetchProviderModels" internal/ | wc -l  # 1

# === 行为验证（契约测试，必须通过） ===
# Provider registry 契约：注册 → 查询 → 能力匹配
go test ./internal/infra/llm/ -run TestProviderRegistryContract  # pass

# Channel plugin 契约：注册 → prompt hint 注入 → 不 import delivery
go test ./internal/app/context/ -run TestChannelHintViaPort  # pass

# App 层不 import delivery channel
go test ./internal/app/... -run TestNoDeliveryImport  # pass (或 make check-arch 覆盖)

# === 硬门禁（必须全部通过才算 Phase 1 完成） ===
make check-arch        # exception < 30
alex dev lint
alex dev test
alex dev test-llm-integration  # 全 provider 端到端验证（OpenAI/Anthropic/Kimi/DeepSeek）
```

> **规则**：`alex dev test-llm-integration` 是 A06/A07/A08 三个切片的**硬门禁**，不是"建议跑"。切片 PR 的 CI 必须包含此步骤。

---

## Phase 2: Interface Refinement — ISP + 抽象后端无关化

**目标**：fat interface 拆分；抽象不泄露实现细节。

**对应切片**：A11, A12, A13

### 2.1 ContextManager 拆分 (A13)

当前 8 方法混合 3 个职责：

```
ContextManager (BEFORE)
├── EstimateTokens    ─┐
├── Compress          ─┤ TokenCompressor (5 methods)
├── AutoCompact       ─┤
├── ShouldCompress    ─┤
├── BuildSummaryOnly  ─┘
├── Preload           ─┐ WindowBuilder (2 methods)
├── BuildWindow       ─┘
└── RecordTurn        ── TurnRecorder (1 method)
```

拆分为 3 个窄接口：

```go
// internal/domain/agent/ports/agent/context.go

type WindowBuilder interface {
    Preload(ctx context.Context) error
    BuildWindow(ctx context.Context, session *storage.Session, cfg ContextWindowConfig) (ContextWindow, error)
}

type TokenCompressor interface {
    EstimateTokens(messages []core.Message) int
    Compress(messages []core.Message, targetTokens int) ([]core.Message, error)
    AutoCompact(messages []core.Message, limit int) ([]core.Message, bool)
    ShouldCompress(messages []core.Message, limit int) bool
    BuildSummaryOnly(messages []core.Message) (string, int)
}

type TurnRecorder interface {
    RecordTurn(ctx context.Context, record ContextTurnRecord) error
}
```

调用方只依赖所需接口：
- ReAct engine → `WindowBuilder` + `TokenCompressor`
- Session persistence → `TurnRecorder`
- Compression logic → `TokenCompressor`

### 2.2 Memory 后端无关化 (A12)

`Engine` 接口移除 `RootDir() string`：

```go
// internal/infra/memory/engine.go (BEFORE)
type Engine interface {
    // ... 7 methods ...
    RootDir() string  // ← 泄露文件路径语义
}

// AFTER
type Engine interface {
    EnsureSchema(ctx context.Context) error
    AppendDaily(ctx context.Context, userID string, entry DailyEntry) (string, error)
    Search(ctx context.Context, userID, query string, maxResults int, minScore float64) ([]SearchHit, error)
    Related(ctx context.Context, userID, path string, fromLine, toLine, maxResults int) ([]RelatedHit, error)
    GetLines(ctx context.Context, userID, path string, fromLine, lineCount int) (string, error)
    LoadDaily(ctx context.Context, userID string, day time.Time) (string, error)
    LoadLongTerm(ctx context.Context, userID string) (string, error)
    IsConfigured() bool  // 替代 RootDir() != "" 检查
}
```

上层（`manager_memory.go:84-90`、`api_handler_memory.go:47-50`）从 `RootDir() == ""` 改为 `IsConfigured() == false`。

### 2.3 Tool Policy 默认规则外部化 (A11)

将 `DefaultPolicyRules()` 中的 Go 常量移到 `configs/tools/default-policy.yaml`：

```yaml
# configs/tools/default-policy.yaml
rules:
  - name: l4-irreversible
    match: { safety_levels: [4] }
    retry: { enabled: false }
    timeout: 60s
  - name: lark-write-ops
    match: { categories: [lark], dangerous: true }
    timeout: 30s
    retry: { enabled: false }
  # ...
```

`DefaultPolicyRules()` 改为加载 YAML，Go 代码不再硬编码规则。

### 2.4 验证门

```bash
# === 辅助告警 ===
grep -c "interface" internal/domain/agent/ports/agent/context.go  # ≥ 3
grep -n "PolicyRule{" internal/infra/tools/policy.go  # 无硬编码 PolicyRule 字面量

# === A12 跨层验证（不仅查 engine.go，还查所有调用点） ===
grep -rn "RootDir" internal/infra/memory/engine.go  # 仅实现体内部（private field/method）
grep -rn "\.RootDir()" internal/app/ internal/delivery/  # 空（上层已无调用）
# 行为测试：Engine mock 不实现 RootDir，上层仍能正常工作
go test ./internal/app/context/ -run TestMemoryWithoutRootDir  # pass
go test ./internal/delivery/server/http/ -run TestMemoryHandlerWithoutRootDir  # pass

# === A13 行为验证 ===
# 每个拆分后的窄接口有独立 mock 可注入
go test ./internal/domain/agent/react/ -run TestReactWithNarrowInterfaces  # pass

# === 硬门禁 ===
make check-arch  # 0 exceptions
alex dev lint && alex dev test
```

---

## Phase 3: Payload Governance — 事件体积控制

**目标**：后端 payload 有界；前端 cap 有后端对应物。

**对应切片**：A15

### 3.1 Snapshot 输出分级

> **层级约束**：Domain 层不做持久化副作用。大 output 落盘由 app/infra 通过 port 完成。Domain 只负责纯数据判定和显式 error 返回。

```go
// internal/domain/workflow/ports.go (新增 — domain port)
type LargeOutputPersister interface {
    Persist(ctx context.Context, data []byte) (ref string, err error)
}
```

```go
// internal/domain/workflow/node.go (domain 层 — 纯数据逻辑，无副作用)
type NodeSnapshot struct {
    // ... existing fields ...
    Output    any    `json:"output,omitempty"`    // 小 output（≤ 4KB）内联
    OutputRef string `json:"output_ref,omitempty"` // 大 output 引用
}

const MaxInlineOutputBytes = 4096

// ClassifyOutput 判定 output 是否超限，返回序列化后的字节。
// 调用方（app/infra）负责对超限 output 调用 LargeOutputPersister。
func ClassifyOutput(output any) (data []byte, inline bool, err error) {
    data, err = json.Marshal(output)
    if err != nil {
        return nil, false, fmt.Errorf("marshal output: %w", err)
    }
    return data, len(data) <= MaxInlineOutputBytes, nil
}
```

```go
// internal/app/agent/coordinator/snapshot_writer.go (app 层 — 调用 port 做持久化)
func (w *SnapshotWriter) SetNodeOutput(ctx context.Context, node *workflow.NodeSnapshot, output any) error {
    data, inline, err := workflow.ClassifyOutput(output)
    if err != nil {
        return err
    }
    if inline {
        node.Output = output
        node.OutputRef = ""
        return nil
    }
    ref, err := w.persister.Persist(ctx, data)
    if err != nil {
        return fmt.Errorf("persist large output: %w", err)
    }
    node.Output = nil
    node.OutputRef = ref
    return nil
}
```

### 3.2 验证门

```bash
# Domain 无持久化副作用
grep -rn "Persist\|os\.Create\|os\.Write" internal/domain/workflow/ | grep -v _test.go | grep -v ports  # 空

# Snapshot 有字节上限且 error 显式处理
go test ./internal/domain/workflow/ -run TestClassifyOutput  # pass
# 测试逻辑：≤4KB → inline=true；>4KB → inline=false；非法 input → error

# App 层 round-trip
go test ./internal/app/agent/coordinator/ -run TestLargeOutputPersistRoundTrip  # pass

# 回归
make check-arch && alex dev lint && alex dev test
```

---

## Dependency Graph

```
Phase 0 (Foundation)
├── A01 ──→ A02
├── A03 (independent)
├── A04 (independent)
├── A05 (independent)
└── A14 (independent)
    │
    ▼
Phase 1 (Capability Registry)
├── A06 ──→ A07 ──→ A08
└── A09a ──→ A09b ──→ A10
    │
    ▼
Phase 2 (Interface Refinement)
├── A13 (depends on A03)
├── A12 (independent)
└── A11 (independent)
    │
    ▼
Phase 3 (Payload Governance)
└── A15 (depends on A14)
```

Phase 内切片可并行（无写冲突时）。Phase 间严格顺序。

---

## Priority Adjustments vs Original Plan

| Slice | Original | Revised | Reason |
|-------|----------|---------|--------|
| A06 | P1 | **P0** | Provider registry 是 A07/A08/A09/A10 的前置条件；不升级则后续在错误基础上构建 |
| A07 | P1 | **P0** | Provider 特判清理必须与 A06 registry 同步完成，否则新老逻辑并存 |
| A13 | P2 | **P1** | 接口拆分是 domain 可测试性基础设施；8 方法 fat interface 阻碍单元测试隔离 |
| A14 | P2 | **P1** | 因果链截断是可观测性硬伤；排障断链不应排在 payload 瘦身之后 |
| A11 | P1 | **P2** | Policy 框架已是 metadata-driven，仅默认规则外部化，优先级低于结构性问题 |

---

## Arch Exception 治理策略

123 条 exception 不能等 3/31 过期日统一处理。治理规则：

1. **每个切片 PR 必须删除对应 exception 条目。** 不删 = 切片未完成。
2. **Phase 0 完成后**，exception 数量应降到 < 80。
3. **Phase 1 完成后**，exception 数量应降到 < 30。
4. **Phase 2 完成后**，exception 数量应降到 0。目标：`make check-arch` 零 exception 是常态。
5. 新代码**禁止**新增 exception。如果 `make check-arch` 报新违规，必须修代码而非加豁免。

---

## Success Metrics

### Quantitative Metrics

| Metric | Current | Phase 0 | Phase 1 | Phase 2 | Phase 3 | Collection Method | Cadence |
|--------|---------|---------|---------|---------|---------|-------------------|---------|
| Arch exceptions | 123 | < 80 | < 30 | 0 | 0 | `grep -c "expires_at" configs/arch/exceptions.yaml` | 每个切片 PR merge 后 |
| Domain infra imports (non-test) | 5+ | 0 | 0 | 0 | 0 | `grep -rn '"os"\|"os/exec"\|"path/filepath"\|"net/http"' internal/domain/ \| grep -v _test.go \| wc -l` | 每个切片 PR merge 后 |
| Provider switch/if sites | 6+ | 6+ | 0 | 0 | 0 | `grep -rn "switch.*provider\|case.*openai.*case.*anthropic" internal/infra/llm/ internal/app/ internal/shared/config/ internal/delivery/ \| wc -l` | Phase 1 完成时 |
| Channel hardcoded sites | 4+ | 3 | 0 | 0 | 0 | `grep -rn '"lark"\|"telegram"' internal/delivery/server/bootstrap/ internal/app/context/ \| grep -v _test.go \| wc -l` | Phase 1 完成时 |
| `fetchProviderModels` copies | 2 | 2 | 1 | 1 | 1 | `grep -rn "func.*fetchProviderModels" internal/ \| wc -l` | Phase 1 完成时 |
| ContextManager interface methods | 8 | 8 | 8 | max 4 per interface | max 4 | `go doc ./internal/domain/agent/ports/agent/ \| grep -c "func"` per interface | Phase 2 完成时 |
| Event causality round-trip | Fail | Pass | Pass | Pass | Pass | `go test ./internal/delivery/server/app/ -run TestEventCausalityRoundTrip` | 每次 CI |
| Status semantic collapse | Yes | No | No | No | No | `go test ./internal/delivery/taskadapters/ -run TestWaitingInputPreserved` | 每次 CI |
| Snapshot output bounded | No | No | No | No | Yes | `go test ./internal/domain/workflow/ -run TestClassifyOutput` | Phase 3 完成时 |
| LLM provider integration | N/A | N/A | All pass | All pass | All pass | `alex dev test-llm-integration` | A06/A07/A08 每个切片 PR |

### 指标说明

- **Provider switch sites = 0**（不是 1）：Phase 1 目标是消除所有 provider 枚举 switch。注册表入口 (`Register`/`Get`) 本身不算 switch site，因为它是通用查找逻辑，不包含 provider 名称枚举。
- **采集周期**：标注为"每次 CI"的指标必须集成到 CI pipeline 中作为硬门禁（test fail = PR blocked）。标注为"Phase N 完成时"的指标在 phase 收尾时手动验收。
- **阈值越界处理**：任何指标如果在某个 phase 之后反弹（如新代码又引入 domain infra import），CI 应立即 fail。

---

## References

- Slice-level execution plan: `docs/plans/2026-03-04-architecture-refactor-slices-priority-impact.md`
- Architecture reference: `docs/reference/ARCHITECTURE.md`
- Prior review: `docs/plans/architecture-review-2026-02-16.md`
- Layer enforcement: `configs/arch/exceptions.yaml`
