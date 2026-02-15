# Plan: Web 模式拆分为独立 Binary

## Context

当前 `cmd/alex-server` 是一个双模式二进制：无参数启动 web HTTP API，`lark` 参数启动 Lark WS 网关。随着 Lark 成为主要交互通道，web 模式的 50+ 个端点（auth、sessions、tasks、evaluations 等）在 Lark 部署中完全不需要，但 dev/debug 页面和 SSE 事件流对观察 Lark 任务执行仍然很有价值。

**目标**：将 `alex-server` 聚焦为 Lark-primary 二进制，内嵌轻量 debug HTTP server；web 模式独立为 `alex-web` 二进制。

## 方案概述

```
Before:  alex-server [lark] → RunServer() / RunLark()
After:   alex-server         → RunLark() + embedded debug HTTP (:9090)
         alex-web            → RunServer() (完整 web API + frontend)
```

两个 binary 共享核心包（`internal/app/di/`、`internal/agent/`、`internal/llm/` 等），通过 `BootstrapFoundation` 统一初始化。

---

## 实施步骤

### Batch 1: 提取 Debug Router

**Step 1.1** — 新建 `internal/delivery/server/http/router_debug.go`

从 `NewRouter` 中提取 debug 子集，创建 `NewDebugRouter(deps DebugRouterDeps) http.Handler`：

```
注册端点:
  GET  /health
  GET  /api/sse                                    ← SSE 事件流（观察 Lark 任务）
  GET  /api/dev/logs, /api/dev/logs/structured, /api/dev/logs/index
  GET  /api/dev/memory
  GET  /api/dev/sessions/{session_id}/context-window
  GET  /api/dev/context-config, PUT, GET preview
  GET  /api/internal/config/runtime, PUT, GET stream, GET models
  GET  /api/internal/subscription/catalog
  GET  /api/internal/onboarding/state, PUT
  GET  /api/internal/sessions/{session_id}/context
  POST /api/hooks/claude-code                      ← Claude Code 钩子桥接（如有 LarkGateway）

中间件栈（最小化）:
  Compression → Logging → Observability
  无 CORS / 无 Rate Limit / 无 Auth（仅 localhost/内网）
```

`DebugRouterDeps` 结构体只需：
- `Broadcaster *app.EventBroadcaster`
- `RunTracker app.RunTracker` (可为 nil)
- `HealthChecker *app.HealthCheckerImpl`
- `ConfigHandler *ConfigHandler`
- `OnboardingStateHandler *OnboardingStateHandler`
- `Obs *observability.Observability`
- `MemoryEngine MemoryEngine`
- `HooksBridge http.Handler` (可为 nil)

复用现有 `SSEHandler`、`APIHandler`（`devMode=true, internalMode=true`）和 `ContextConfigHandler`。

**Step 1.2** — `internal/delivery/server/bootstrap/config.go` 添加 `DebugPort` 字段

```go
DebugPort string // 默认 "9090"，环境变量 ALEX_DEBUG_PORT
```

**Step 1.3** — 新建 `internal/delivery/server/bootstrap/lark_debug.go`

提供 `BuildDebugHTTPServer(broadcaster, container, config) (*http.Server, error)` 辅助函数：
1. 创建 HealthChecker，注册 LLM/Degraded 探针
2. 创建 ConfigHandler、OnboardingStateHandler
3. 构建 HooksBridge（如 `container.LarkGateway != nil`）
4. 调用 `NewDebugRouter(deps)`
5. 返回 `*http.Server` 监听 `:DebugPort`

---

### Batch 2: 修改 RunLark 嵌入 Debug HTTP

**修改文件**: `internal/delivery/server/bootstrap/lark.go`

在 Phase 2（optional stages）之后、Phase 3（subsystems）之前，添加：

```
Phase 2b: EventBroadcaster (in-memory only)
  - NewEventBroadcaster(WithMaxHistory(500), WithMaxSessions(50), WithSessionTTL(1h))
  - 无 Postgres EventHistoryStore（debug 模式不需要持久化事件）
  - subscribeDiagnostics(broadcaster) 订阅环境诊断
```

Phase 3 中 `startLarkGateway` 改为传入 `broadcaster`（当前传 `nil`）。

Phase 4 之后添加：
```
Phase 5: Debug HTTP
  - BuildDebugHTTPServer(broadcaster, container, config)
  - async.Go 启动 debug HTTP server
  - waitForSignal 阻塞
  - signal 到来后 Shutdown debug HTTP (5s timeout)
```

---

### Batch 3: 二进制拆分

**Step 3.1** — 新建 `cmd/alex-web/main.go`

```go
func main() {
    runtimeconfig.LoadDotEnv()
    obsConfig := os.Getenv("ALEX_OBSERVABILITY_CONFIG")
    serverBootstrap.RunServer(obsConfig)
}
```

直接调用现有 `RunServer()`，不做任何修改。

**Step 3.2** — 简化 `cmd/alex-server/main.go`

移除 web 分支，始终运行 Lark：

```go
func main() {
    runtimeconfig.LoadDotEnv()
    obsConfig := os.Getenv("ALEX_OBSERVABILITY_CONFIG")
    // 向后兼容：忽略遗留 "lark" 子命令
    serverBootstrap.RunLark(obsConfig)
}
```

保留对 `os.Args[1] == "lark"` 的兼容（log deprecation warning + continue），避免破坏现有 scripts 过渡期。

---

### Batch 4: 构建和脚本更新

| 文件 | 变更 |
|------|------|
| `Makefile` | 添加 `web-build`/`web-run` targets，更新 `server-build` 描述为 "Lark server" |
| `dev.sh: build_server()` | 同时构建 `alex-server` 和 `alex-web` |
| `dev.sh: start_server()` | 改为运行 `alex-web`（web dashboard） |
| `dev.sh` | 添加 `start_lark()` 函数运行 `alex-server`（无需 `lark` 参数） |
| `scripts/lark/main.sh` | 移除 `lark` 参数：`"${BIN}"` 而非 `"${BIN}" lark` |
| `scripts/lark/test.sh` | 同上 |
| `scripts/lark/cleanup_orphan_agents.sh` | 更新进程匹配模式：不再匹配 `alex-server lark` |
| `scripts/validate-deployment.sh` | 添加 `alex-web` 编译验证 |

---

### Batch 5: 配置支持

**Step 5.1** — `internal/shared/config/` 中添加 `debug_port` 的 file config 和 env 解析

```yaml
server:
  debug_port: "9090"    # 或 ALEX_DEBUG_PORT 环境变量
```

---

### Batch 6: 测试

| 测试 | 内容 |
|------|------|
| `router_debug_test.go` | 验证 debug router 只注册了 debug 端点，无 auth/task/session/eval 端点 |
| `lark_debug_test.go` | 集成测试 `BuildDebugHTTPServer`：health 200、SSE 连接、config API |
| 编译验证 | `go build ./cmd/alex-server && go build ./cmd/alex-web` |
| 端到端 | 启动 `alex-server`，验证 Lark WS 连接 + debug HTTP `:9090/health` |

---

## 关键文件清单

| 文件 | 操作 |
|------|------|
| `cmd/alex-server/main.go` | 修改：移除 web 分支 |
| `cmd/alex-web/main.go` | 新建：web 模式入口 |
| `internal/delivery/server/http/router_debug.go` | 新建：debug-only 路由 |
| `internal/delivery/server/http/router_debug_test.go` | 新建：测试 |
| `internal/delivery/server/bootstrap/lark_debug.go` | 新建：debug HTTP 构建辅助 |
| `internal/delivery/server/bootstrap/lark_debug_test.go` | 新建：集成测试 |
| `internal/delivery/server/bootstrap/lark.go` | 修改：添加 Phase 2b + Phase 5 |
| `internal/delivery/server/bootstrap/config.go` | 修改：添加 DebugPort |
| `internal/delivery/server/http/router_deps.go` | 不改动（复用现有类型） |
| `internal/delivery/server/bootstrap/server.go` | 不改动（原样用于 alex-web） |
| `Makefile` | 修改 |
| `dev.sh` | 修改 |
| `scripts/lark/main.sh` | 修改 |
| `scripts/lark/test.sh` | 修改 |
| `scripts/lark/cleanup_orphan_agents.sh` | 修改 |
| `scripts/validate-deployment.sh` | 修改 |

## 注意事项

1. **CCHooksAutoConfig**: `startLarkGateway` 中 `gatewayCfg.CCHooksAutoConfig.ServerURL` 当前用 `cfg.Port`（web 端口 8080）。拆分后应改用 `cfg.DebugPort`，因为 hooks bridge 端点挂在 debug HTTP 上。
2. **Next.js 前端不从 debug server 提供**：开发时用 `next dev --port 3000` 指向 debug server API（通过 `.env.local` 配置 `NEXT_PUBLIC_API_URL=http://localhost:9090`）。生产环境只有 `alex-web` 提供前端静态资源。
3. **过渡期兼容**：`alex-server lark` 命令仍然可用（打印 deprecation warning），确保现有部署脚本不会立即失败。
4. **coordinator 缺失的端点**：`/api/dev/sessions/{session_id}/context-window` 需要 `ServerCoordinator`。第一期返回 503（nil-safe），后续可为 Lark 模式构建轻量 coordinator wrapper。

## 验证方式

1. `go build ./cmd/alex-server && go build ./cmd/alex-web` — 两个 binary 都能编译
2. 启动 `alex-server`：Lark WS 连接正常 + `curl localhost:9090/health` 返回 200
3. 启动 `alex-web`：完整 web API + frontend 正常工作
4. 通过 Lark 发消息，在 `localhost:9090/api/sse?session_id=xxx&debug=1` 实时观察事件流
5. `./dev.sh up` 正常启动（web dashboard + infra）
6. `scripts/lark/main.sh` 正常启动 Lark 模式
7. 全量 lint + test 通过
