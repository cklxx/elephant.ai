# 2026-02-24 Kernel Reply Path Analysis

## Goal
Map the kernel request-to-reply generation flow, document the normal, tool, exception, timeout, and fallback surfaces, and point to the files/functions that implement each path so future debugging is easier.

## Scope
- Review kernel entry points in `internal/app/agent/kernel` and related delivery components (`internal/delivery`)
- Identify major code paths for standard replies, tool invocation, exception handling, timeout handling, and fallback/panic recovery
- Capture referenced files/functions for each category
- Record findings in this repo's documentation style with precise paths

## Steps
- [x] Sketch the high-level dispatch flow by reading kernel bootstrap and loop code to confirm the cycle boundaries.
- [x] Trace the message lifecycle (`RunCycle` → `Executor` → `AgentCoordinator` → ReAct) and note the core file/function references.
- [ ] Identify the code that handles tool calls, exceptions, timeouts, and fallback/write-restricted paths, then catalogue those references.
- [ ] Summarize the paths with function-level citations so the user can follow each branch.

## Progress
- 2026-02-24 15:00: Plan created.
- 2026-02-24 15:45: Kernel loop entry and executor/coordinator/reactract chain reviewed to prepare detailed path documentation.
