# 2026-01-22 - workflow events missing session context

- Summary: Early workflow envelopes emitted without session/task IDs were broadcast globally and replayed into all sessions, showing up as duplicated/unclear events.
- Remediation: populate `OutputContext` from request context IDs before creating the workflow tracker; assert session ID on the prepare node in tests.
