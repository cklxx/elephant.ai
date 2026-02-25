# Lark Model Pin Investigation Plan

## Goal
Identify why `/model use codex/gpt-5.3-codex` does not take effect in the same Lark chat, map command parsing + model pin persistence/resolution, and propose fixes with exact file+line suspects.

## Plan
- [x] Locate Lark `/model` command parsing and selection scope order.
- [x] Trace model pin persistence and resolver path.
- [x] Trace Lark session reuse and where pinned selections are applied to execution context.
- [x] Summarize likely root causes and concrete fixes.

## Notes
- Keep scope to Lark channel behavior and selection store/resolution.
