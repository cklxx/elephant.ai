# Kernel API rate-limit bursts under minute-level cadence with shared subscription

**Date:** 2026-02-12
**Severity:** High
**Category:** Runtime Reliability / LLM Throughput

## What happened

Kernel dispatches intermittently failed with:

- `LLM call failed: API rate limit reached. Retrying with exponential backoff. Streaming request failed after 9s.`

Observed in `~/alex-kernel.log` across consecutive minute ticks for `autonomous-state-loop`.

## Root cause

1. Kernel was executed at minute-level cadence during repeated validation windows (`* * * * *`).
2. The same upstream subscription/profile was shared by multiple request paths and repeated cycles.
3. Consecutive cycles produced bursty LLM demand in a short window, exceeding provider quota windows.

Prompt assembly itself was not the primary issue: sampled dispatch prompts were bounded (~600-1000 chars) and successful cycles with the same prompt structure existed immediately before/after failures.

## Fix / mitigation

1. Keep production kernel cadence at `0,30 * * * *` (30-minute schedule) to avoid unnecessary burst pressure.
2. Preserve pinned selection propagation (`channel/chat/user -> resolved selection`) so kernel uses the intended profile consistently.
3. Keep post-condition guard: kernel dispatch is successful only when at least one non-orchestration tool action succeeds.

## Validation

- Real non-mock e2e run with current branch binary produced successful cycles (`run-9Zu4YZwvNvxj`, `run-BCqJR1eFnI8H`) without rate-limit errors in the controlled window.
- `STATE.md` runtime block persisted `agent_summary`; `SYSTEM_PROMPT.md` refreshed; `INIT.md` stayed immutable.

## Lessons

- For proactive loops, throughput/cadence design is a first-class reliability control.
- Shared subscription quotas require conservative default cadence and burst avoidance during validations.
