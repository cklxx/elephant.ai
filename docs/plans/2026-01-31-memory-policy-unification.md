# Plan: MemoryPolicy Unification + Iteration Refresh + Event Ordering

Date: 2026-01-31
Owner: Codex

## Goals
- Make MemoryPolicy the single behavior switch for recall/capture/refresh.
- Remove memory refresh logic from domain; delegate to app-layer iteration hook.
- Guarantee event listener thread safety with per-run serialization.

## Decisions
- Memory behavior is controlled by MemoryPolicy only; config provides storage/retention/tuning defaults.
- Event ordering: per-run serial queue, cross-run async.
- Refresh injection uses system role + proactive source.

## Steps
1) Add/extend MemoryPolicy fields (refresh enabled/interval/max tokens) and update channel defaults.
2) Remove default userID fallbacks in memory hooks and update tests.
3) Add IterationHook interface; wire ReactEngine to call it; implement app-layer refresh hook.
4) Remove domain refresh logic; emit proactive refresh events based on hook results.
5) Add serializing event listener wrapper and wire into coordinator.
6) Update DI wiring for memory hooks/iteration hook; ensure config gating removed for memory behavior.
7) Run full lint + tests.

## Status
- [ ] In progress
