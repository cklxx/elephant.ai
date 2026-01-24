# Plan: Performance Optimization Execution (2026-01-24)

## Goal
- Resolve all validated performance findings (P0/P1/P2) and earlier P0 list items, excluding Redis tool-result cache (explicitly deferred).
- Ship backend safeguards, web performance improvements, CI coverage, and documentation updates.

## Plan
1. Backend safety & DB
   - Configure pgxpool settings with sane defaults + YAML config.
   - Add query safety (io.LimitReader) and migration timeouts.
   - Add memory search trigram index when pg_trgm is available.
2. HTTP/SSE reliability
   - Add HTTP rate limiting middleware.
   - Cap per-connection SSE caches and reduce serialization overhead.
   - Add gzip response compression for non-stream routes.
   - Add non-stream request timeouts to offset WriteTimeout=0.
3. Event pipeline & concurrency
   - Refactor EventBroadcaster to avoid holding locks during sends.
   - Finalize tool concurrency limiter and circuit breaker integration (already in progress).
4. LLM streaming memory
   - Reduce scanner memory usage and remove expensive JSON pretty-printing.
5. Web UI performance
   - Remove unused dependencies.
   - Memoize heavy components; reduce prop drilling.
   - Replace O(n) SSE final-event squash with indexed updates.
   - Cache Intl formatters and tool icon map.
   - Consolidate/optimize logo assets.
6. CI/Test coverage
   - Add web lint/test/build/perf jobs to CI.
   - Add missing tool tests (file_edit, list_files, memory_recall).
7. Docs
   - Update CONFIG docs and example YAML.
   - Reorganize README with clearer progression.
   - Update plan progress as work completes.

## Progress
- 2026-01-24: Plan created; engineering practices reviewed.
- 2026-01-24: Fixed session list pagination signature across stores/call sites; updated SSE attachment caches to LRU; resolved HTTP rate limiter naming conflict; migrations list pagination and SSE tests adjusted; go test ./... passed.
