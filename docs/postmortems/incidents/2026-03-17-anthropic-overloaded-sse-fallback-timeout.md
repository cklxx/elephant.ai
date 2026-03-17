# Incident Postmortem: Anthropic Overloaded SSE Error + Fallback Timeout

## Incident
- Date: 2026-03-17
- Incident ID: INC-2026-03-17-01
- Severity: P1 (recurring user-visible task failure)
- Component(s): `infra/llm/anthropic_client`, `infra/llm/retry_client`, `infra/llm/failure_logging`
- Reporter: ckl

## What Happened
- Symptom: Users see `执行失败：AI 服务暂时不可用（claude-sonnet-4-6），请稍后重试` on task execution. Recurring at 15:49, 17:26, 17:31 UTC+8.
- Trigger condition: Anthropic API returns an `overloaded_error: Overloaded` **within the SSE stream body** (not as an HTTP 5xx). All 4 retry attempts exhaust (~20s). Fallback to `kimi-for-coding` is attempted but also fails with `context deadline exceeded` (6s remaining after retries).
- Detection channel: User report + `alex-llm.log` grep.

## Impact
- User-facing impact: Task execution fails entirely. Repeated occurrences in the same hour (two confirmed failure pairs).
- Internal impact: All retries exhaust before Anthropic service recovers. Fallback always times out.
- Blast radius: Any session routed to `anthropic/claude-sonnet-4-6` during Anthropic overload window.

## Timeline (absolute dates)
1. 2026-03-17 15:49:15 — First `overloaded_error` in SSE stream for `log-3B41tyEbzljwgldnmMnxzA5EIGV`. Retried 4x (~20s). All fail.
2. 2026-03-17 15:49:25 — Subsequent requests succeed after ~10s pause. Suggests brief overload window.
3. 2026-03-17 17:26:31 — Second incident: `log-3B4DjTzWXAwKCQWo2kYcyUsVPsE`. Same pattern: overloaded_error 4x retries (21.182s), fallback kimi-for-coding times out (6.001s). User sees error.
4. 2026-03-17 17:31:09 — Third hit within same session: `log-3B4EFn0vh4iOkrsWsP6a68szr4J`. Same pattern (22.065s), fallback also times out. User sees error again.
5. 2026-03-17 — User reports recurring issue, investigation initiated.

## Root Cause

Three-layer failure:

**Layer 1 (External — Anthropic)**: Anthropic's `overloaded_error` is delivered as an SSE `event.type=error` payload inside the response stream body, not as an HTTP 5xx status code. This occurs after the HTTP handshake succeeds (200 OK), so network-level and HTTP-level error handling (e.g. `mapHTTPError`) are bypassed.

**Layer 2 (Code — SSE error classification)**:
- `consumeAnthropicSSE` returns `fmt.Errorf("anthropic stream error: overloaded_error: Overloaded")` — a raw error with no `StatusCode`.
- `classifyLLMError` correctly detects "overloaded" and wraps it as `TransientError`, so retries DO happen. However:
  - `failure_logging.go` logs this as `error_class=unknown` (because it reads the raw error before `classifyLLMError` wraps it), creating a misleading metric.
  - `retryDelay` uses the default 1s base backoff (not the `rateLimitBaseDelay=5s` path), because `is429` checks `StatusCode == 429` — but SSE-path errors have `StatusCode=0`.

**Layer 3 (Design — fallback timing)**:
- By the time all 4 retries exhaust (~20s), the task context's remaining deadline is ~6s.
- `tryFallbackStreamComplete` passes the **same `ctx`** to the fallback client. For large context requests (~25k tokens), 6s is insufficient for `kimi-for-coding` to respond.
- Result: fallback always times out with `context deadline exceeded`, and the user sees the primary's error.

**Why existing checks did not catch it**:
- The `overloaded_error` is a new failure mode not covered by existing unit tests (which mock 5xx HTTP errors).
- No monitoring alert on `overloaded_error` frequency in SSE streams.
- The fallback timing issue was not tested against large-context requests.

## Fix

No immediate code change deployed (external upstream issue). Identified three improvements:

**Short-term (logging fix)**:
- Fix `failure_logging.go`'s `classifyFailureError` to recognize `overloaded_error` SSE pattern as `transient` instead of `unknown`. This does not change behavior but fixes metrics.

**Medium-term (backoff fix)**:
- In `consumeAnthropicSSE`, when returning `overloaded_error`, return a structured error (e.g. `alexerrors.NewTransientError` with a synthetic `StatusCode=529`) so that `retryDelay` applies the more conservative backoff path, giving the service more time to recover between attempts.

**Medium-term (fallback context fix)**:
- In `tryFallbackStreamComplete`, derive a **fresh deadline** for the fallback attempt (e.g. `min(remaining, 30s)`) rather than passing the potentially-exhausted parent context. This ensures the fallback always has a meaningful time budget.

## Prevention Actions
1. Action: Fix `classifyFailureError` to recognize SSE `overloaded_error` as `transient`
   Owner: ckl
   Due date: 2026-03-24
   Validation: `error_class=transient` in failure logs for `overloaded_error` pattern

2. Action: Wrap SSE `overloaded_error` in `alexerrors.NewTransientError` with `StatusCode=529` in `consumeAnthropicSSE`
   Owner: ckl
   Due date: 2026-03-24
   Validation: Retry delay for SSE overloaded uses exponential backoff starting at ≥5s

3. Action: Use a fresh bounded context for `tryFallbackStreamComplete` (not the caller's nearly-exhausted ctx)
   Owner: ckl
   Due date: 2026-03-24
   Validation: Fallback succeeds in integration test where primary takes 20s to exhaust retries

4. Action: Add `overloaded_error` frequency alert to LLM error dashboard
   Owner: ckl
   Due date: 2026-03-31
   Validation: Alert fires within 5 min if ≥3 overloaded_error events in 10 min window

## Follow-ups
- Open risks: Anthropic `overloaded_error` frequency may increase as model usage grows. Current fallback design does not fully protect against primary + fallback simultaneous failure.
- Deferred items: Consider a second fallback (e.g. claude-3-7-sonnet via API key) if kimi-for-coding also fails. Needs config design.

## Metadata
- id: INC-2026-03-17-01
- tags: [llm, anthropic, overloaded, sse, fallback, retry, timeout]
- links:
  - related-postmortem: docs/postmortems/incidents/2026-03-02-session-error-persistence-gap.md
  - log: logs/alex-llm.log (2026-03-17 15:49, 17:26, 17:31)
