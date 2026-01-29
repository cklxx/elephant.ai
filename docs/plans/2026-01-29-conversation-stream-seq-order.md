# Plan: Render conversation stream by seq order (2026-01-29)

- Reviewed `docs/guides/engineering-practices.md`.
- Memory load completed (latest error/good entries + summaries + long-term memory).

## Goals
- Render conversation events strictly by seq order (no custom reordering).
- Remove front-end subagent grouping; rely on subagent tool aggregation.

## Plan
1. Sort events by seq before rendering the stream.
2. Simplify timeline rendering to use the ordered event list.
3. Keep pending tool rendering intact.
4. Run full lint + tests.
5. Update plan progress and commit changes.

## Progress
- 2026-01-29: Plan created; engineering practices reviewed; starting implementation.
- 2026-01-29: Removed subagent grouping from ConversationEventStream; ordered events by seq before rendering; kept pending tool rendering intact.
- 2026-01-29: Ran `./dev.sh lint` and `./dev.sh test` (pass; linker warnings about LC_DYSYMTAB during Go tests).
