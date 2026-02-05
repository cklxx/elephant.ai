# Plan: MCP ProcessManager stderr tail CI fix

## Status: In Progress
## Date: 2026-02-05

## Problem
CI intermittently fails `TestProcessManager_StderrTailCapturesOutput` because `pm.waitDone` can be signaled before the stderr monitor goroutine drains the pipe and writes to the tail buffer.

## Plan
1. Confirm the race between `monitorExit` and `monitorStderr`.
2. Ensure `waitDone` is delivered only after stderr draining completes for the process.
3. Add/adjust tests to prevent regressions.
4. Run full lint + tests.

## Progress
- [x] Create worktree and plan.
- [x] Confirm root cause and desired ordering guarantees.
- [x] Implement fix and keep regression coverage (existing test).
- [x] Run lint + tests.
- [ ] Merge back to `main` and cleanup worktree.
