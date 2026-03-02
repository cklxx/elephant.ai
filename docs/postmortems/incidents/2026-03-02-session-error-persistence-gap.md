# Incident Postmortem: Session Error Persistence Gap

## Incident
- Date: 2026-03-02
- Incident ID: INC-2026-03-02-01
- Severity: P1 (observability gap, no data loss)
- Component(s): `coordinator`, `session_manager`, `react/runtime`
- Reporter: ckl

## What Happened
- Symptom: When LLM calls fail during the think step (503 upstream unavailable, 401 auth failure), the error message is logged but NOT persisted to the session. Inspecting sessions after failure shows no trace of what went wrong — only the pre-error messages (system prompt + user input) are stored.
- Trigger condition: Any `executionErr` from `reactEngine.SolveTask()` — LLM transient errors (503), auth failures (401), context overflow, etc.
- Detection channel: Manual log inspection after kernel dispatch failures across all agents (founder-operator, build-executor, audit-executor, research-executor, data-executor) at 23:09 UTC+8.

## Impact
- User-facing impact: Sessions that failed due to LLM errors appear as if nothing happened. No error message, no stop reason, no timestamp of failure. Unable to diagnose issues from session inspection alone.
- Internal impact: Kernel dispatch failures are logged but not traceable back to sessions. Debugging requires cross-referencing kernel logs with session IDs manually.
- Blast radius: All agent execution paths that end in `executionErr != nil` — both kernel dispatches and direct API task execution.

## Timeline (absolute dates)
1. 2026-03-02 20:38 — First observed: all 5 kernel agents fail with `Authentication failed` (Claude Sonnet 4.6 API key issue). Errors logged to kernel log but sessions have no error metadata.
2. 2026-03-02 20:38–22:38 — Repeated every 30 min. Same pattern: auth failures, no session error trace.
3. 2026-03-02 23:09 — Error changes to `Upstream service temporarily unavailable` (Anthropic 503). 5 agents fail with 35-40s streaming timeout. Error still not in sessions.
4. 2026-03-02 23:28 — Investigation begins. Root cause identified: `coordinator.go` and `session_manager.go` never write error info to session metadata.
5. 2026-03-02 23:39 — Fix deployed and verified with real request. Session metadata now correctly contains `last_error`, `last_error_at`, `stop_reason`.

## Root Cause
- Technical root cause: Three gaps in the error-to-session persistence pipeline:
  1. `coordinator.go:449-460` creates a synthetic `TaskResult` when `SolveTask` returns `nil, err`, but the result has no error field — only `StopReason: "error"` and `Messages: env.State.Messages` (pre-error state).
  2. `session_manager.go:SaveSessionAfterExecution` persists `session_id`, `last_task_id`, `last_parent_task_id` to metadata, but never `stop_reason` or any error information.
  3. The `executionErr` error object is returned to the caller at `coordinator.go:545` but its message string is never stamped onto the session.
- Process root cause: No test coverage for the "execution error → session metadata" path. Existing tests only verified successful execution persistence.
- Why existing checks did not catch it: Session persistence tests (`session_manager_test.go`) only tested the happy path (save session ID, run ID, parent run ID). No test case for error metadata. Code review did not flag the missing error persistence when the error path was originally implemented.

## Fix
- Code/config changes:
  1. `coordinator.go`: Added error stamping block before `SaveSessionAfterExecution` — writes `last_error` (error message) and `last_error_at` (UTC timestamp) to session metadata when `executionErr != nil`.
  2. `session_manager.go`: Added `stop_reason` persistence to session metadata on every save. Added cleanup logic: `last_error` and `last_error_at` are automatically cleared on successful execution (non-"error" stop reason).
  3. `session_manager_test.go`: Added 4 test cases covering: stop_reason persistence, stop_reason cleanup, error metadata cleanup on success, error metadata preservation on error.
- Scope control / rollout strategy: Changes are localized to coordinator/session_manager. No schema change needed — metadata is a `map[string]string`. Backward compatible.
- Verification evidence:
  ```
  === Session Metadata (after fix) ===
    last_error: think step failed: LLM call failed: [openai/kimi-for-coding] Authentication failed...
    last_error_at: 2026-03-02T15:39:47Z
    stop_reason: error
  ```

## Prevention Actions
1. Action: Add error metadata assertions to coordinator acceptance tests
   Owner: ckl
   Due date: 2026-03-09
   Validation: Test that covers `executionErr != nil` path verifies `last_error` in session metadata
2. Action: Add session metadata dashboard/monitoring for `stop_reason=error` rate
   Owner: ckl
   Due date: 2026-03-16
   Validation: Alert when error rate exceeds threshold per hour

## Follow-ups
- Open risks: The `last_error` field stores the full error chain which may be verbose. Consider truncating to 500 chars in a future pass.
- Deferred items: Consider adding an error message to `result.Messages` as a system message so it appears in session conversation history replay (not just metadata).

## Metadata
- id: INC-2026-03-02-01
- tags: [session, error-handling, observability, coordinator]
- links:
  - error-experience: docs/error-experience/entries/ (TBD)
  - commit: fix(session): persist execution errors and stop_reason to session metadata
