# Plan: Log chain + timing instrumentation (2026-01-27)

## Goal
- Make logid correlate server, LLM, and raw request logs and expose them in the dev conversation-debug panel.
- Add timing instrumentation so multi-iteration agent flow can be broken down by stage, LLM call, and tool execution.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Add log fetch utilities and dev API endpoint for logid-based retrieval (service/LLM/latency/request logs).
2. Ensure logid propagates across LLM clients and subagent execution.
3. Add LLM timing metadata for think/tool LLM calls and expose node durations for debug.
4. Update dev conversation-debug UI to query log traces by logid and render timing breakdown.
5. Run full lint + tests.
6. Commit changes (split into small commits).

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Added timing breakdown card in conversation-debug, debug-mode SSE coverage, and share handler debug flag fix.
- 2026-01-27: Fixed env usage guard for log_fetch; ran `./dev.sh lint` and `./dev.sh test` (pass; linker emitted LC_DYSYMTAB warnings).
