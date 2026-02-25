# Plan: Subagent Parallel Request Staggering

## Status: Completed
## Date: 2026-02-03

## Problem
Subagent parallel execution can trigger upstream LLM request rejections when many subtasks start at the same time. The current `subagent` tool takes a `maxWorkers` parameter in `NewSubAgent(...)` but does not enforce it, so default parallelism becomes `len(tasks)` and can burst requests.

## Plan
1. Confirm the current parallel scheduling behavior for `subagent` (default parallelism and job start timing).
2. Enforce `maxWorkers` as the default cap for parallelism when `max_parallel` is not set.
3. Add a small start stagger between dispatching parallel jobs to reduce synchronized bursts.
4. Add tests to ensure parallel execution never exceeds the configured worker cap.
5. Run full lint and tests.
6. Record an error-experience entry + summary, and update long-term memory if this becomes a durable rule.

## Progress
- [x] Confirmed `maxWorkers` was not enforced; defaulted to `len(tasks)`.
- [x] Enforced default parallelism cap via `maxWorkers`.
- [x] Added start stagger between job dispatches.
- [x] Added regression test covering the default cap.
- [x] Ran `./dev.sh lint` and `./dev.sh test`.
- [x] Recorded error experience entry + summary and updated long-term memory.
