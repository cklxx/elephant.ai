# Plan: Streaming Event History P95 Optimization (2026-02-10)

## Status
- in_progress

## Goal
- Improve backend streaming-path P95 latency by reducing flush contention and backpressure amplification in async event-history persistence.
- Preserve business-critical history while allowing only debug/diagnostic persistence degradation under high backpressure.

## Scope
- `internal/delivery/server/app/async_event_history_store.go`
- `internal/delivery/server/bootstrap/config.go`
- `internal/delivery/server/bootstrap/server.go`
- `internal/delivery/server/bootstrap/config_log.go`
- `internal/shared/config/file_config.go`
- `internal/delivery/server/app/*_test.go`
- `internal/delivery/server/bootstrap/config_test.go`
- `configs/config.yaml`
- `docs/reference/CONFIG.md`

## Decisions
- Code-level optimization first.
- Only diagnostic/debug events may be degraded when queue backpressure crosses a configured watermark.
- Keep default behavior safe: degradation on by default only for debug events; critical/business events remain durable.

## Steps
- [x] Add failing tests for flush coalescing and debug-event backpressure degradation.
- [x] Implement async history flush request coalescing and bounded per-request drain.
- [x] Implement debug-event degradation policy and metrics in async history store.
- [x] Wire new tuning knobs through server config + bootstrap wiring.
- [x] Update config docs/examples.
- [x] Run targeted tests, then full lint + test.
- [x] Run mandatory code review workflow before commit.
- [ ] Commit incrementally, merge to `main`, cleanup worktree.

## Progress Log
- 2026-02-10: Created worktree `perf/streaming-p95-optimization-20260210`, copied `.env`, loaded engineering practices and memory context.
- 2026-02-10: Added new async-history tests for backpressure debug degradation + flush request coalescing.
- 2026-02-10: Implemented async-history enhancements: request coalescing, bounded flush-request drain, debug-event degradation, and new stats/option hooks.
- 2026-02-10: Wired new async-history knobs into YAML/server bootstrap and updated config docs/examples.
- 2026-02-10: Review found one P1 risk (coalesce window could add fixed latency to single flush); fixed by only waiting coalesce window when multiple flush requests are already pending.
- 2026-02-10: Re-ran full gates (`make dev-lint`, `make dev-test`) after fix; final run passed.
