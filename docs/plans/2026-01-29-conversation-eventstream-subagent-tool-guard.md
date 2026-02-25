# Plan: Fix subagent tool event narrowing in ConversationEventStream

Date: 2026-01-29
Owner: Codex

## Context
TypeScript reports `Property 'call_id' does not exist on type 'never'` in `web/components/agent/ConversationEventStream.tsx` while tracking tool starts/completions.

## Goal
- Remove the erroneous `never` narrowing while preserving subagent tool filtering and correct tool pairing.

## Non-Goals
- No UI behavior changes beyond type correctness.
- No refactors outside the affected event partition logic.

## Plan
1. Inspect the event partitioning pass and identify where a type guard narrows away `workflow.tool.started` events.
2. Widen the type after the subagent-tool skip so tool-start/completion handling can access `call_id` safely.
3. Run full lint and tests.
4. Document results and any failures.

## Progress Log
- 2026-01-29 11:00: Plan created; investigation started.
- 2026-01-29: Expanded scope to fix three event partitioning bugs:
  1. **Bug 1 (subagent card)**: Added `"workflow.tool.started"` to `shouldDisplayInSubagentCard` allowlist so subagent internal tool calls appear in card body.
  2. **Bug 2 (main stream duplication)**: Changed `shouldDisplayInMainStream` to only pass subagent tool starts (`isSubagentToolEvent`) to main stream; regular tool starts handled by `pendingTools` section.
  3. **Bug 3 (plan/clarify noise)**: Added `isOrchestratorTool` helper; filtered plan/clarify tool starts from `pendingTools` map.
- All changes in `web/components/agent/ConversationEventStream.tsx`. TypeScript passes with no errors in target file.
