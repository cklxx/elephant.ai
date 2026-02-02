# Lark Gateway Session History + Memory Restore Plan

## Goal
Restore Lark gateway session history usage so prior chat context and memory are available during task execution.

## Scope
- Update Lark gateway context handling to allow session history persistence/injection.
- Adjust tests that currently expect history to be disabled.
- Verify behavior with lint + tests.

## Plan
1. Update tests to reflect session history enabled for Lark context.
2. Fix gateway context setup to stop disabling session history.
3. Run full lint + tests to validate changes.

## Progress
- 2026-02-02: Plan created.
- 2026-02-02: Updated Lark gateway tests to expect session history enabled.
- 2026-02-02: Removed Lark session history disablement in gateway context.
- 2026-02-02: Ran `./dev.sh lint` and `./dev.sh test` (failed: pre-existing typecheck errors in agent ports/container code).
