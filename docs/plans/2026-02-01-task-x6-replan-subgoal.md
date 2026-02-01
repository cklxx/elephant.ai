# Plan: X6 replan trigger + sub-goal state machine (MVP)

Status: done

## Steps
- [x] Review existing ReAct runtime state (plan/clarify gates, plan review).
- [x] Add tests for sub-goal plan node updates and replan trigger on tool error.
- [x] Implement clarify-driven plan node updates + tool-error replan injection.
- [x] Run gofmt, lint/tests, and restart the dev stack.
- [x] Update this plan with completion details.

## Notes
- Clarify results now populate TaskState plan nodes and mark previous tasks completed.
- Tool errors mark the current task blocked and inject a replan prompt.
