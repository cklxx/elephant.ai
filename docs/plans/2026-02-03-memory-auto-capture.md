# Plan: Auto Memory Capture + Flat Per-User Memory Layout (2026-02-03)

Owner: cklxx

## Goal
- Use `~/.alex/memory/<user-id>/...` for per-user Markdown memory (no `users/` layer).
- Auto-capture memory after each successful task (LLM summary + fallback), written to daily logs.
- Migrate legacy `~/.alex/memory/users/<user-id>/` data without loss.

## Scope
- Memory path/layout changes and legacy migration.
- New `MemoryCaptureHook` (post-task auto write).
- Docs + tests + cleanup.

## Non-Goals
- Auto promotion into `MEMORY.md`.
- Remote memory backends.

## Plan of Work
1) Add plan + review practices. ✅
2) Update memory layout + indexer and implement legacy migration.
3) Add auto memory capture hook + DI wiring.
4) Update docs + tests; clean repo artifacts.
5) Run full lint/tests.

## Progress
- 2026-02-03: Plan created.
- 2026-02-03: Implemented flat per-user memory layout + legacy migration.
- 2026-02-03: Added MemoryCaptureHook and DI wiring.
- 2026-02-03: Updated docs, tests, and gitignore.
