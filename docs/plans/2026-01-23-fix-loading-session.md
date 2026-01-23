# Plan: Fix conversation "Loading session" regression

## Context
- UI stuck at "Loading session…" after refactor.
- Compare refactored hooks with pre-refactor behavior to locate missing connection trigger.

## Steps
1. Compare old `useSSE` session-change behavior with current refactor to find regressions.
2. Restore connection trigger on session changes (and ensure reconnection state resets).
3. Adjust AGENTS header wording per request.
4. Run lint + tests; record any failures.

## Progress
- [x] Compare prior `useSSE` logic (commit `398b20f1^`) to new refactor and identify missing session-change connect.
- [x] Restore session-change connect trigger in `web/hooks/useSSE/useSSE.ts`.
- [x] Update `AGENTS.md` title for clarity.
- [x] Run lint + tests and capture results.
