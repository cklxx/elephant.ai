# Plan: Stabilize session replay by compacting event history storage (2026-01-28)

## Context
- Refreshing a session shows missing/partial final summaries and subagent titles.
- Likely caused by event history truncation or dropped events under high-volume streaming.
- Reduce history write volume while preserving critical replay events.

## Plan
1. Add tests that assert high-volume streaming events are skipped while final completion and subflow metadata remain persisted.
2. Update event history persistence filter to drop streaming deltas/progress and in-flight final chunks, keeping only terminal results and summary events.
3. Validate replay path still includes final answers and subagent previews; adjust tests as needed.
4. Run full lint + tests.

## Progress
- 2026-01-28: Plan created.
- 2026-01-28: Added history persistence filter to drop streaming deltas/progress and in-flight final chunks; added regression tests.
