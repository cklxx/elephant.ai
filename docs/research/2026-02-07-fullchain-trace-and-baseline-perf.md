# 2026-02-07 全链路 Trace 与基础性能优化调研

## 背景
当前工程已有 HTTP/Task/SSE 层面 observability 基础，但 ReAct 迭代、LLM 流式调用、Tool 执行三个性能关键路径缺少统一 span，导致端到端性能分析粒度不足。

## 目标
- 打通可关联的 trace：`HTTP -> Task -> ReAct iteration -> LLM -> Tool -> SSE`。
- 在不改变功能语义的前提下，先落地低风险高收益的基础性能优化。

## 调研结论
- 已有 span：
  - HTTP 请求：`alex.http.request`
  - 任务执行：`alex.session.solve_task`
  - SSE 连接：`alex.sse.connection`
- 缺失/不足：
  - ReAct iteration 未落 span。
  - LLM 流式调用路径未落 span（仅有事件/日志）。
  - Tool 执行未落 span（仅有事件）。
  - SSE 与 active run 的 trace 关联不足。

## 本次落地
### 1) Trace 埋点增强
- 新增 ReAct trace helper，并在以下热路径接入：
  - `alex.react.iteration`
  - `alex.llm.generate`
  - `alex.tool.execute`
- 统一附加核心属性：
  - `alex.session_id`
  - `alex.run_id`
  - `alex.parent_run_id`
  - `alex.iteration`
  - `alex.status`
- SSE 连接 span 补充 active run 关联（当存在 active run 时写入 `alex.run_id`）。

### 2) 基础性能优化
- LLM 流式 delta 合批：
  - 将 delta 发送阈值从 1 字符提升到 64 字符，减少高频事件风暴与序列化开销。
- SSE 附件去重降本：
  - 用轻量签名替代 `json.Marshal + sha256(全对象)` 的重哈希路径；
  - 对无 `Data` 附件走字符串签名，保留 `Data` 时再走哈希。

## 验证
- 新增测试：
  - `internal/domain/agent/react/tracing_test.go`
  - `internal/delivery/server/http/sse_render_attachments_test.go`
- 回归：
  - `go test ./internal/domain/agent/react ./internal/delivery/server/http` 通过
  - `go test ./...` 通过
  - `golangci-lint` 针对改动目录通过；全量 lint 受仓库既有问题影响失败（与本次改动无关）

## 后续建议
- 在 trace 后端（OTLP/Zipkin）建立按 `run_id/session_id` 的端到端查询模板。
- 补充 ReAct/SSE 热路径基准测试，纳入 CI 趋势对比。
- 对 `EventBroadcaster` 高并发锁竞争优化单独立项（中风险，不建议与本次合并）。
