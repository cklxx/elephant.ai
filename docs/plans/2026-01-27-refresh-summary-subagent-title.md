# Plan: Fix refresh missing summary + subagent title

## Context
- Refreshing the conversation view drops the main agent final summary and removes subagent titles.
- Target: keep post-refresh rendering consistent with live stream, preserve core summary display, and keep subagent titles even when preview metadata is missing or only present in payload.

## Plan
1. Reproduce in code via tests: add/adjust unit tests for summary dedupe behavior and subagent preview fallbacks.
2. Implement fixes:
   - Keep core summary events even when they duplicate final answers.
   - Add resilient preview/content fallbacks from payload/task fields.
3. Run full lint + test suite and update plan progress.
4. Commit changes with focused messages.

## Progress
- 2026-01-27: Updated summary dedupe rules and added subagent preview fallback + tests.
- 2026-01-27: Ran `make fmt`, `make test`, `cd web && npm run lint`, `cd web && npm test`.
