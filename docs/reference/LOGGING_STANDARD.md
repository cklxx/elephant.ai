# Logging standards

This document defines the logging conventions for all runtime components
(server, CLI, tools, and LLM clients).

## Goals
- Every request-scoped log line includes `log_id`.
- Consistent key naming: use `log_id`, `session_id`, `task_id`, `parent_task_id`,
  and `request_id`.
- Logs remain human-readable while enabling grep-based correlation.
- Keep signal-to-noise high on hot paths (no per-iteration/per-event INFO spam).

## Required fields
- **log_id**: primary correlation key for a single run.
- **request_id**: external request key (LLM/vendor), should embed `log_id`.
- **session_id/task_id**: used for conversation/run grouping.

## Standard prefixes
- Use `log_id=<id>` (not `logid=`).
- LLM logs must include `[log_id=<id>]` plus `[req:<request_id>]`.
- Subagent runs must generate a derived log_id using the parent as a prefix:
  `<parent_log_id>:sub:<new_log_id>`.

## Context handling
- Entry points must generate a `log_id` if missing.
- When a context is available, wrap logs with `logging.FromContext(...)` or
  include `log_id` in the message prefix.
- CLI commands use `cliBaseContext()` to ensure a `log_id` per invocation.
- CLI latency output should use `clilatency.PrintfWithContext(...)` to keep
  `log_id` visible in stderr timing lines.

## Level policy (noise control)
- `ERROR`: execution failed and user-impacting behavior cannot proceed.
- `WARN`: degraded behavior with fallback/retry/drop occurred.
- `INFO`: lifecycle milestones only (start/stop, task completed, one-shot summaries).
- `DEBUG`: high-frequency internals (iteration loop, tool-call details, stream counters).

## Redundancy rules
- Do not duplicate component names in message text when component logger already scopes logs.
- Avoid logging the same state transition in both caller and callee unless they add different dimensions.
- In loops/hot paths, avoid `INFO` logs for every iteration/event; use `DEBUG` or sampling.
- Prefer one summary log over many step-by-step logs when steps are deterministic and already observable via events/metrics.
- Do not include large dynamic payloads in routine logs (task body, long args, full session list).

## Review checklist
- Is this log needed for incident triage within 2 minutes?
- Is the chosen level aligned with impact, not developer curiosity?
- Will this line fire in a loop or high-frequency code path?
- Does an existing log already express the same event?
- Can IDs (`log_id/session_id/task_id`) replace verbose textual repetition?

## Example (YAML)
```yaml
timestamp: 2026-01-27 12:34:56
level: INFO
category: SERVICE
component: Router
log_id: log-20260127-001
message: "POST /api/tasks from 127.0.0.1"
```

## LLM request example (YAML)
```yaml
timestamp: 2026-01-27 12:35:10
level: DEBUG
category: LLM
component: openai
log_id: log-20260127-001
request_id: log-20260127-001:llm-abc123
message: "=== LLM Request ==="
```
