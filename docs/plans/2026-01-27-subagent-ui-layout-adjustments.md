# Plan: Subagent UI layout adjustments

## Goal
Adjust subagent event layout and grouping to reflect concurrent subagent threads without collapsing.

## Context
- Subagent thread grouping is needed for parallel executions.
- UI should render threads in a vertical stack (per latest request) and keep grouping stable.

## Steps
1. Update subagent grouping keys to avoid collisions when subtask indexes are missing.
2. Render grouped subagent threads in a stacked container.
3. Update ConversationEventStream tests for new grouping wrapper.
4. Run full web lint + tests.

## Progress Log
- 2026-01-27: Grouped subagent threads by parent task id and rendered vertical stack.
- 2026-01-27: Updated test selector for subagent group wrapper.
- 2026-01-27: Collapsed subagent cards now clip to ~3 lines using fixed max height.
- 2026-01-27: When completed, inline token count is displayed in the header row before progress.
- 2026-01-27: Tightened header/body spacing and nested summary padding for cleaner collapsed layout.
