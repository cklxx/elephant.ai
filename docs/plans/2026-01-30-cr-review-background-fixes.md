# Plan: CR review fixes for background tasks + UI chevron (2026-01-30)

## Goal
- Address CR review findings: background task duration correctness, dispatch/completion event emission, and ToolOutputCard chevron consistency.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Fix background task duration for pending/running tasks and add unit coverage.
2. Emit `background.task.dispatched` events via a dispatcher wrapper and add tests.
3. Ensure background completion events are emitted during cleanup (dedupe by task ID) with tests.
4. Restore ToolOutputCard chevron affordance.
5. Run full lint + tests.
6. Commit in small, focused commits.

## Progress
- 2026-01-30: Plan created; engineering practices reviewed.
- 2026-01-30: Fixed background task duration for pending tasks; updated unit coverage.
- 2026-01-30: Added background dispatch/completion event emission with dedupe; added runtime tests.
- 2026-01-30: Restored ToolOutputCard chevron affordance.
- 2026-01-30: `./dev.sh lint` failed (errcheck in hooks tests); logged error experience entry.
- 2026-01-30: `./dev.sh test` passed (linker warnings about malformed LC_DYSYMTAB).
