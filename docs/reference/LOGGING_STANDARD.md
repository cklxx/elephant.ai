# Logging standards

This document defines the logging conventions for all runtime components
(server, CLI, tools, and LLM clients).

## Goals
- Every request-scoped log line includes `log_id`.
- Consistent key naming: use `log_id`, `session_id`, `task_id`, `parent_task_id`,
  and `request_id`.
- Logs remain human-readable while enabling grep-based correlation.

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
