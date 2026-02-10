# Plan: Startup Memory Growth / OOM Investigation (2026-02-10)

## Objective
- Reproduce the startup-time continuous memory growth issue.
- Identify root cause with concrete evidence (allocation owner + growth path).
- Ship a minimal maintainable fix with regression tests.
- Validate with lint + full tests before merge.

## Active Memory (ranked)
1. Apply response-size caps + retention/backpressure to prevent unbounded growth.
2. SSE/event paths are hot; avoid unbounded in-memory accumulation.
3. Keep architecture boundaries clean (`agent/ports` no memory/RAG deps).
4. Prefer deterministic host-side probes and evidence before changing behavior.
5. TDD for logic changes; run full lint + tests before delivery.
6. Prior incidents: orphan/background processes can create hidden resource pressure.
7. Avoid defensive complexity when invariants are guaranteed.
8. Keep changes small and reversible.

## Scope
- In scope: startup/runtime memory growth in backend runtime and supporting components.
- Out of scope: broad performance optimization unrelated to continuous growth.

## Steps
- [ ] Reproduce and capture baseline memory growth curve.
- [ ] Narrow growth source (heap owners / long-lived structures / goroutines).
- [ ] Implement targeted fix + unit/integration tests.
- [ ] Run `alex dev lint` + `alex dev test`.
- [ ] Mandatory code review workflow + incremental commits.
- [ ] Merge back to `main` and clean up worktree.

## Progress Log
- 2026-02-10 23:32 CST: Initialized worktree `fix/startup-memory-oom-20260210`, copied `.env`, loaded engineering practices and recent memory summaries.
