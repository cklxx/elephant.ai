# 2026-02-26 Retire update_config Tool Path

## Goal
- Fully remove the `update_config` tool execution path so the model cannot switch runtime provider/model during a run.

## Plan
- [completed] Remove ReAct runtime config-override plumbing (`config_override` store usage, runtime injection, iteration apply hook).
- [completed] Remove engine/coordinator rebuilder wiring tied to update-config model switching.
- [completed] Remove shared/ports context helpers that only serve `update_config`.
- [completed] Delete obsolete tests and update remaining assertions/messages that mention `update_config`.
- [completed] Run focused tests for changed packages and ensure `rg update_config internal` has no runtime/tool references left.
- [completed] Run mandatory code review skill and commit only files for this task.
