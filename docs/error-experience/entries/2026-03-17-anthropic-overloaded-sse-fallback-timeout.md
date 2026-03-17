# Error Experience: Anthropic Overloaded SSE Error + Fallback Timeout

Date: 2026-03-17

## Error
```
执行失败：AI 服务暂时不可用（claude-sonnet-4-6），请稍后重试
```

Log evidence:
```
[WARN] anthropic client: stage=sse_consume error=anthropic stream error: overloaded_error: Overloaded
[WARN] llm-retry: LLM STREAM FAILED error_class=transient error=Server overloaded (529). Retrying request.
(×4 retries, ~21s total)
[WARN] openai client: kimi-for-coding timeout: context deadline exceeded (6s)
[WARN] llm-retry: LLM COMPLETE FAILED error_class=transient error=context cancelled during retry: context deadline exceeded
```

## What Actually Happened

Three compounding failures:

### 1. SSE-level overloaded_error bypasses HTTP error path
Anthropic returns `overloaded_error: Overloaded` **inside the SSE stream body** (event.type=error), not as HTTP 5xx. So:
- `mapHTTPError` is never called (HTTP status is 200)
- `consumeAnthropicSSE` returns a plain `fmt.Errorf("anthropic stream error: overloaded_error: Overloaded")`
- `classifyLLMError` correctly identifies "overloaded" → `TransientError`, retries fire ✓
- BUT: failure logging classifies it as `error_class=unknown` (reads raw error before classification) ✗
- BUT: `retryDelay` uses 1s base backoff (not `rateLimitBaseDelay=5s`) because `StatusCode=0` ✗

### 2. All 4 retries exhaust before service recovers
~5s per attempt × 4 = ~20s. If Anthropic's overload lasts >20s, all retries fail.

### 3. Fallback runs on exhausted context
After ~20s of retries, the fallback (`kimi-for-coding`) is invoked using the **same parent `ctx`**.
For large context requests (~25k tokens), kimi-for-coding needs >6s. The remaining deadline is ~6s → always times out.

## Code Locations
- `internal/infra/llm/anthropic_client.go:377-386` — SSE error handler, returns plain `fmt.Errorf`
- `internal/infra/llm/retry_client_classify.go:79-81` — 529/overloaded rule (correct), but `StatusCode` not set from SSE path
- `internal/infra/llm/retry_client_stream.go:116-122` — fallback invocation passes caller's `ctx`
- `internal/infra/llm/failure_logging.go` — `classifyFailureError` doesn't recognize SSE overloaded pattern

## Fixes Needed (not yet applied)
1. `consumeAnthropicSSE`: return `alexerrors.NewTransientError(...).WithStatusCode(529)` for `overloaded_error`
2. `tryFallbackStreamComplete`: use `context.WithTimeout(ctx, min(remaining, 30s))` for fallback
3. `failure_logging.go`: add "overloaded_error" to transient pattern list in `classifyFailureError`

## References
- Postmortem: docs/postmortems/incidents/2026-03-17-anthropic-overloaded-sse-fallback-timeout.md
