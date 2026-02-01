# 仓库 review 修复方案

Updated: 2026-02-01 01:00

## 目标
- 消除外部响应读取的内存风险。
- 保证事件历史在失败场景下不静默丢失。
- 规范调度器并发策略与 session 边界。
- 收敛事件广播的隔离风险。
- 优化长连接与存储路径的可取消性。

## 非目标
- 调整产品交互或 UI 体验。
- 改变现有工具 API 的业务语义（仅增强安全与稳定性）。

## 设计原则
- 安全优先：所有外部响应必须有上限。
- 可观测性：失败必须显式暴露（metrics + 日志）。
- 可回滚：策略参数通过 `.yaml` 配置切换。
- 兼容性：不改变既有接口签名与输出格式（除非必要）。

---

## Phase 1 — 外部 HTTP 响应大小上限

### 方案
- 在工具层统一引入 `max_response_bytes` 并默认启用。
- 对 “内容分析/网页抓取/第三方搜索/媒体查询/模型列表”等路径进行 `io.LimitReader`。
- 超限时返回明确错误并记录指标。

### 涉及模块
- `internal/tools/builtin/web/*`
- `internal/tools/builtin/media/music_play.go`
- `internal/subscription/catalog.go`
- `internal/server/http/runtime_models.go`
- `internal/sandbox/client.go`

### 配置示例（YAML）
```yaml
# configs/runtime.yaml
http_limits:
  default_max_response_bytes: 1048576   # 1 MiB
  web_fetch_max_response_bytes: 2097152 # 2 MiB
  web_search_max_response_bytes: 1048576
  music_search_max_response_bytes: 1048576
  model_list_max_response_bytes: 524288
```

### 风险与取舍
- 过低上限可能截断合法内容；需通过配置逐步校准。
- 建议优先对 web_fetch/web_search 设高于默认的上限。

---

## Phase 2 — AsyncEventHistoryStore 可靠性提升

### 方案
- `flushBuffer` 失败时 **保留 buffer** 并指数退避重试。
- 超过重试阈值后将事件写入本地死信（可选）或警报。
- 对 `Stream/Delete/HasSessionEvents` 的 flush 失败路径增加 metrics。

### 设计要点
- 保持现有异步队列与 append timeout，不影响实时链路。
- 失败时允许有限阻塞或降级，避免持续丢失。

---

## Phase 3 — Scheduler 并发策略与 session 隔离

### 方案
- 每次触发生成唯一 sessionID（如 `scheduler-<name>-<run_id>`）。
- 使用 `cron.WithChain(cron.SkipIfStillRunning)` 或显式互斥。
- 为每次 trigger 加超时，超时后强制取消。

### 配置示例（YAML）
```yaml
# configs/runtime.yaml
scheduler:
  enabled: true
  trigger_timeout_seconds: 900
  concurrency_policy: "skip" # skip | delay
```

---

## Phase 4 — EventBroadcaster session 缺失处理

### 方案
- 将“全局事件”与“缺失 sessionID”分离：仅明确标记的 global 事件允许广播。
- 对缺失 sessionID 的事件改为告警 + drop。

### 影响
- 消除跨 session 泄露风险。
- 降低多会话广播成本。

---

## Phase 5 — 长连接与存储路径优化

### SSE 连接
- 对 `lastSeqByRun` 设上限（LRU 或基于时间清理）。

### FileStore 列表
- 列表读取改用调用方 ctx。
- 增加轻量索引或仅读取元信息字段。

### Attachment 存储
- Cloudflare 操作使用调用方 ctx 或统一超时。

---

## 测试计划
- 新增/更新单测：
  - HTTP 限制触发路径（超限返回错误）。
  - AsyncEventHistoryStore 失败时 buffer 保留与重试。
  - Scheduler 并发策略与 sessionID 生成规则。
- 运行全量 `./dev.sh lint` 与 `./dev.sh test`。

## 发布与回滚
- 所有行为通过 `.yaml` 配置开关控制。
- 分阶段灰度：先开启观测与限制，后启用强制拒绝。
