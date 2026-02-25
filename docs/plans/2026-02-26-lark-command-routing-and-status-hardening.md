# 2026-02-26 Lark Command Routing and Status Hardening

## Context
- User reports Lark replies are incorrect and `/new` has no practical effect.
- Evidence from `logs/alex-service.log` shows messages received while `slotRunning` were logged as `Injected user input into active session ...`, meaning command intent was swallowed by in-flight injection path.
- User also requested improving status visibility (kernel run counts) and adding an explicit Lark restart command path.

## Goals
1. Fix command routing so `/new` is handled as a command even while a task is running.
2. Prevent stale in-flight cleanup from clobbering slot state after command-based session switch.
3. Extend `alex dev lark status` output with kernel run count summary.
4. Add an explicit top-level `./dev.sh lark-restart` alias.

## Plan
- [completed] Add regression test: `/new` during running task must not be injected as user input.
- [completed] Update gateway/task manager routing and slot finalization logic.
- [completed] Add kernel run stats parsing + status display.
- [completed] Add `lark-restart` shell alias.
- [completed] Run targeted tests + command-level verification (`lark status`, `lark inject /new`).

## Verification Notes
- Unit and package tests passed:
  - `go test ./internal/delivery/channels/lark -count=1`
  - `go test ./internal/devops/supervisor -count=1`
  - `go test ./cmd/alex -run 'TestFormatLarkComponentStatusKernelIncludesRuns|TestFormatLarkComponentStatusNonKernelOmitsRuns|TestWaitForSupervisorPIDPublication|TestReadLivePIDFile' -count=1`
- Status output now includes kernel runs: `kernel ... runs=0` (or higher on restart events).
- Added and validated `./dev.sh lark-restart` alias (deprecated compatibility wrapper to `./dev.sh lark restart`).
- Important runtime nuance observed: `./dev.sh lark restart` restarts the supervisor process but does not force-restart `main/kernel/loop` component processes if they are already healthy.
  - To load new gateway code in-process for verification, component restart was required via `scripts/lark/main.sh restart`.
- Post-restart inject evidence confirms `/new` command path cancels active task and opens a new session instead of slot injection:
  - no `Injected user input into active session` log for the `/new` message in chat `inject-new-routing-after-main-1772036117`.
