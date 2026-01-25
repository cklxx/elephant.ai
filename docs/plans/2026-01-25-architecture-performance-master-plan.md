# Plan: Architecture + Performance Master Roadmap (2026-01-25)

## Goal
- Provide a unified, prioritized roadmap to address Elephant.AI architecture and performance issues with minimal risk and measurable wins.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Principles
- Small, reviewable changes; each change fully tested.
- Solve hot paths and user-visible latency first.
- Prefer refactors that reduce future complexity (not compatibility layers).
- Record incidents in error-experience entries when they occur.

## Phase 0 — Baseline + Guardrails (1-2 commits)
1. Add lightweight perf counters/logs for:
   - Session list endpoint (total time + per-session time).
   - Session title update latency.
   - JSON (de)serialization hotspots in session store.
2. Add a minimal profiling doc under `docs/operations/` describing how to capture pprof and endpoint timings.

## Phase 1 — User-visible Latency & N+1 Fixes (P0)
1. **Session list endpoint**
   - Add a `SessionSummary` path to avoid loading full sessions/messages in list views.
   - Avoid per-session `ListSessionTasks` scans; keep `task_count` + `last_task` in TaskStore summary.
   - API `/api/sessions` uses summary data only.
2. **Frontend cache sync**
   - Keep session label cache and list cache consistent (React Query cache update on plan event).

## Phase 2 — Hot Path JSON Optimizations (P0)
1. Replace high-frequency JSON hot paths with:
   - pgx JSONB scanning where possible.
   - or faster JSON implementation (jsoniter/sonic) where safe.
2. Guardrails:
   - Benchmarks for session store read/write.
   - Regression tests for JSON schema integrity.

## Phase 3 — Concurrency & Resource Safety (P1)
1. Lock granularity:
   - Replace `sync.Mutex` with `sync.RWMutex` in read-heavy hot paths.
   - Eliminate potential deadlock orderings.
2. Goroutine lifecycle:
   - Ensure all background goroutines have structured cancellation/timeouts.
   - Use shared panic recovery wrappers for consistency.

## Phase 4 — Cache Strategy & Memory (P1)
1. LLM client cache: introduce LRU + TTL + basic health checks.
2. Session memory pressure:
   - Reduce deep clones of messages where safe.
   - Consider COW or immutable slices for session messages.

## Phase 5 — Architecture Cleanups (P2)
1. Break down mega-files (>500 LOC) into domain-focused modules:
   - `api_handler.go`, `seedream.go`, `react_engine.go`, `execution_preparation_service.go`.
2. Ports restructuring by domain (`ports/llm`, `ports/tools`, `ports/storage`).
3. DI container refactor (builder or Wire).

## Phase 6 — Tool Execution Efficiency (P2)
1. Reuse worker pools for tool batch execution.
2. Add timeouts + priority scheduling for tool runs.

## Validation
- For each phase: `make fmt`, `make vet`, `make test`.
- For web changes: `npm run lint`, `npm test` in `web/`.
- Track perf deltas vs baseline per phase.

## Deliverables
- Each phase is a set of small commits with tests and measurable improvements.
- Update plan progress and log notable incidents in error-experience.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
