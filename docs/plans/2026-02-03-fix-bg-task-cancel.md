# Plan: Prevent background task cancellation on run end (2026-02-03)

## Goal
Allow `bg_dispatch` tasks (especially external Codex/Claude) to keep running after the parent run ends, so they are not cancelled with `context canceled` errors, and can be collected later via `bg_status`/`bg_collect`.

## Plan
1. Inspect background task lifecycle and confirm cancellation path (runtime cleanup + manager shutdown).
2. Add a shared background-task manager per session and wire React runtime to reuse it without shutdown on run end.
3. Ensure dispatched tasks capture per-run event sinks/IDs to avoid cross-run leakage.
4. Add regression test(s) covering shared-manager cleanup behavior.
5. Run lint + tests.

## Progress
- [x] Inspect lifecycle + identify cancellation source
- [x] Implement shared manager + event-sink capture
- [x] Add/adjust tests
- [x] Run lint + tests
