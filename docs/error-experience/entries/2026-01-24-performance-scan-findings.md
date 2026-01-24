# 2026-01-24 - Performance Scan Findings (Top-Tier Standards)

## Error: Multiple performance optimization gaps identified across Go backend, web frontend, and test infrastructure

### Summary

Repo-wide scan identified ~40 optimization opportunities. This entry reflects validated findings and marks items already fixed in the current codebase.

---

## Go Backend Findings

### CRITICAL (P0 - Resolved in current codebase)

1. **Regex recompilation in hot paths**
   - `internal/parser/parser.go`, `internal/agent/domain/react_engine.go`
   - Status: ✅ moved patterns to module-level `regexp.MustCompile`.

2. **Slice copying in React engine**
   - `internal/agent/domain/react_engine.go`
   - Status: ✅ replaced prepend copy with `slices.Insert`.

3. **Uninitialized slice appends**
   - `internal/server/app/postgres_event_history_store.go`
   - Status: ✅ pre-allocated batch slice capacity.

### HIGH (P1)

4. **Event broadcaster lock contention risk**
   - `internal/server/app/event_broadcaster.go`
   - Multiple mutexes are used but not held simultaneously; no deadlock observed.
   - Risk: high event volume can contend on `mu`/`historyMu`/`taskMu`.
   - Fix: measure with pprof; simplify lock strategy if hot.

5. **JSON pretty-print logging on LLM responses**
   - `internal/llm/anthropic_client.go`, `internal/llm/openai_client.go`
   - `json.Indent()` runs on every request/response before logging.
   - Fix: gate pretty formatting behind log-level checks or make log level configurable.

6. **Per-user rate limiter map lock**
   - `internal/llm/user_rate_limit_client.go`
   - Limiter creation occurs under a single mutex; optimize if it becomes hot.

### MEDIUM (P2)

7. **web_fetch HTTP client per tool instance**
   - `internal/tools/builtin/web_fetch.go`
   - If tool instances are short-lived, consider a shared `http.Client` to reuse pools.

---

## Web Frontend Findings

### CRITICAL (P0)

1. **Prop drilling anti-pattern**
   - `web/app/conversation/components/ConversationMainArea.tsx` (30+ props)
   - Impact: cascading re-renders across component tree.
   - Fix: co-locate state or introduce context/hooks for grouped state.

2. **SSE event buffer O(n) processing**
   - `web/hooks/useSSE/useSSE.ts`
   - Current implementation has an early-return fast path but is still O(n) in worst case.
   - Fix: maintain per-task indexes to avoid re-scanning full arrays.

3. **Markdown re-parse on delta updates**
   - `web/components/ui/markdown/StreamingMarkdownRenderer.tsx`
   - Lazy renderer is invoked on streaming deltas; needs memoization or diffing to avoid reparse.

### HIGH (P1)

4. **Next.js image optimization disabled (by design for static export)**
   - `web/next.config.mjs` sets `output: "export"` and `images.unoptimized = true`.
   - Required for static export; only actionable if moving to server rendering.

5. **Missing React.memo on high-frequency components**
   - Only 2 memoized components found (EventLine, ToolCallCard).
   - Targets: `ArtifactPreviewCard.tsx`, `ConversationMainArea.tsx`, message list cells.

6. **Event state recomputation**
   - `web/hooks/useAgentStreamStore.ts`
   - `applyEventToDraft` runs per event; worst-case cost depends on reducer logic.
   - Fix: profile before assuming O(n^2); consider incremental aggregation caches.

### GOOD PATTERNS (Already Implemented)

- Virtual scrolling (`@tanstack/react-virtual`)
- Event deduplication (`useSSEDeduplication.ts`)
- LRU cache for events (MAX_EVENT_COUNT=1000)
- Event buffering with RAF scheduling
- TypeScript strict mode enabled

---

## Test Infrastructure Findings

### GAPS

1. **Tool test coverage (partial)**
   - 57 builtin source files vs 30 test files in `internal/tools/builtin`
   - Missing tests for: `file_edit`, `list_files`, `memory_recall` (memory_write covered)

2. **Integration test gaps**
   - No explicit multi-agent orchestration tests found
   - Missing SSE reconnection scenario tests
   - No stress/load tests for event streaming

3. **CI configuration**
   - `.github/workflows/ci.yml` runs Go lint/tests/builds only
   - Missing: web build/test jobs, bundle size checks

4. **Vitest coverage thresholds not enforced in CI**
   - Thresholds exist in `web/vitest.config.mts` but web tests are not run in CI

---

## Phase 3: Deep Infrastructure Scan (2026-01-24)

### Database & Connection Pooling

#### CRITICAL (P0)

1. **pgxpool using defaults (no explicit tuning)**
   - `internal/di/container.go` uses `pgxpool.New(ctx, dsn)`
   - Impact: default pool size (4 conns), no explicit health checks or lifetimes
   - Fix: configure `pgxpool.Config` (MaxConns, MinConns, MaxConnLifetime, etc.)

2. **Prepared statements not used in hot DB paths**
   - Session/memory/history stores use Exec/Query without prepare
   - Impact: parse/plan overhead per call; measure before adding.

#### HIGH (P1)

3. **Memory store fallback ILIKE without index**
   - `internal/memory/postgres_store.go` (content ILIKE '%...%')
   - Impact: full table scans on substring fallback
   - Fix: optional trigram GIN index on `content` if this path is hot.

4. **Session store List without pagination limit**
   - `internal/session/postgresstore/store.go:220`
   - Impact: OOM risk if session count grows
   - Fix: add default limit/cursor API.

5. **Async event store drops on full buffer**
   - `internal/server/app/async_event_history_store.go`
   - Impact: data loss under load; only warning log is emitted
   - Fix: metrics + backpressure option (block or persist).

---

### Memory & GC Patterns

#### HIGH (P1)

1. **Streaming scanner max buffer is 2MB**
   - `internal/llm/openai_client.go`
   - `bufio.Scanner` max token size set to 2MB; buffer grows when large chunks arrive.
   - Fix: reduce max or use pooled buffers if large payloads are common.

2. **Unbounded event history storage growth**
   - `internal/server/app/postgres_event_history_store.go`
   - Impact: DB storage grows indefinitely without retention policy
   - Fix: add TTL or archival job.

3. **io.ReadAll without size limits (OOM risk)**
   - `internal/llm/anthropic_client.go`, `internal/llm/openai_client.go`
   - Fix: wrap with `io.LimitReader` (e.g., 10MB).

4. **web_fetch cache cleanup goroutine lacks shutdown**
   - `internal/tools/builtin/web_fetch.go`
   - Acceptable if tool is singleton; problematic if tools are recreated.
   - Fix: pass context/stop channel or share cache across instances.

---

### Caching Strategies

#### GOOD PATTERNS (Already Implemented)

- LRU Embedder Cache (`internal/rag/embedder.go`)
- DataCache for agent state (`internal/server/http/data_cache.go`)
- WebFetch 15-minute cache with cleanup (`internal/tools/builtin/web_fetch.go`)
- Frontend event LRU (MAX_EVENT_COUNT=1000)

#### MEDIUM (P2)

1. **Tool icon map recreated per call**
   - `web/lib/utils.ts` (`getToolIcon`)
   - Fix: move map to module-level constant.

2. **Intl formatters not cached**
   - `web/lib/utils.ts` (formatRelativeTime creates new Intl.* each call)
   - Fix: cache formatter instances by locale.

3. **Session JSON parsing on every Get()**
   - `internal/session/postgresstore/store.go`
   - Fix: optional small LRU cache if session reads dominate.

---

### Concurrency & Goroutine Patterns

#### HIGH (P1)

1. **HTTP server WriteTimeout = 0**
   - `internal/server/bootstrap/server.go`
   - Required for SSE, but leaves non-streaming routes exposed to slow clients.
   - Fix: consider separate SSE server or per-route timeout guards; StreamGuard already limits SSE duration/bytes.

2. **SSE handler per-connection caches are unbounded**
   - `internal/server/http/sse_handler.go` (`sentAttachments`, `finalAnswerCache`)
   - Impact: long-lived sessions can grow memory unbounded.
   - Fix: cap with LRU or clear by size/age.

3. **Startup migration without timeout**
   - `internal/server/bootstrap/server.go` + session migration
   - Impact: startup can hang if DB unavailable.
   - Fix: wrap migration with context timeout.

4. **Event broadcaster lock order complexity**
   - `internal/server/app/event_broadcaster.go`
   - Multiple locks are used; no deadlock observed, but contention and ordering complexity are risks.
   - Fix: document lock order or consolidate under a single lock if needed.

5. **MCP process restart loses monitor goroutines**
   - `internal/mcp/process.go`
   - `stopChan` is closed on Stop() and never reinitialized; new monitor goroutines exit immediately after restart.
   - Fix: re-create stop channel on Start() or create a new ProcessManager per restart.

---

### API Serialization & Response

#### HIGH (P1)

1. **SSE per-event json.Marshal overhead**
   - `internal/server/http/sse_handler.go`
   - Fix: reuse encoder/buffer pool or batch events.

2. **No response compression middleware**
   - HTTP responses are sent uncompressed.
   - Fix: add gzip (or brotli) middleware for responses >1KB.

3. **Pagination without upper bounds**
   - `internal/server/http/api_handler.go` (limit param accepted without cap)
   - Fix: cap limit (e.g., 100) and document defaults.

---

### Web Bundle & Build Optimization

#### HIGH (P1)

1. **Unused dependency: react-syntax-highlighter**
   - `web/package.json`
   - No in-repo usage found; remove dependency and types.

2. **Large unmemoized components**
   - `web/components/agent/ArtifactPreviewCard.tsx` (1016 lines)
   - `web/app/conversation/components/ConversationMainArea.tsx` (30+ props)
   - Fix: add memoization and split rendering.

3. **Minimal memo usage in web UI**
   - Only 2 memoized components found (EventLine, ToolCallCard)
   - Fix: add memo for high-frequency render paths.

4. **Duplicate/overlapping static assets**
   - `web/public/elephant.jpg` and `web/public/elephant.jpeg`
   - Fix: consolidate and compress.

#### MEDIUM (P2)

5. **No bundle analyzer in CI**
   - Bundle regressions are not tracked.
   - Fix: add bundle size monitoring job.

---

## Updated Remediation Priority Matrix

| Priority | Issue | Files | Est. Effort | Impact | Status |
|----------|-------|-------|-------------|--------|--------|
| P0 | Configure pgxpool | internal/di/container.go | 30m | Connection stability | Pending |
| P0 | Add io.LimitReader | anthropic_client.go, openai_client.go | 30m | OOM prevention | Pending |
| P0 | Cap SSE per-connection caches | sse_handler.go | 1h | Memory stability | Pending |
| P0 | Add startup migration timeout | server.go, session_migration.go | 15m | Startup reliability | Pending |
| P0 | Remove react-syntax-highlighter | web/package.json | 15m | Smaller deps | Pending |
| P1 | Reduce scanner max buffer / pool | openai_client.go | 1h | Lower RSS under load | Pending |
| P1 | Event broadcaster lock strategy | event_broadcaster.go | 2h | Contention avoidance | Pending |
| P1 | GIN trigram index for memory content | migrations | 30m | Recall speed | Pending |
| P1 | Memo heavy components | ArtifactPreviewCard.tsx | 30m | Re-render reduction | Pending |
| P1 | Gzip compression middleware | server/http | 1h | Bandwidth reduction | Pending |
| P2 | Cache Intl formatters | web/lib/utils.ts | 30m | Minor CPU | Pending |
| P2 | Session LRU cache | internal/session/postgresstore/store.go | 1h | DB load reduction | Pending |
| P2 | Shared http.Client for web_fetch | web_fetch.go | 30m | Connection reuse | Pending |
| P2 | Prepared statements for hot paths | DB stores | 2h | Query CPU | Pending |
| P2 | Bundle analyzer in CI | web/ci | 1h | Visibility | Pending |
