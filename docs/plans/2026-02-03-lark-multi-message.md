# Plan: Lark multi-message responsiveness

## Goal
Ensure Lark responds to every user message, including messages sent while a task is running.

## Steps
- [x] Inspect Lark gateway message flow and identify why queued messages are dropped.
- [x] Add a regression test covering reprocessing of in-flight messages.
- [x] Fix Lark gateway reprocessing to bypass dedup for drained messages.
- [x] Run lint and full test suite.
- [x] Update plan status and summary.

## Status
- Done. Lark reprocessing bypasses dedup for drained messages, with regression coverage.
