# Plan: Fix session replay metadata + subagent tool display

## Context
- Session replay shows different events after refresh; likely subtask metadata is lost when persisting to history.
- Subagent tool rendering should be visually distinct from core agent tool cards.

## Plan
1. Preserve subtask wrapper metadata when persisting session history (Postgres store Append + AppendBatch).
2. Add regression coverage to assert persisted subtask metadata survives replay.
3. Update subagent tool rendering to use compact display distinct from core tool cards and adjust tests.
4. Run full lint + tests, then commit changes and log error experience.

## Progress
- 2026-01-27: Plan created.
