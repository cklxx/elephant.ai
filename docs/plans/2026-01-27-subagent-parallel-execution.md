# Plan: Enable parallel subagent execution and task arrays

## Goal
Allow the subagent tool to accept multiple tasks and run them in parallel with bounded concurrency.

## Context
- Users want multiple subagents to start concurrently.
- Existing subagent tool only accepted a single prompt and executed serially.
- Explore tool should forward task arrays instead of a single prompt.

## Steps
1. Extend subagent tool schema to accept `tasks`, `mode`, and `max_parallel`.
2. Add parsing helpers and concurrency control for parallel execution.
3. Update explore tool to pass tasks list and mode.
4. Add/adjust tests for new argument shape.
5. Run full lint + tests.

## Progress Log
- 2026-01-27: Implemented task array parsing, mode handling, and parallel execution worker pool.
- 2026-01-27: Updated explore tool to pass tasks array; updated tests accordingly.
