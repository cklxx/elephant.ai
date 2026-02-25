# Plan: Lark code-task summaries every 3 minutes

## Goal
Update Lark background summaries for code tasks every 3 minutes.

## Steps
- [x] Inspect background progress listener and identify per-task update interval hooks.
- [x] Add regression coverage for code-task interval messaging.
- [x] Implement per-task interval override (codex/claude_code -> 3 minutes) and dynamic summary labels.
- [x] Run lint and full test suite.
- [x] Update plan status.

## Status
- Done. Code-agent background summaries update every 3 minutes with dynamic labels.
