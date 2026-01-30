# Plan: 服务端架构抽象优化 — 用更少代码实现所有功能 (2026-01-31)

## Goal

系统性消除 elephant.ai Go 后端中的代码重复和抽象不足问题，通过更好的基类/接口/工具函数，用**更少的代码**实现**相同的功能**，提升可维护性和扩展效率。

## Pre-checks

- 已审阅 `docs/reference/BACKEND_ARCHITECTURE_ISSUES.md`（P1-P2 问题）
- 已审阅 `docs/REFACTOR_LEDGER.md`（现有重构进度）
- 已审阅 `docs/memory/long-term.md`（长期记忆约束）

## 核心原则

1. **不改变外部行为** — 纯内部重构，所有 API/协议/输出保持不变
2. **增量提交** — 每个 Tier 独立可验证，一个 Tier 内也拆成多个 commit
3. **测试先行** — 重构前确保测试通过，重构后跑全量 lint+test
4. **从底层往上** — 先改基础设施（LLM client），再改中间层（tools），最后改交付层（HTTP handlers）

---

## 现状诊断摘要

| 问题类别 | 重复次数 | 涉及代码量 | 影响范围 |
|---|---|---|---|
| LLM Client 初始化/日志/响应处理 | 4 clients × 5 patterns | ~400 行重复 | `internal/llm/` |
| HTTP Handler 方法校验+JSON 解析 | 39+ 处 | ~300 行重复 | `internal/server/http/` |
| Channel Gateway 公共接口/锁管理 | 2 gateways | ~60 行重复 | `internal/channels/` |
| Tool 注册样板 | 50+ 行手工注册 | ~150 行 | `internal/toolregistry/` |
| Tool 实现三方法样板 | 40+ 工具 | ~600 行 | `internal/tools/builtin/` |
| DI Config 字段拷贝 | 1 处 30+ 字段 | ~30 行 | `internal/di/` |
| **合计可消除** | | **~1540 行** | |

---

## Tier 1: LLM Client 基类抽取 (最高收益)

### 1.1 问题

4 个 LLM Client（OpenAI、Anthropic、Antigravity、Ollama）在以下方面存在大量重复：

```
openai_client.go:37-58    — 初始化 (timeout/logger/httpClient/baseURL)
anthropic_client.go:46-67 — 完全相同的初始化模式
antigravity_client.go:37-58 — 完全相同
ollama_client.go:28-51    — 略有不同但核心一致

每个 Complete() 方法开头:
  extractRequestID → buildLogPrefix (openai:60-69, anthropic:70-78, antigravity:61-70)

每个 Complete() 方法中间:
  HTTP 请求构建 (headers, auth, retry) — 各 ~15 行

每个 Complete() 方法结尾:
  Response Summary 日志 — 各 ~10 行 (openai:272-281, antigravity: 类似位置)
```

### 1.2 方案

新建 `internal/llm/base_client.go`，提供 `baseClient` 结构体：

```go
// base_client.go

type baseClient struct {
    model      string
    apiKey     string
    baseURL    string
    httpClient *http.Client
    logger     logging.Logger
    headers    map[string]string
    maxRetries int
    usageCallback func(usage ports.TokenUsage, model string, provider string)
}

type baseClientConfig struct {
    defaultBaseURL string
    defaultTimeout time.Duration
    logCategory    string
    logComponent   string
}

func newBaseClient(model string, config Config, opts baseClientConfig) baseClient {
    baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
    if baseURL == "" {
        baseURL = opts.defaultBaseURL
    }
    timeout := opts.defaultTimeout
    if timeout == 0 {
        timeout = 120 * time.Second
    }
    if config.Timeout > 0 {
        timeout = time.Duration(config.Timeout) * time.Second
    }
    logger := utils.NewCategorizedLogger(opts.logCategory, opts.logComponent)
    return baseClient{
        model:      model,
        apiKey:     config.APIKey,
        baseURL:    baseURL,
        httpClient: httpclient.New(timeout, logger),
        logger:     logger,
        headers:    config.Headers,
        maxRetries: config.MaxRetries,
    }
}

// buildLogPrefix 统一日志前缀构建 (消除 12 处重复)
func (c *baseClient) buildLogPrefix(ctx context.Context, metadata map[string]string) (string, string) {
    requestID := extractRequestID(metadata)
    if requestID == "" {
        requestID = id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
    }
    logID := id.LogIDFromContext(ctx)
    prefix := fmt.Sprintf("[req:%s] ", requestID)
    if logID != "" {
        prefix = fmt.Sprintf("[log_id=%s] %s", logID, prefix)
    }
    return requestID, prefix
}

// doPost 统一 HTTP POST + headers (消除 4 处重复)
func (c *baseClient) doPost(ctx context.Context, endpoint string, body []byte, prefix string) (*http.Response, error) {
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    httpReq.Header.Set("Content-Type", "application/json")
    if c.apiKey != "" {
        httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
    }
    if c.maxRetries > 0 {
        httpReq.Header.Set("X-Retry-Limit", strconv.Itoa(c.maxRetries))
    }
    for k, v := range c.headers {
        httpReq.Header.Set(k, v)
    }
    c.logRequest(prefix, httpReq)
    return c.httpClient.Do(httpReq)
}

// logResponseSummary 统一响应摘要日志 (消除 4 处重复)
func (c *baseClient) logResponseSummary(prefix string, result *ports.CompletionResponse) {
    c.logger.Debug("%s=== LLM Response Summary ===", prefix)
    c.logger.Debug("%sStop Reason: %s", prefix, result.StopReason)
    c.logger.Debug("%sContent Length: %d chars", prefix, len(result.Content))
    c.logger.Debug("%sTool Calls: %d", prefix, len(result.ToolCalls))
    c.logger.Debug("%sUsage: %d prompt + %d completion = %d total tokens",
        prefix, result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens)
    c.logger.Debug("%s==================", prefix)
}

// fireUsageCallback 统一回调触发
func (c *baseClient) fireUsageCallback(usage ports.TokenUsage, provider string) {
    if c.usageCallback != nil {
        c.usageCallback(usage, c.model, provider)
    }
}
```

### 1.3 改造后的 Client 示例

```go
// openai_client.go — 改造后
type openaiClient struct {
    baseClient               // 嵌入，不再重复 6 个字段
}

func NewOpenAIClient(model string, config Config) (portsllm.LLMClient, error) {
    return &openaiClient{
        baseClient: newBaseClient(model, config, baseClientConfig{
            defaultBaseURL: "https://openrouter.ai/api/v1",
            logCategory:    utils.LogCategoryLLM,
            logComponent:   "openai",
        }),
    }, nil
}

func (c *openaiClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
    requestID, prefix := c.buildLogPrefix(ctx, req.Metadata) // 1 行代替 10 行
    // ... 构建 payload (provider-specific，保持不变)
    resp, err := c.doPost(ctx, c.baseURL+"/chat/completions", body, prefix) // 1 行代替 15 行
    // ... 解析响应 (provider-specific，保持不变)
    c.logResponseSummary(prefix, result) // 1 行代替 6 行
    c.fireUsageCallback(result.Usage, c.detectProvider()) // 1 行代替 10 行
    return result, nil
}
```

### 1.4 预期收益

- **消除 ~300 行**重复代码
- 新增 provider 只需实现 payload 构建 + response 解析，~50 行即可
- 日志格式统一，行为一致

### 1.5 关键文件

- 新建: `internal/llm/base_client.go`
- 修改: `openai_client.go`, `anthropic_client.go`, `antigravity_client.go`, `ollama_client.go`
- 修改: 对应的 streaming 文件 (`openai_stream.go`, `anthropic_stream.go` 等)

---

## Tier 2: HTTP Handler 中间件与通用解析 (中高收益)

### 2.1 问题

```
api_handler_tasks.go:63-64   — Method 校验
api_handler_sessions.go:89   — Method 校验
api_handler_misc.go:33       — Method 校验
... 共 39+ 处

api_handler_tasks.go:68-99   — JSON body 解析 + 错误分类
api_handler_misc.go:33-42    — 类似 JSON 解析
... 共 4+ 处，每处 ~30 行
```

### 2.2 方案

新建 `internal/server/http/handler_helpers.go`：

```go
// handler_helpers.go

// requireMethod 包装 handler，自动校验 HTTP 方法
func (h *APIHandler) requireMethod(method string, handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != method {
            h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed",
                fmt.Errorf("method %s not allowed", r.Method))
            return
        }
        handler(w, r)
    }
}

// decodeJSONBody 统一 JSON 请求体解析 (消除 4 处 ~30 行重复)
func (h *APIHandler) decodeJSONBody(w http.ResponseWriter, r *http.Request, maxSize int64, dst any) bool {
    body := http.MaxBytesReader(w, r.Body, maxSize)
    defer func() { _ = body.Close() }()

    decoder := json.NewDecoder(body)
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(dst); err != nil {
        h.handleDecodeError(w, err)
        return false
    }
    return true
}

func (h *APIHandler) handleDecodeError(w http.ResponseWriter, err error) {
    var syntaxErr *json.SyntaxError
    var typeErr *json.UnmarshalTypeError
    var maxBytesErr *http.MaxBytesError
    switch {
    case errors.Is(err, io.EOF):
        h.writeJSONError(w, http.StatusBadRequest, "Request body is empty", err)
    case errors.As(err, &syntaxErr):
        h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON at position %d", syntaxErr.Offset), err)
    case errors.As(err, &typeErr):
        h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid value for field '%s'", typeErr.Field), err)
    case errors.As(err, &maxBytesErr):
        h.writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large", err)
    default:
        h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
    }
}
```

### 2.3 改造后的 Handler 示例

```go
// 改造前: 40 行
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { ... }       // 4 行
    body := http.MaxBytesReader(...)              // 30 行解析
    ...
}

// 改造后: 15 行
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
    var req CreateTaskRequest
    if !h.decodeJSONBody(w, r, h.maxCreateTaskBodySize, &req) {
        return
    }
    // 直接开始业务逻辑
}
```

### 2.4 路由注册处使用 `requireMethod` 包装

```go
// 路由注册 (router.go 或 routes.go)
mux.HandleFunc("/api/tasks", h.requireMethod(http.MethodPost, h.HandleCreateTask))
mux.HandleFunc("/api/sessions/", h.requireMethod(http.MethodGet, h.HandleGetSession))
```

### 2.5 预期收益

- **消除 ~250 行**样板代码
- 错误响应格式完全统一
- 新增 handler 只需 ~5 行样板

---

## Tier 3: Channel Gateway 基础抽象 (中等收益)

### 3.1 问题

```
lark/gateway.go:41-44    — AgentExecutor interface 定义
wechat/gateway.go:22-25  — 完全相同的 interface 定义

两者共享: sessionLocks sync.Map, cfg 校验, logger 初始化, session ID 哈希
```

### 3.2 方案

新建 `internal/channels/base/gateway.go`：

```go
// base/gateway.go

package base

// AgentExecutor 所有 channel gateway 共用的 agent 执行接口
type AgentExecutor interface {
    EnsureSession(ctx context.Context, sessionID string) (*storage.Session, error)
    ExecuteTask(ctx context.Context, task string, sessionID string,
        listener agent.EventListener) (*agent.TaskResult, error)
}

// SessionLocker 管理 per-session 串行执行
type SessionLocker struct {
    locks sync.Map
}

func (sl *SessionLocker) Lock(sessionID string) func() {
    val, _ := sl.locks.LoadOrStore(sessionID, &sync.Mutex{})
    mu := val.(*sync.Mutex)
    mu.Lock()
    return mu.Unlock
}

// HashSessionID 统一 session ID 哈希
func HashSessionID(prefix string, parts ...string) string {
    h := sha1.New()
    for _, p := range parts {
        h.Write([]byte(p))
    }
    return fmt.Sprintf("%s-%x", prefix, h.Sum(nil)[:8])
}
```

### 3.3 改造

```go
// lark/gateway.go
type Gateway struct {
    cfg           Config
    agent         base.AgentExecutor    // 改为引用共用接口
    logger        logging.Logger
    client        *lark.Client
    sessions      base.SessionLocker    // 替换 sessionLocks sync.Map
    ...
}

// wechat/gateway.go
type Gateway struct {
    cfg      Config
    agent    base.AgentExecutor
    logger   logging.Logger
    sessions base.SessionLocker
    ...
}
```

### 3.4 预期收益

- **消除 ~60 行**重复
- 新 channel (如 Telegram、Slack) 不需要再复制 interface + 锁管理

---

## Tier 4: Tool 实现样板消除 (中等收益)

### 4.1 问题

每个 tool 必须实现 3 个方法：`Execute()`, `Definition()`, `Metadata()`。
其中 `Definition()` 和 `Metadata()` 返回纯静态数据，但每个 tool 都要写 ~15 行。
40+ 个 tool 意味着 ~600 行纯样板。

```go
// 当前: 每个 tool 15 行静态样板
func (t *fileRead) Definition() ports.ToolDefinition {
    return ports.ToolDefinition{
        Name: "file_read", Description: "Read file contents",
        Parameters: ports.ParameterSchema{ ... },
    }
}
func (t *fileRead) Metadata() ports.ToolMetadata {
    return ports.ToolMetadata{Name: "file_read", Version: "1.0.0", Category: "file_operations"}
}
```

### 4.2 方案

新建 `internal/tools/builtin/shared/base_tool.go`：

```go
// base_tool.go

// BaseTool 封装静态定义信息，tool 只需实现 ExecuteFunc
type BaseTool struct {
    Def  ports.ToolDefinition
    Meta ports.ToolMetadata
    Exec func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error)
}

func (b *BaseTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
    return b.Exec(ctx, call)
}
func (b *BaseTool) Definition() ports.ToolDefinition { return b.Def }
func (b *BaseTool) Metadata() ports.ToolMetadata     { return b.Meta }
```

### 4.3 改造后的 Tool 示例

```go
// 改造前: 58 行
type fileRead struct{}
func NewFileRead(cfg shared.FileToolConfig) tools.ToolExecutor { ... }
func (t *fileRead) Execute(...) { ... }       // 业务逻辑
func (t *fileRead) Definition() { ... }       // 15 行静态
func (t *fileRead) Metadata() { ... }         // 3 行静态

// 改造后: 25 行
func NewFileRead(cfg shared.FileToolConfig) tools.ToolExecutor {
    return &shared.BaseTool{
        Def: ports.ToolDefinition{
            Name: "file_read", Description: "Read file contents",
            Parameters: ports.ParameterSchema{
                Type: "object",
                Properties: map[string]ports.Property{
                    "path": {Type: "string", Description: "File path"},
                },
                Required: []string{"path"},
            },
        },
        Meta: ports.ToolMetadata{Name: "file_read", Version: "1.0.0", Category: "file_operations"},
        Exec: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
            // 纯业务逻辑，和现在一样
        },
    }
}
```

### 4.4 注意

- 对于需要在 struct 上持有状态的 tool（如 `htmlEdit` 持有 `llmClient`），仍然用传统 struct 方式
- `BaseTool` 是可选的便利层，不强制所有 tool 使用
- 逐步迁移，先从无状态 tool 开始

### 4.5 预期收益

- **消除 ~400 行**样板（40 个无状态 tool × 10 行 Definition/Metadata）
- 新增 tool 只需关注 Execute 逻辑

---

## Tier 5: Tool 注册声明化 (低-中收益)

### 5.1 问题

`toolregistry/registry.go:353-508` 有 ~150 行手工注册代码：
```go
r.static["file_read"] = fileops.NewFileRead(fileConfig)
r.static["file_write"] = fileops.NewFileWrite(fileConfig)
// ... 重复 50+ 次
```

### 5.2 方案

改为表驱动注册：

```go
type toolEntry struct {
    name    string
    factory func() tools.ToolExecutor
    enabled bool // 条件注册
}

func (r *Registry) registerBuiltins(config Config) error {
    fileConfig := shared.FileToolConfig{}
    shellConfig := shared.ShellToolConfig{}

    entries := []toolEntry{
        // File operations
        {"file_read", func() tools.ToolExecutor { return fileops.NewFileRead(fileConfig) }, true},
        {"file_write", func() tools.ToolExecutor { return fileops.NewFileWrite(fileConfig) }, true},
        {"file_edit", func() tools.ToolExecutor { return fileops.NewFileEdit(fileConfig) }, true},
        {"list_files", func() tools.ToolExecutor { return fileops.NewListFiles(fileConfig) }, true},
        // Shell
        {"bash", func() tools.ToolExecutor { return execution.NewBash(shellConfig) }, execution.LocalExecEnabled},
        {"grep", func() tools.ToolExecutor { return search.NewGrep(shellConfig) }, true},
        // ... 所有 tool 统一格式
    }

    for _, e := range entries {
        if e.enabled {
            r.static[e.name] = e.factory()
        }
    }

    // 需要特殊配置的 tool (media/sandbox) 保留原有逻辑
    r.registerMediaTools(config)
    r.registerSandboxTools(config)

    return nil
}
```

### 5.3 预期收益

- 注册逻辑从 ~150 行压缩到 ~60 行
- 一眼可见所有注册的 tool 列表
- 条件注册统一表达

---

## Tier 6: DI Config 字段映射消除 (低收益但简单)

### 6.1 问题

`di/container_builder.go:151-172` 手工拷贝 20+ 字段：
```go
appconfig.Config{
    LLMProvider: b.config.LLMProvider,
    LLMModel:    b.config.LLMModel,
    // ... 20+ 行
}
```

### 6.2 方案

让 `appconfig.Config` 嵌入 `di.Config` 的核心子集，或提取一个共享的 `CoreConfig` struct：

```go
// internal/agent/app/config/core.go
type CoreLLMConfig struct {
    LLMProvider       string
    LLMModel          string
    LLMSmallProvider  string
    LLMSmallModel     string
    LLMVisionModel    string
    APIKey            string
    BaseURL           string
    MaxTokens         int
    MaxIterations     int
    ToolMaxConcurrent int
    Temperature       float64
    TemperatureProvided bool
    TopP              float64
    StopSequences     []string
    AgentPreset       string
    ToolPreset        string
    ToolMode          string
    EnvironmentSummary string
    SessionStaleAfter time.Duration
    Proactive         bool
}

// di.Config 嵌入 CoreLLMConfig
type Config struct {
    CoreLLMConfig
    SessionDir string
    CostDir    string
    // ... DI-specific fields
}

// appconfig.Config 也嵌入
type Config struct {
    CoreLLMConfig
    // ... app-specific fields
}
```

### 6.3 改造

```go
// 改造前: 20+ 行逐字段复制
coordinator := agentcoordinator.NewAgentCoordinator(..., appconfig.Config{
    LLMProvider: b.config.LLMProvider, LLMModel: b.config.LLMModel, ...
})

// 改造后: 1 行
coordinator := agentcoordinator.NewAgentCoordinator(..., appconfig.Config{
    CoreLLMConfig: b.config.CoreLLMConfig,
})
```

### 6.4 预期收益

- **消除 ~20 行**手工映射
- Config 字段新增时不再需要同步两处

---

## 执行顺序与依赖

```
Tier 1 (LLM base client)      ← 无依赖，最先做
  │
  ├── Tier 6 (Config 嵌入)     ← 无依赖，可并行
  │
  ├── Tier 3 (Channel base)    ← 无依赖，可并行
  │
Tier 4 (Tool BaseTool)         ← 无依赖，可并行
  │
  └── Tier 5 (Tool 声明化注册)  ← 依赖 Tier 4
  │
Tier 2 (HTTP middleware)       ← 无依赖，可并行
```

建议执行批次:
1. **Batch 1**: Tier 1 + Tier 6 + Tier 3 (并行)
2. **Batch 2**: Tier 4 → Tier 5
3. **Batch 3**: Tier 2

---

## 总预期收益

| Tier | 消除代码 | 新增代码 | 净减少 |
|---|---|---|---|
| T1 LLM base | ~300 行 | ~80 行 | **-220 行** |
| T2 HTTP middleware | ~250 行 | ~40 行 | **-210 行** |
| T3 Channel base | ~60 行 | ~30 行 | **-30 行** |
| T4 Tool BaseTool | ~400 行 | ~20 行 | **-380 行** |
| T5 Tool 声明化 | ~90 行 | ~40 行 | **-50 行** |
| T6 Config 嵌入 | ~20 行 | ~10 行 | **-10 行** |
| **合计** | **~1120 行** | **~220 行** | **-900 行** |

---

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| LLM 行为回归 | 全量 LLM mock test + 手工验证 1 个真实 provider |
| Tool 行为回归 | 现有 tool test 全部通过 |
| HTTP API 回归 | 现有 handler test + web E2E |
| import cycle (channel base → agent ports) | base 包只引用 ports，不引用具体实现 |
| BaseTool 与有状态 tool 不兼容 | BaseTool 是可选层，有状态 tool 保持原方式 |

---

## 验证清单 (每个 Tier 完成后)

- [ ] `go build ./...` 通过
- [ ] `go test ./...` 全部通过
- [ ] `go vet ./...` 无告警
- [ ] `golangci-lint run` 无新告警
- [ ] 手工跑一次 CLI 任务确认端到端正常
- [ ] git diff 确认无意外文件变更

---

## Progress

- 2026-01-31: Plan created. Comprehensive codebase analysis completed.
- 2026-01-31: Tier 1 (LLM base client) completed. ~330 lines reduced across 5 LLM client files.
- 2026-01-31: Tier 2 (HTTP handler middleware) completed. ~50 lines reduced across 6 handler files.
- 2026-01-31: Tier 3 (Channel Gateway base) completed. Extracted BaseConfig, BaseGateway, and 4 shared helpers. ~80 lines reduced.
- 2026-01-31: Tier 4 (Tool BaseTool) completed. 50+ tool structs refactored to embed BaseTool, eliminating Definition/Metadata boilerplate.
- 2026-01-31: Tier 5 (Tool registry) completed. Simplified NewRegistry to pass Config directly instead of field-by-field copy (~25 lines removed).
- 2026-01-31: Tier 6 (DI Config) completed. Embedded channels.BaseConfig in bootstrap gateway configs, eliminating 16 duplicated field declarations.
- 2026-01-31: Full validation passed: `go build ./...`, `go vet ./...`, `go test ./...` — all green, zero failures across 80+ packages.

### Post-completion optimization (continued)

- 2026-01-31: Extracted `doSandboxCall[T]` and `doSandboxRequest[T]` generic helpers in `sandbox_tools.go` — deduplicated 8 identical DoJSON+Success+nil-Data validation blocks across sandbox file/shell/code tools. Net **-36 lines** in sandbox package.
- 2026-01-31: Replaced local `boolArg` in `web/html_edit.go` with `shared.BoolArgWithDefault`. Net **-23 lines**.
- 2026-01-31: Fixed QF1003 lint (tagged switch) in `openai_responses_parse.go`.
- 2026-01-31: Total additional reduction: **-58 lines** (commit `e2e6f7d0`).

## Status: COMPLETE
