# Plan: Full project scan for optimization opportunities (2026-01-24)

## Goal
- Perform a repo-wide scan and report optimization opportunities suited for top-tier performance requirements, with concrete file references.

## Plan
1. Inventory critical subsystems (agent loop, storage/session, observability, web streaming, evaluation harness) and map high-traffic code paths.
2. Run targeted searches for perf signals (TODO/perf, unbounded loops, heavy JSON work, file I/O, cache usage, concurrency patterns).
3. Inspect representative files in each subsystem and collect concrete optimization findings with references.
4. Summarize findings ordered by severity and expected performance impact; call out measurement gaps and recommended profiling.

## Progress
- 2026-01-24: Read engineering practices; plan created.
- 2026-01-24: Completed repo scan via targeted searches and inspection of streaming, storage, RAG, and web attachment paths; prepared performance findings with file references.
- 2026-01-24: Deep scan phase 2 - launched 4 parallel agents for Go backend, web frontend, tests/CI, and error handling analysis.
- 2026-01-24: Identified 40+ optimization opportunities across all layers:
  - Go: Regex recompilation (parser.go, react_engine.go), slice pre-allocation, mutex contention in EventBroadcaster
  - Web: SSE event O(n) processing, markdown re-parse on delta, prop drilling in ConversationMainArea (30+ props)
  - Tests: 30 test files for 57 builtin source files, missing multi-agent orchestration tests
  - Findings documented in `docs/error-experience/entries/2026-01-24-performance-scan-findings.md`
- 2026-01-24: Implemented P0 optimizations:
  - Go: Moved regex to module-level variables in parser.go (3 patterns) and react_engine.go (5 patterns)
  - Go: Pre-allocated slice in postgres_event_history_store.go:365 with batchSize capacity
  - Go: Replaced slice prepend with slices.Insert in react_engine.go:1056-1070
  - Web: Added React.memo with custom comparator to ToolCallCard.tsx (20-30% re-render reduction)
  - Web: Optimized squashFinalEvents in useSSE.ts with early return for common case (no duplicates)
- 2026-01-24: Deep scan phase 3 - launched 6 parallel agents for infrastructure-level analysis:
  - DB Queries & Pooling: Found pgxpool using defaults (4 conn), no prepared statements, ILIKE without GIN index
  - Memory & GC: 2MB scanner buffers per stream, unbounded event history, io.ReadAll without limits
  - Caching: Good existing caches (LRU embedder, DataCache); gaps in tool icon maps, Intl formatters
  - Concurrency: WriteTimeout=0 risk for non-stream routes, SSE handler caches unbounded per connection, startup migration no timeout
  - API Serialization: Per-event json.Marshal, no gzip compression, pagination without upper bound
  - Web Bundle: Unused react-syntax-highlighter, only 2 memoized components in web UI
- 2026-01-24: Validated findings, corrected file paths/counts, and updated summary tables.
- 2026-01-24: Implemented additional fixes:
  - Backend: pgxpool tuning, query timeouts, LLM response size caps, reduced stream scanner buffers, migration timeout, MCP restart safety, API limit caps.
  - HTTP/SSE: gzip compression for non-stream responses, SSE per-connection LRU caches, non-stream timeout guard.
  - Web: SSE final-event indexing, streaming markdown deferral, memoization of ConversationMainArea/ArtifactPreviewCard, cached Intl/tool icons, removed react-syntax-highlighter + duplicate logo asset.
  - Tests: added file_edit/list_files/memory_recall tool tests.

## Summary of All Findings

### Completed Optimizations (P0/P1)
| Optimization | File | Impact |
|-------------|------|--------|
| pgxpool tuning + health checks | internal/di/container.go | Connection stability |
| LLM response size caps + scanner buffer trim | internal/llm/* | OOM prevention, lower RSS |
| SSE LRU caches + non-stream timeout guard | internal/server/http/sse_handler.go, middleware | Memory stability, slow-client protection |
| Migration timeout | internal/server/bootstrap/server.go | Startup reliability |
| API pagination caps | internal/server/http/api_handler.go | Prevent unbounded loads |
| Gzip middleware | internal/server/http/middleware.go | Bandwidth reduction |
| SSE final-event indexing | web/hooks/useSSE/useSSE.ts | Reduced O(n) scans |
| Streaming markdown deferral | web/components/ui/markdown/StreamingMarkdownRenderer.tsx | Fewer re-parses |
| Memoize heavy components | ConversationMainArea.tsx, ArtifactPreviewCard.tsx | Lower re-renders |
| Cache Intl + tool icons | web/lib/utils.ts | Lower CPU |
| Remove react-syntax-highlighter + duplicate logo | web/package.json, web/public | Smaller bundle/assets |
| Tool coverage gaps | internal/tools/builtin/*_test.go | Added tests |

### Remaining High-Priority Items
| Issue | File | Impact |
|-------|------|--------|
| Event broadcaster lock strategy | internal/server/app/event_broadcaster.go | Contention avoidance |
| Event history retention policy | postgres_event_history_store.go | DB growth control |
| Session LRU cache | internal/session/postgresstore/store.go | DB load reduction |
| Shared http.Client for web_fetch | internal/tools/builtin/web_fetch.go | Connection reuse |
| Prepared statements for hot paths | DB stores | Query CPU |
| Bundle analyzer + web CI jobs | .github/workflows/ci.yml | Regression visibility |
| Memoization for remaining web hotspots | web/components/* | Re-render reduction |

### Architecture Observations
- Good: Virtual scrolling, event buffering, LRU caches, TypeScript strict mode.
- Remaining: EventBroadcaster lock strategy, long-term event retention, and web CI visibility.
