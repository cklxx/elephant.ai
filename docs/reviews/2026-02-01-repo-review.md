# elephant.ai 仓库代码广泛审查报告

Date: 2026-02-01
Reviewer: Codex (for cklxx)

## 范围
- 代码路径：`internal/`, `cmd/`, `web/`
- 重点关注：工具调用链路、事件流/广播、调度器、存储与网络 I/O
- 非目标：UI 视觉/交互体验、产品层需求一致性

## 方法
- 快速扫描：I/O 无上限读取、长生命周期 goroutine、无界内存结构、Background context、以及 TODO 记录。
- 深读模块：SSE 流、EventBroadcaster、AsyncEventHistoryStore、Scheduler、关键工具链（web、media）。
- 验证：运行 `./dev.sh lint` 与 `./dev.sh test`。

## 结论摘要
- 已为外部 HTTP 响应读取引入统一上限配置（含 web/媒体/模型列表/沙箱），P1 内存放大风险已消除。
- AsyncEventHistoryStore 已改为失败保留 + 退避重试，避免历史事件静默丢失。
- 调度器并发策略/超时与 EventBroadcaster session 处理已规范化；SSE/FileStore/Attachment 可取消性已增强。

## 发现清单（按严重度排序）

### P1 — 外部 HTTP 响应 `io.ReadAll` 无上限（潜在 OOM/DoS）
**位置**
- `internal/tools/builtin/web/web_fetch.go:261`
- `internal/tools/builtin/web/html_edit.go:396`
- `internal/tools/builtin/web/web_search.go:150`
- `internal/tools/builtin/media/music_play.go:145`
- `internal/server/http/runtime_models.go:121`
- `internal/subscription/catalog.go:289`
- `internal/sandbox/client.go:83`、`internal/sandbox/client.go:113`

**问题**
多个工具/服务端请求对外部 HTTP 响应使用 `io.ReadAll`，未设置上限。外部服务异常或被恶意利用时，可通过超大响应触发内存占用暴涨甚至 OOM，导致服务不可用。

**影响**
- 运行时内存可被对外请求放大（尤其中间件/工具链可被主动触发）。
- 生产环境可能出现短时异常峰值导致容器被杀。

**建议**
- 统一引入响应体大小上限（例如 1–4 MiB，按工具类型可配置）。
- 采用 `io.LimitReader` 或 `http.MaxBytesReader`，对错误场景返回明确“响应过大”提示。
- 将上限与超时纳入 `.yaml` 配置，并在工具/HTTP client 层统一实现。

---

### P1 — AsyncEventHistoryStore 刷写失败时静默丢数据
**位置**
- `internal/server/app/async_event_history_store.go:195-207`

**问题**
`flushBuffer` 无论 `Append/AppendBatch` 成功与否都会清空 `buffer`，失败时仅记录日志。意味着历史事件持久化可在失败时被直接丢弃，无自动重试或回放策略。

**影响**
- 事件历史回放与审计不可靠（对外 SSE replay 或诊断回放会缺失）。
- 失败窗口期无法追溯，用户感知为“历史丢失”。

**建议**
- 失败时保留 buffer 并指数退避重试；必要时落盘或死信队列。
- 在 `Stream/Delete/HasSessionEvents` 前的强制 flush 可加超时与告警，避免无限阻塞。

---

### P2 — Scheduler 触发执行缺少并发/超时策略，sessionID 固定
**位置**
- `internal/scheduler/executor.go:24-34`
- `internal/scheduler/scheduler.go:46-77`

**问题**
Scheduler 使用 `context.Background()` 启动任务，且 `sessionID = scheduler-<name>` 固定不变。cron 触发频率若高于任务执行时间，会发生并发执行，导致事件/上下文混写。

**影响**
- 结果可能交叉覆盖、事件广播混乱。
- 当任务挂起时无法取消，导致资源占用不可控。

**建议**
- 每次触发创建唯一 sessionID（例如 `scheduler-<name>-<run_id>`）。
- 使用 `cron.WithChain(cron.SkipIfStillRunning/DelayIfStillRunning)` 或显式互斥。
- 为每次 trigger 设置超时（基于配置或默认值）。

---

### P2 — EventBroadcaster sessionID 缺失时广播到所有会话
**位置**
- `internal/server/app/event_broadcaster.go:151-156`

**问题**
若事件缺少 sessionID，将广播到所有 session，并记录包含 session 列表的日志。虽然有“全局事件”设计，但缺失 sessionID 与全局事件不是同一语义。

**影响**
- 可能泄露跨会话事件，影响数据隔离。
- O(N) 广播与日志噪音随 session 数增长。

**建议**
- 仅允许明确标记的全局事件进入 global 通道。
- 对 sessionID 缺失事件直接丢弃并告警（或仅记录 metrics）。

---

### P3 — SSE 长连接内 `lastSeqByRun` 无界增长
**位置**
- `internal/server/http/sse_handler_stream.go:142-145, 255-266`

**问题**
`lastSeqByRun` map 随 run 增长而增长，长连接中可能累积大量 runID。

**影响**
- SSE 连接驻留时间越长，内存累计越多（虽不致命但可持续增长）。

**建议**
- 引入 LRU 或设置上限（例如只保留最近 N 个 run）。

---

### P3 — FileStore 列表读取使用 `context.Background()` + 逐条读取完整快照
**位置**
- `internal/session/state_store/file_store.go:158-169`

**问题**
列表接口在获取元信息时读取完整快照，且使用 `context.Background()`，无法取消。快照数量大时线性放大 I/O。

**影响**
- 列表接口延迟上升、资源占用增加。
- 无法对上层取消/超时进行响应。

**建议**
- 将上层 ctx 传入 `GetSnapshot`，或引入仅读取元数据的索引。

---

### P3 — Attachment Cloudflare 操作使用 `context.Background()`
**位置**
- `internal/attachments/store.go:233-249`

**问题**
上传/预签名请求使用 `context.Background()`，无法被上层取消。

**影响**
- 请求在上层已取消/关闭时仍持续占用资源。

**建议**
- 改为使用调用方 ctx 或在 store 内设定明确超时。

---

## 积极模式与已有防护
- SSE/事件流中已有 `LRU`、事件去重与 drop 通知机制，防止客户端缓冲无限增长。
- EventHistory 支持 TTL/MaxSessions/MaxHistory 配置，具备容量控制基础。

## 测试/验证
- `./dev.sh lint`：通过。
- `./dev.sh test`：通过。

## 建议优先级
1. 为所有外部 HTTP 读取引入统一的响应体大小上限与配置化策略（P1）。
2. 修复 AsyncEventHistoryStore 的失败语义，避免事件历史静默丢失（P1）。
3. Scheduler 执行引入并发策略与唯一 sessionID（P2）。
4. 收敛 EventBroadcaster 缺失 sessionID 的广播行为（P2）。
5. SSE/LRU 与 FileStore/Attachment ctx 改造（P3）。

## 修复状态（2026-02-01）
- P1 外部 HTTP 上限：已新增 `http_limits` 统一配置并覆盖 web_fetch/web_search/html_edit/music_play/runtime_models/subscription/sandbox 路径。
- P1 AsyncEventHistoryStore：失败保留 buffer，指数退避重试（min 250ms / max 5s）。
- P2 Scheduler：唯一 sessionID（`scheduler-<name>-<run_id>`）+ 并发策略（skip|delay）+ trigger 超时配置。
- P2 EventBroadcaster：缺失 session 直接 drop；`__global__` 明确全局广播。
- P3 SSE/FileStore/Attachment：lastSeqByRun LRU（默认 2048）+ 列表 ctx 传递 + Cloudflare 统一超时。
