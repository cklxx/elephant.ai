# Plan: Lark Break Optimization (State Machine + Codex PID Control) — 2026-02-10

## Status: Completed

## Goal
- Eliminate long-lived `test` suppression caused by stale validation phases.
- Add timeout-based fallback restart for `test` during validation suppression.
- Track and control Codex subprocess PIDs (loop autofix + supervisor/autofix paths) so `./lark.sh down` can reliably kill them.
- Keep behavior observable via supervisor status/doctor and script regressions.

## Batch 1: Baseline + TDD Harness
- [x] Sync worktree branch to latest `main`.
- [x] Confirm current suppression path (`skip test restart during validation`) and stale phase symptoms.
- [x] Extend script smoke tests first:
  - [x] validation suppression timeout fallback restart
  - [x] stale loop state recovery to idle
  - [x] codex pid cleanup on supervisor stop
  - [x] loop codex pid lifecycle regression
  - [x] autofix codex pid lifecycle regression

## Batch 2: Supervisor State-Machine Refactor
- [x] Add stale-loop-state reconciliation (`validation phase` + loop down + timeout => reset idle).
- [x] Add validation suppression timeout control for `test` restart fallback.
- [x] Extend status output JSON with suppression/recovery/codex-pid observability.
- [x] Ensure `stop` path kills tracked codex processes and clears pid files.

## Batch 3: Codex PID Lifecycle Wiring
- [x] `scripts/lark/loop.sh`: record codex/claude autofix subprocess pid and cleanup reliably.
- [x] `scripts/lark/autofix.sh`: record codex subprocess pid while timeout wrapper runs; cleanup on exit/timeout.
- [x] `scripts/lark/supervisor.sh`: read/report codex pid files and enforce cleanup in stop flow.

## Batch 4: Validation + Review + Delivery
- [x] Run targeted script tests.
- [x] Run `./dev.sh lint`.
- [x] Run `./dev.sh test`.
- [x] Execute mandatory code review workflow and produce P0-P3 findings.
- [x] Commit in multiple incremental commits.
- [x] Merge branch back into `main` (fast-forward) and remove temporary worktree (branch retained due local policy restriction on deletion command).
