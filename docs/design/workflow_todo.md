# Workflow migration TODOs

## Done
- Workflow snapshots are now attached to every coordinator return path, including cancellations and preparation failures, so debuggers receive the same structure regardless of outcome.
- Added SSE reconnection coverage to replay `workflow_event` and `step_completed` payloads so reconnecting clients can render ordered nodes.

## Remaining
- Extend the shared workflow tracker to other agent entrypoints (e.g., delegated subagents or evaluation flows) to keep observability consistent across task types.
