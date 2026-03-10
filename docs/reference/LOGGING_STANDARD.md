# Logging Standards

Updated: 2026-03-10

## Principles

- Every request-scoped log includes `log_id`.
- Consistent keys: `log_id`, `session_id`, `task_id`, `parent_task_id`, `request_id`.
- Human-readable, grep-correlatable.
- No per-iteration INFO spam on hot paths.

## Required Fields

- `log_id` — primary correlation key.
- `request_id` — LLM/vendor call key (embeds `log_id`).
- `session_id` / `task_id` — conversation/run grouping.

## Format

- Use `log_id=<id>` (not `logid=`).
- LLM logs: `[log_id=<id>]` + `[req:<request_id>]`.
- Subagent runs: `<parent_log_id>:sub:<new_log_id>`.

## Context

- Entry points generate `log_id` if missing.
- Use `logging.FromContext(...)` when context available.
- CLI: `cliBaseContext()` per invocation, `clilatency.PrintfWithContext(...)` for timing.

## Level Policy

| Level | When |
|-------|------|
| ERROR | Execution failed, user-impacting |
| WARN | Degraded with fallback/retry |
| INFO | Lifecycle milestones only (start/stop, task complete) |
| DEBUG | High-frequency internals (iterations, tool details, counters) |

## Noise Control

- No duplicate component names when logger already scopes.
- No same-transition logs in both caller and callee unless adding different dimensions.
- Hot paths: use DEBUG or sampling, not INFO.
- Prefer one summary log over many step-by-step logs for deterministic sequences.
- No large dynamic payloads in routine logs.

## Review Checklist

- Needed for incident triage within 2 min?
- Level aligned with impact, not curiosity?
- Fires in a loop/hot path?
- Existing log already expresses this?
- Can IDs replace verbose text?
