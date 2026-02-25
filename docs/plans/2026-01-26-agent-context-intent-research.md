# Plan: Strengthen system prompt for intent recognition + research depth (2026-01-26)

## Goal
- Encourage the model to identify user intent and expected depth, and to treat research-heavy requests (e.g., stock research) as complex, multi-round investigations with structured reporting.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Locate the system prompt sources (default + preset suffix) that influence all agents.
2. Add concise intent + research-depth guidance without bloating prompts.
3. Run full lint + tests.
4. Commit changes; document any validation failures.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Added intent + research-depth objectives to `configs/context/goals/default.yaml`.
- 2026-01-26: Ran `./dev.sh lint` and `./dev.sh test` (both pass).
