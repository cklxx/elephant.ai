# Incident Postmortem: Anthropic Overloaded SSE Error + Pinned Fallback Bypass

## Incident
- Date: 2026-03-17
- Incident ID: INC-2026-03-17-01
- Severity: P1 (recurring user-visible task failure)
- Component(s): `app/agent/llmclient/rate_limit.go`, `app/agent/preparation/llm_fallback.go`, `infra/llm/anthropic_client`, `infra/llm/retry_client`
- Reporter: ckl

## What Happened
- Symptom: Users see `执行失败：AI 服务暂时不可用（claude-sonnet-4-6），请稍后重试` on task execution. Recurring at 15:49, 17:26, 17:31 UTC+8.
- Trigger condition: Anthropic API returns an `overloaded_error: Overloaded` **within the SSE stream body** (HTTP 200, not 5xx). All 4 retry attempts exhaust (~20s). Pinned fallback to `kimi-for-coding` was armed but **never activated** because `IsRateLimitError` did not recognize 529/overloaded.
- Detection channel: User report + `alex-llm.log` grep.

## Impact
- User-facing impact: Task execution fails entirely. Repeated occurrences in the same hour (two confirmed failure pairs).
- Internal impact: All retries exhaust before Anthropic service recovers. Pinned fallback (kimi-for-coding) is configured but completely bypassed.
- Blast radius: Any session routed to `anthropic/claude-sonnet-4-6` during Anthropic overload window.

## Timeline (absolute dates)
1. 2026-03-17 15:49:15 — First `overloaded_error` in SSE stream for `log-3B41tyEbzljwgldnmMnxzA5EIGV`. Retried 4× (~20s). All fail. Pinned fallback not triggered.
2. 2026-03-17 15:49:25 — Subsequent requests succeed after ~10s pause. Suggests brief overload window.
3. 2026-03-17 17:26:31 — Second incident: `log-3B4DjTzWXAwKCQWo2kYcyUsVPsE`. Same pattern: overloaded_error 4× retries (21.182s), pinned fallback bypassed. User sees error.
4. 2026-03-17 17:31:09 — Third hit within same session: `log-3B4EFn0vh4iOkrsWsP6a68szr4J`. Same pattern (22.065s). User sees error again.
5. 2026-03-17 — User reports recurring issue, investigation initiated.

## Root Cause

**`pinnedRateLimitFallbackClient` only recognized HTTP 429 as rate limit. Anthropic `overloaded_error` (529) bypassed the pinned fallback entirely, returning the error to the user even though a fallback (kimi-for-coding) was armed and available.**

`IsRateLimitError` (in `app/agent/llmclient/rate_limit.go`) checked `StatusCode == 429` but not 529. The SSE-path `overloaded_error` has no HTTP status code at all (HTTP response is 200), so both the StatusCode check and string pattern check failed to match — `IsRateLimitError` returned `false`, the pinned fallback condition was never satisfied, and the error propagated to the user unchanged.

The apparent "kimi-for-coding timeout" seen in logs was the **Lark narration service** (6s timeout in `narrate.go`), not the pinned fallback — the pinned fallback never ran.

### Contributing failures (secondary)

1. **SSE error bypasses HTTP classification** — `overloaded_error` arrives inside the stream body (HTTP 200), so `mapHTTPError` is never called. `consumeAnthropicSSE` returned plain `fmt.Errorf` with no `StatusCode`, causing:
   - `retryDelay` to use 1s base backoff instead of `rateLimitBaseDelay=5s`, so 4 retries exhaust in ~20s instead of ~35s
   - `failure_logging` to classify as `error_class=unknown` instead of `transient`

2. **Fallback context not isolated** — Both `pinnedRateLimitFallbackClient` and `tryFallbackStreamComplete` passed the caller's nearly-exhausted context to the fallback (secondary issue; only matters once the detection bug is fixed).

3. **529 not treated as rate-limit backoff in `retryClient`** — `isRateLimitError` in `retry_client_classify.go` only checked `StatusCode==429`, so 529 overload errors used 1s base delay even after being correctly tagged as `TransientError{StatusCode:529}`.

## Fixes Applied

**Fix 1 — TRUE ROOT CAUSE** (`6721249f`): `IsRateLimitError` now matches `StatusCode==529` and "overloaded" string pattern. `pinnedRateLimitFallbackClient.StreamComplete/Complete` use `context.WithTimeout(context.WithoutCancel(ctx), 90s)` for fresh fallback budget.

**Fix 2** (`3963c285`): `consumeAnthropicSSE` returns `alexerrors.NewTransientError` with `StatusCode=529` for `overloaded_error`. `tryFallbackStreamComplete` uses fresh 90s context. `failure_logging` recognizes "overloaded_error" as transient.

**Fix 3** (`06efa5e0`): `isRateLimitError` in `retry_client_classify.go` now matches `StatusCode==529`, so 529 overload errors use `rateLimitBaseDelay=5s` (≈35s total retry window).

**Fix 4** (`1ec631a4`): Fallback context propagates user cancellation (not just deadline) — prevents zombie fallback requests when user cancels the task.

## Prevention Actions
1. ✅ Fix `IsRateLimitError` to recognize 529 and "overloaded" for pinned fallback trigger — `6721249f`
2. ✅ Wrap SSE `overloaded_error` in `TransientError{StatusCode:529}` — `3963c285`
3. ✅ Fresh context for fallback calls (both pinned and retry-level) — `6721249f`, `3963c285`, `1ec631a4`
4. ✅ 529 uses `rateLimitBaseDelay=5s` in `retryClient` — `06efa5e0`
5. ✅ Fix `failure_logging` to classify `overloaded_error` as transient — `3963c285`
6. ⬜ Add `overloaded_error` frequency alert to LLM error dashboard — due 2026-03-31

## Follow-ups
- Open risks: Anthropic `overloaded_error` frequency may increase as model usage grows. Current design does not protect against primary + fallback simultaneous failure.
- Deferred items: Consider a second fallback (e.g. claude-3-7-sonnet via API key) if kimi-for-coding also fails. Needs config design.

## Metadata
- id: INC-2026-03-17-01
- tags: [llm, anthropic, overloaded, sse, fallback, pinned-fallback, rate-limit, bypass]
- links:
  - related-postmortem: docs/postmortems/incidents/2026-03-02-session-error-persistence-gap.md
  - log: logs/alex-llm.log (2026-03-17 15:49, 17:26, 17:31)
  - commits: 6721249f, 3963c285, 06efa5e0, 1ec631a4
