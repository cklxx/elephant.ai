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

## Summary of All Findings

### Completed Optimizations (P0)
| Optimization | File | Impact |
|-------------|------|--------|
| Regex to module-level | parser.go, react_engine.go | Reduce per-call regex overhead |
| Slice pre-allocation | postgres_event_history_store.go | Reduce allocations |
| slices.Insert for prepend | react_engine.go | Avoid extra slice copy |
| React.memo ToolCallCard | ToolCallCard.tsx | Reduce re-renders |
| squashFinalEvents early return | useSSE.ts | Reduce CPU in common case |

### Pending P0 Issues (Highest Priority)
| Issue | File | Risk |
|-------|------|------|
| Remove react-syntax-highlighter | web/package.json | Dependency bloat |
| Configure pgxpool | container.go | Connection stability |
| io.LimitReader | anthropic/openai_client.go | OOM risk |
| Cap SSE per-connection caches | sse_handler.go | Memory exhaustion |
| Migration timeout | server.go | Startup hang |

### Pending P1 Issues (High Priority)
| Issue | File | Impact |
|-------|------|--------|
| Reduce scanner max buffer / pool | openai_client.go | Lower RSS under load |
| Event broadcaster lock strategy | event_broadcaster.go | Contention avoidance |
| GIN trigram index for memory content | migrations | Memory search speed |
| Memo ArtifactPreviewCard | ArtifactPreviewCard.tsx | Re-render reduction |
| Gzip compression middleware | middleware | 70% bandwidth reduction |

### Architecture Observations
- Good: Virtual scrolling, event buffering, LRU caches, TypeScript strict mode
- Concern: 30+ prop drilling in ConversationMainArea, only 2 memoized components in web UI
- Concern: 4 separate mutexes in EventBroadcaster without documented lock order
- Concern: WriteTimeout=0 required for SSE but exposes non-stream routes to slow clients
