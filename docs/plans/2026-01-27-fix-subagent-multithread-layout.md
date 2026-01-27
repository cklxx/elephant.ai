# Plan: Fix subagent multi-thread layout in web UI

## Goal
Ensure multiple subagent executions render side-by-side (as designed) instead of collapsing to a single visible panel.

## Context
- Affects web frontend subagent rendering.
- Must keep subagent aggregation behavior intact.
- Follow TDD and run full lint + tests.

## Steps
1. Inspect current subagent aggregation and layout in `web/components/agent/ConversationEventStream.tsx` and `AgentCard`.
2. Add/adjust tests to capture multi-subagent layout expectation (render multiple threads without collapsing).
3. Implement layout fix (likely container/grid/flex adjustments or keying/ordering logic).
4. Validate with unit tests + e2e, then run full lint + tests.
5. Update plan with progress and record any notable incidents.

## Progress Log
- 2026-01-27: Plan created.
- 2026-01-27: Problem identified - threads with empty events are being rendered as "idle" cards.
  - Initial hypothesis: Threads created but no displayable events added.
  - Solution attempted: Filter out threads with `events.length === 0`.
  - Result: Test failed - one test expects empty threads to be rendered.
- 2026-01-27: Re-analysis based on user feedback: **3 subagents sent but 4 displayed**.
  - **Root cause identified**: Events with `parent_task_id === task_id` were incorrectly identified as subagent events.
  - **Fix applied**: Modified `isSubagentLike` to check `parentTask && parentTask !== currentTaskId` (line 645 in EventLine/index.tsx).
  - **Additional debug logging**: Added logging in `getSubagentKey` and `combinedEntries` to trace key generation.
- 2026-01-27: New issue from user: **没有工具调用时 subagent 的详细信息不能展开的问题** (subagent details cannot be expanded when there are no tool calls).
  - Root cause: `CardFooter` only shows expand button when `eventCount > 1`.
  - When only 1 event exists, content is truncated with `line-clamp-3` but no expand button.
  - **Fix applied**: Modified `CardFooter` to show expand button for `eventCount >= 1`.
  - Button text changes: "Show full content" / "Collapse" for single event.
  - All tests pass ✅, lint passes ✅.
