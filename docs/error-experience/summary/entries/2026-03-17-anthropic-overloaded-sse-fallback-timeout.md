Summary: Anthropic 在 SSE 流体内返回 `overloaded_error` 事件（HTTP 200，非 5xx），导致 4 次重试全部失败（~20s）；fallback 使用耗尽的父 ctx（剩余 ~6s），kimi-for-coding 对大上下文请求（~25k tokens）来不及响应，用户见 `AI 服务暂时不可用（claude-sonnet-4-6）`。需三处修复：①`consumeAnthropicSSE` 返回带 `StatusCode=529` 的 `TransientError`；②fallback 使用独立有界 context；③failure_logging 识别 `overloaded_error` 为 transient。

## Metadata
- id: errsum-2026-03-17-anthropic-overloaded-sse-fallback-timeout
- tags: [summary, anthropic, overloaded, sse, fallback, retry, timeout]
- derived_from:
  - docs/error-experience/entries/2026-03-17-anthropic-overloaded-sse-fallback-timeout.md
