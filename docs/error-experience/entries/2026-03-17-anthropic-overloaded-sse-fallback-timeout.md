# Error Experience: Anthropic Overloaded SSE Error + Pinned Fallback Bypass

Date: 2026-03-17

## Error
```
执行失败：AI 服务暂时不可用（claude-sonnet-4-6），请稍后重试
```

Log evidence:
```
[WARN] anthropic client: stage=sse_consume error=anthropic stream error: overloaded_error: Overloaded
[WARN] llm-retry: LLM STREAM FAILED error_class=unknown error=anthropic stream error: overloaded_error: Overloaded
(×4 retries, ~21s total)
[no [FALLBACK] entries — pinned fallback never triggered]
[WARN] openai client: kimi-for-coding timeout: context deadline exceeded (6s)  ← Lark narration, NOT pinned fallback
```

## Root Cause

**`pinnedRateLimitFallbackClient` only recognized HTTP 429 as rate-limit trigger. `IsRateLimitError` did not match 429/overloaded, so the pinned fallback to `kimi-for-coding` was never activated.**

The absence of `[FALLBACK]` log entries confirms the pinned fallback never ran. The kimi 6s timeout in logs is from the Lark narration service (`narrate.go` default 6s), which is unrelated.

## What Actually Happened

### 1. SSE-level overloaded_error bypasses HTTP error path
Anthropic returns `overloaded_error: Overloaded` **inside the SSE stream body** (HTTP 200). `consumeAnthropicSSE` returned plain `fmt.Errorf` — no `StatusCode`. Downstream:
- `classifyLLMError` correctly detects "overloaded" → `TransientError` (retries fire ✓)
- `retryDelay` uses 1s base backoff instead of `rateLimitBaseDelay=5s` (StatusCode=0) ✗
- `failure_logging` classifies as `error_class=unknown` ✗

### 2. Pinned fallback blindly bypassed (TRUE ROOT CAUSE)
After retries exhaust, `pinnedRateLimitFallbackClient.StreamComplete` checks `IsRateLimitError(err)`. Before the fix, `IsRateLimitError` only checked `StatusCode==429` — it did NOT match `StatusCode==529` or the "overloaded" string. Result: `IsRateLimitError` returns `false`, fallback never activates, error surfaces to user.

### 3. Retry window too short
`isRateLimitError` in `retry_client_classify.go` also only matched 429, so 529 overload errors used 1s base delay. 4 retries × ~5s = ~20s. Anthropic's overload window often exceeds 20s.

## Fixes Applied

| Commit | Fix |
|--------|-----|
| `6721249f` | **Root cause**: `IsRateLimitError` matches 529 + "overloaded"; pinned fallback uses fresh 90s context |
| `3963c285` | SSE returns `TransientError{StatusCode:529}`; `tryFallbackStreamComplete` fresh ctx; `failure_logging` transient pattern |
| `06efa5e0` | 529 → `rateLimitBaseDelay=5s` in retryClient (≈35s total retry window) |
| `1ec631a4` | Fallback context propagates user cancellation |

## Code Locations
- `internal/app/agent/llmclient/rate_limit.go` — `IsRateLimitError` (pinned fallback gate)
- `internal/app/agent/preparation/llm_fallback.go` — `pinnedRateLimitFallbackClient.StreamComplete`
- `internal/infra/llm/anthropic_client.go` — SSE error handler
- `internal/infra/llm/retry_client_classify.go` — `isRateLimitError`, `retryDelay`
- `internal/infra/llm/retry_client_stream.go` — `tryFallbackStreamComplete`
- `internal/delivery/channels/lark/narrate.go` — 6s narration timeout (unrelated to pinned fallback)

## References
- Postmortem: docs/postmortems/incidents/2026-03-17-anthropic-overloaded-sse-fallback-timeout.md
