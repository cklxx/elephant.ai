# Workflow Event Duplication + Session Context

**Goal:** Stop ambiguous repeated workflow events by ensuring workflow envelopes carry session/task IDs from the start of execution.

## System View
- Workflow stage events (prepare/execute/summarize/persist) are emitted by `agentWorkflow` via `workflowEventBridge`.
- `workflowEventBridge` seeds its base context from `ports.OutputContext`, not the request/session context.
- In server flows, output context often lacks `SessionID`/`TaskID` even when the request context has them, so early workflow events are emitted with empty session/task IDs.
- SSE treats empty-session events as global, which leaks them into all sessions and feels like duplicates/noise.

## Plan
1) Propagate session/task IDs from context into `OutputContext` before creating the workflow tracker.
2) Add a test that asserts `workflow.node.started` (prepare) includes the expected session ID.
3) Run full lint/tests.

## Progress Log
- 2026-01-22: Planned fix for session context propagation to eliminate ambiguous workflow events.
- 2026-01-22: Hydrated OutputContext IDs in ExecuteTask; added session propagation test for prepare node; recorded error-experience entry.
- 2026-01-22: Ran `./dev.sh lint` and `./dev.sh test` (passes; happy-dom AbortError noise after vitest teardown).
