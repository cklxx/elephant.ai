# Plan: Stream subagent tool start/complete events to frontend

Date: 2026-01-28
Owner: Codex

## Goal
Ensure subagent tool `workflow.tool.started` and `workflow.tool.completed` events reach the frontend with `tool_name: subagent` and task identifiers for association.

## Plan
1. Inspect SSE streaming filters and subagent tool event envelope generation.
2. Add tests that assert subagent tool started/completed events are streamed with task identifiers.
3. Adjust SSE streaming filter to allow subagent tool started/completed (keep other filters intact).
4. Run full lint + test suite and fix any regressions.

## Progress
- 2026-01-28: Plan created.
- 2026-01-28: Added SSE streaming test coverage for subagent tool start/complete events.
- 2026-01-28: Updated SSE streaming filter to allow subagent tool start/complete events through.
- 2026-01-28: Ran `./dev.sh lint` and `./dev.sh test` (Go tests passed; linker warnings observed).
