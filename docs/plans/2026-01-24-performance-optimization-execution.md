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
8. API hygiene
   - Remove unused parameters from internal helpers (not interface-bound).
   - Delete no-op hooks that add dead parameters and update callers/tests.

## Progress
- 2026-01-24: Plan created; engineering practices reviewed.
- 2026-01-24: Fixed session list pagination signature across stores/call sites; updated SSE attachment caches to LRU; resolved HTTP rate limiter naming conflict; migrations list pagination and SSE tests adjusted; go test ./... passed.
- 2026-01-24: Reviewed engineering practices for next batch; starting API hygiene + next perf items.
- 2026-01-24: Added LLM response size caps, reduced stream scanner buffer, removed pretty JSON logging, and fixed MCP restart stop channel.
- 2026-01-24: Added non-stream migration timeout, API list caps, cached Intl/tool icons, SSE final-event indexing, and streaming markdown deferral.
- 2026-01-24: Memoized ConversationMainArea + ArtifactPreviewCard, removed react-syntax-highlighter + duplicate logo, and added missing tool tests.
- 2026-01-24: Removed event broadcaster hot-path locks via copy-on-write, added event history retention pruning, session cache, shared web_fetch client, pgx statement cache, web CI + bundle analyzer, and memoized ConversationHeader/QuickPromptButtons.
