# Plan: Render conversation stream by seq order (2026-01-29)

- Reviewed `docs/guides/engineering-practices.md`.
- Memory load completed (latest error/good entries + summaries + long-term memory).

## Goals
- Render conversation events strictly by seq order (no custom reordering).
- Render subagent cards anchored to subagent tool start events.
- Ensure tool started events render before completions.

## Plan
1. Sort events by seq before rendering the stream.
2. Add subagent card aggregation anchored at subagent tool start events.
3. Ensure tool started events render in main stream before completions.
4. Run full lint + tests.
5. Update plan progress and commit changes.

## Progress
- 2026-01-29: Plan created; engineering practices reviewed; starting implementation.
- 2026-01-29: Ordered events by seq; restored subagent cards anchored at subagent tool start; ensured tool started events render; skipped duplicate subagent tool completed output.
- 2026-01-29: Ran `./dev.sh lint` and `./dev.sh test` (pass; linker warnings about LC_DYSYMTAB during Go tests).
