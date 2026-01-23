# Web Frontend Architecture Refactor

**Goal:** Implement the multi-phase refactor plan for the web frontend (hooks split, event matching, component decomposition, markdown/attachment consolidation, state + types cleanups) with test coverage and CI parity.

## Plan
1) Baseline & inventory: verify existing refactors (useSSE split) and locate remaining targets (ConversationPageContent, TaskCompleteCard, markdown renderers, attachments, types).
2) Phase 1-2 refactors: centralize event matching + decompose ConversationPageContent and TaskCompleteCard; add unit tests for extracted hooks/segments.
3) Phase 3 consolidation: unify markdown renderer + attachments modules; update imports and tests.
4) Phase 4-5 hygiene: state management guidance + useAgentStreamStore reducer extraction + type safety cleanup + types split.
5) Validation: run lint/tests, update docs, and document any follow-up work.

## Progress Log
- 2026-01-23: Plan created and inventory started.
- 2026-01-23: Split `useSSE` into modular hooks + added event matching helper (`web/lib/events/matching.ts`); updated key consumers.
- 2026-01-23: Decomposed `ConversationPageContent` into focused hooks/components; extracted TaskCompleteCard into subcomponents + hooks.
- 2026-01-23: Consolidated markdown renderer under `components/ui/markdown/` and split attachments into `lib/attachments/`.
- 2026-01-23: Extracted agent stream reducer + types; updated `useAgentStreamStore` to use reducer + shallow selectors.
- 2026-01-23: Split `web/lib/types.ts` into `web/lib/types/` (api/events/ui) and removed `eventMatches` in favor of `isEventType`.
- 2026-01-23: Added state management guide under `web/docs/STATE_MANAGEMENT_GUIDE.md`.
- 2026-01-23: Web test run (`npm --prefix web test`) hung in `next build` after ~8 minutes; process terminated and logged in error experience.
