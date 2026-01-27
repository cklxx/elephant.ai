# Domain layers and ID semantics

## Scope
This document defines the domain layering model and the ID semantics used for
end-to-end correlation. It is the single source of truth for which IDs exist,
where they are generated, and how they propagate.

## Domain layering
- **Domain** (`internal/agent/domain`, `internal/workflow`): pure business logic
  and domain events. No side effects, no IO, no config access.
- **Application** (`internal/agent/app`, `internal/server/app`): orchestration,
  workflows, use cases, and coordination across domain + ports.
- **Ports** (`internal/agent/ports`, `internal/tools/ports`): interfaces, DTOs,
  and contracts for inbound/outbound behavior.
- **Adapters/Infrastructure** (`internal/server/http`, `internal/llm`,
  `internal/logging`, `internal/utils`): IO, storage, network, logging, and
  vendor integrations.

IDs are created at the application boundary and propagated downward via context
and output envelopes. Domain types can carry IDs but should not generate them.

## ID taxonomy
- **session_id**: conversation or user thread identifier. Stable across multiple
  requests; used for grouping history.
- **log_id** (primary trace key): single request / single agent execution
  identifier. Used to join logs, SSE payloads, LLM requests, and tool traces.
- **task_id**: agent execution instance within a session.
- **parent_task_id**: parent execution for subagent or delegated runs.
  - Subagent log_id is derived as `<parent_log_id>:sub:<new_log_id>` to preserve correlation.
- **workflow_id**: internal workflow instance identifier for a run.
- **node_id / step_index / iteration**: workflow node identifiers for stage
  timing and debugging.
- **request_id / llm_request_id**: external request identifiers (LLM or vendor).

## Correlation rules
1. **log_id is the primary correlation key** for a single run. Every log line,
   SSE payload, and tool/LLM request should carry it.
2. **session_id groups runs** and should not be used as the sole trace key.
3. **task_id and parent_task_id** define the execution tree under a session.
4. **workflow_id and node identifiers** describe internal stages and timing.

## Generation and propagation
- **Entry points** (HTTP/CLI/worker) must ensure a log_id and task_id exist:
  use `internal/utils/id` helpers and store IDs on context.
- **OutputContext** (`internal/agent/ports/agent`) is the rendering/streaming
  carrier for session_id, task_id, parent_task_id, and log_id.
- **Logging** should always use context-backed IDs (`logging.FromContext`)
  so `log_id` is present in service, LLM, and latency logs.

## Example event envelope (YAML)
```yaml
event_type: workflow.node.completed
session_id: session-abc123
task_id: task-xyz789
parent_task_id: task-parent-001
log_id: log-20260127-001
payload:
  workflow_id: wf-42
  node_id: execute
  step_index: 1
  iteration: 2
  duration_ms: 1534
```

## Practical defaults
- **Prefer log_id for trace lookups** (dev panel, log bundle fetch).
- **Use session_id for history views** and report aggregation.
- **Use task_id + parent_task_id** for subagent debugging and fan-out tracing.
