# Workflow migration TODOs

## Done
- Workflow snapshots are now attached to every coordinator return path, including cancellations and preparation failures, so debuggers receive the same structure regardless of outcome.
- Added SSE reconnection coverage to replay `workflow_event` and `step_completed` payloads so reconnecting clients can render ordered nodes.
- Delegated subagent executions expose their workflow snapshots in structured tool metadata so downstream evaluators and UI clients can inspect nested runs alongside the primary task.

## Remaining
- None (keep this list current as new gaps are identified)
