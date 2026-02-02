# Plan: Remove Memory Service Hooks (2026-02-02)

## Goal
Finish the markdown-memory refactor by removing legacy memory service hooks and wiring, updating DI/coordinator/dev endpoints/config types, and cleaning remaining references.

## Plan
1) Inventory remaining memory service hooks and wiring points (coordinator, preparation, context, DI, server/router, tests).
2) Remove memory service hook usage and update DI/container builder + coordinator dependencies accordingly.
3) Update server dev endpoints and config types to match the markdown memory engine.
4) Fix remaining references/tests and run lint + full tests.

## Progress
- 2026-02-02: Removed memory hook wiring (stale capture + compaction flush) and coordinator dependency; cleaned dev memory endpoint wiring and server deps; updated tests/tooling and fixed memory tool compile issues.
- 2026-02-02: `./dev.sh lint` and `./dev.sh test` passed.
