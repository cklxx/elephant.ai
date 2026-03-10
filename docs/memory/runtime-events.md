# Runtime & Events

Updated: 2026-03-10 18:00

## Event Partitioning

- `groupKey` uses `parent_run_id`.
- A run is a subagent run when `parent_run_id != run_id`.
- The main stream shows only the subagent `workflow.tool.started` event; noisy tool events stay pending or merged.
- Replans caused by tool failure must emit `workflow.replan.requested` explicitly.

## Streaming Rules

- Dedup with bounded caches and cap response reads.
- Drop deltas, tool progress, and chunk events from session history.
- Use lightweight attachment signatures instead of hashing full objects.
- Keep retention and backpressure on event buffers and per-session metrics maps.

## Subagent And Proactive Rules

- Snapshot pruning must also drop tool outputs for pruned call IDs.
- Cap and stagger parallel subagents to reduce upstream rejection.
- Proactive hooks should use a registry plus per-request memory policy.
- Persona-level `always confirm` rules need delegated-autonomy guardrails.
- Only coalesce flushes when there is real contention.

## Infrastructure

- Restart recovery needs persisted tasks plus a startup resume hook.
- Tool SLA collection and tracing must be wired end to end.
- Keep observability workbench features consolidated and lazily expanded.
- Prefer host CLI exploration and inject only safe, redacted env hints.
