# Plan: SSE event stream filtering (2026-01-31)

## Goal
Reduce SSE events to only those needed for UI and session rebuild/replay, while keeping debug-only streams available behind `debug=1`.

## Notes
- `claude -p` failed due to rate limit (reset at 1am Asia/Shanghai).

## Plan
1. Audit frontend SSE consumption vs server allowlist; define core vs debug-only event sets.
2. Add/adjust SSE handler tests to enforce filtering for non-debug and allow debug-only events with `debug=1`.
3. Update SSE handler allowlist + gating logic accordingly.
4. Run full lint + tests; update plan status and any required docs/memory.

## Progress
- [x] Audit event usage + define core/debug sets.
- [x] Add/adjust SSE handler tests.
- [x] Update SSE allowlist/gating logic.
- [x] Run lint + tests, update docs.
