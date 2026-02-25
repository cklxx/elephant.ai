# 2026-02-12 Kernel Real-Action Guard + Init Doc Idempotency

## Goal
- Eliminate kernel "fake success" cycles where the dispatch returns success without any real tool execution.
- Keep `INIT.md` as a first-boot snapshot (do not overwrite on every process restart).
- Verify with real kernel runtime execution (no mocks) that state artifacts and cycle status behave as expected.

## Scope
- `internal/app/agent/kernel/executor.go`
- `internal/app/agent/kernel/executor_test.go`
- `internal/app/di/container_builder.go`
- Runtime validation against `~/.alex/kernel/default/{STATE.md,INIT.md,SYSTEM_PROMPT.md}`

## Implementation Plan
1. Add a kernel dispatch post-condition: at least one non-orchestration tool call must exist in `TaskResult`.
2. Add unit tests for success/failure paths of the new executor contract.
3. Switch kernel init doc write path from overwrite semantics to seed-only semantics.
4. Run full lint + tests.
5. Run real kernel e2e cycle and verify:
   - real action guard behavior;
   - `STATE.md` runtime section update;
   - `INIT.md` remains immutable after first seed;
   - `SYSTEM_PROMPT.md` remains visible and refreshed.

## Progress
- [x] Design and code updates for executor guard.
- [x] Unit tests for kernel executor guard.
- [x] Seed-only init doc wiring in DI.
- [x] Full lint + tests.
- [x] Real kernel e2e validation.
- [x] Code review completed (no blocking findings).
- [ ] Commit + merge back to `main`.
