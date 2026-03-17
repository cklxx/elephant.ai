根因：`pinnedRateLimitFallbackClient.IsRateLimitError` 只匹配 HTTP 429，不匹配 529/overloaded，导致 kimi-for-coding pinned fallback 被完全绕过——fallback 已配置但从未触发。次要问题：① SSE `overloaded_error` 无 StatusCode，retryDelay 用 1s 而非 5s；② fallback context 未隔离，继承父 ctx 已耗尽 deadline；③ Lark narrate 6s timeout（独立路径，非 pinned fallback）。共 4 个 commit 修复：`6721249f`（根因）、`3963c285`（SSE+fallback ctx）、`06efa5e0`（529→5s backoff）、`1ec631a4`（user cancel 传播）。

## Metadata
- id: errsum-2026-03-17-anthropic-overloaded-sse-fallback-timeout
- tags: [summary, anthropic, overloaded, sse, fallback, pinned-fallback, rate-limit, bypass]
- derived_from:
  - docs/error-experience/entries/2026-03-17-anthropic-overloaded-sse-fallback-timeout.md
