# Scheduler X7 MVP (JobStore + Cooldown + Concurrency + Recovery)

Created: 2026-02-02
Owner: cklxx
Status: done

## Goals
- Integrate JobStore into scheduler start/recovery (file-based when configured).
- Enforce per-job cooldown + concurrency guard during execution.
- Add basic failure recovery with bounded retries and backoff.
- Expand tests to cover new behaviors.

## Plan
- [x] Update scheduler config schema + defaults + merge (job store path, cooldown, concurrency, recovery).
- [x] Implement JobStore integration + job payload mapping + execution guards + recovery timers.
- [x] Add scheduler tests for cooldown, concurrency, recovery, persistence.
- [x] Update YAML config example.
- [x] Run full lint/tests; restart dev (`./dev.sh down && ./dev.sh`).

## Progress Log
- 2026-02-02: plan created.
- 2026-02-02: schema + scheduler integration + tests + config example updated.
- 2026-02-02: schema + scheduler integration + tests + config example updated.
