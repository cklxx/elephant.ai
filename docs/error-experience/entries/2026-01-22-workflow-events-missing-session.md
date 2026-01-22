# 2026-01-22 - workflow events missing session context

- Error: `workflow.node.started` envelopes emitted during the prepare stage lacked session/task IDs, so SSE treated them as global and replayed them across sessions, appearing as duplicated/noise events.
- Remediation: hydrate `OutputContext` with IDs from request context before creating the workflow tracker, and add a test to assert the prepare event carries the session ID.
