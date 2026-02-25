# 2026-02-12 — Kernel single-cycle real E2E command + goal alignment verification

Impact: Added a deterministic real E2E trigger (`alex-server kernel-once`) so kernel validation no longer depends on cron timing, and verified runtime goal alignment injection end-to-end.

## What changed

- Added `alex-server kernel-once` command path:
  - bootstraps full runtime dependencies (real config/DB/LLM/toolchain)
  - executes exactly one `KernelEngine.RunCycle()`
  - prints cycle summary (`cycle_id/status/dispatched/succeeded/failed`)
  - returns non-zero when cycle status is not `success`
- Added command dispatch tests in `cmd/alex-server/main_test.go`.
- Documented one-shot real E2E workflow in `docs/kernel-prompt-state-flow.md`.
- Updated `~/.alex/kernel/default/GOAL.md` objective and re-ran real cycles to confirm the objective appears in `SYSTEM_PROMPT.md` kernel alignment section.

## Why this worked

- Reuses production startup path (`BootstrapFoundation`) instead of mock wiring, so behavior is representative.
- Decouples verification cadence from scheduler cadence; reduces waiting and false negatives from timing.
- Uses kernel-native artifacts (`STATE.md` + `SYSTEM_PROMPT.md`) as source of truth for runtime validation.

## Validation

- `go test ./cmd/alex-server ./internal/delivery/server/bootstrap` ✅
- Real runs:
  - `go run ./cmd/alex-server kernel-once` → `run-8cy7ObfYf5Pb` (success)
  - `go run ./cmd/alex-server kernel-once` → `run-9QVzWyxNBJzy` (success)
- Runtime artifacts verified:
  - `~/.alex/kernel/default/STATE.md` updated with latest `kernel_runtime`
  - `~/.alex/kernel/default/SYSTEM_PROMPT.md` includes updated kernel goal lines
