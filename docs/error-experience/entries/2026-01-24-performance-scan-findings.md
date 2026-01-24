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
   - Status: ✅ replaced client map with copy-on-write + atomic reads to remove hot read locks.

5. **JSON pretty-print logging on LLM responses**
   - `internal/llm/anthropic_client.go`, `internal/llm/openai_client.go`, `internal/llm/openai_responses_client.go`, `internal/llm/antigravity_client.go`
   - Status: ✅ replaced with direct logging to avoid `json.Indent` overhead on every request.

6. **Per-user rate limiter map lock**
   - `internal/llm/user_rate_limit_client.go`
   - Limiter creation occurs under a single mutex; optimize if it becomes hot.

### MEDIUM (P2)

7. **web_fetch HTTP client per tool instance**
   - `internal/tools/builtin/web_fetch.go`
   - If tool instances are short-lived, consider a shared `http.Client` to reuse pools.
   - Status: ✅ shared singleton client; removed background cleanup goroutine.

---

## Web Frontend Findings

### CRITICAL (P0)

1. **Prop drilling anti-pattern**
   - `web/app/conversation/components/ConversationMainArea.tsx` (30+ props)
   - Status: ✅ grouped props into structured objects and memoized the component to reduce re-renders.

2. **SSE event buffer O(n) processing**
   - `web/hooks/useSSE/useSSE.ts`
   - Status: ✅ added per-task index map to avoid full-array scans during streaming updates.

3. **Markdown re-parse on delta updates**
   - `web/components/ui/markdown/StreamingMarkdownRenderer.tsx`
   - Status: ✅ render plain text while streaming and defer markdown parse until settled.

### HIGH (P1)

4. **Next.js image optimization disabled (by design for static export)**
   - `web/next.config.mjs` sets `output: "export"` and `images.unoptimized = true`.
   - Required for static export; only actionable if moving to server rendering.

5. **Missing React.memo on high-frequency components**
   - Only 2 memoized components found (EventLine, ToolCallCard).
   - Status: ✅ memoized `ArtifactPreviewCard`, `ConversationMainArea`, `ConversationHeader`, `QuickPromptButtons`.

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
   - Status: ✅ added tests for `file_edit`, `list_files`, `memory_recall`.

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
   - Status: ✅ configured `pgxpool.Config` with tunable pool sizing, lifetimes, and health checks.

2. **Prepared statements not used in hot DB paths**
   - Session/memory/history stores use Exec/Query without prepare
   - Impact: parse/plan overhead per call; measure before adding.
   - Status: ✅ enabled pgx statement cache and cached exec mode for session DB pool.

#### HIGH (P1)

3. **Memory store fallback ILIKE without index**
   - `internal/memory/postgres_store.go` (content ILIKE '%...%')
   - Status: ✅ trigram GIN index creation already present when `pg_trgm` is available.

4. **Session store List without pagination limit**
   - `internal/session/postgresstore/store.go:220`
   - Status: ✅ added limit/offset with sensible defaults and call-site updates.

5. **Async event store drops on full buffer**
   - `internal/server/app/async_event_history_store.go`
   - Status: ✅ added bounded wait/backpressure and surfaced queue-full error.

---

### Memory & GC Patterns

#### HIGH (P1)

1. **Streaming scanner max buffer is 2MB**
   - `internal/llm/openai_client.go`
   - Status: ✅ reduced max buffer to 512KB via shared stream scanner helper.

2. **Unbounded event history storage growth**
   - `internal/server/app/postgres_event_history_store.go`
   - Impact: DB storage grows indefinitely without retention policy
   - Status: ✅ added retention window with periodic pruning.

3. **io.ReadAll without size limits (OOM risk)**
   - `internal/llm/anthropic_client.go`, `internal/llm/openai_client.go`
   - Status: ✅ wrapped response reads with bounded `io.LimitReader` (10MB cap).

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
   - Status: ✅ added LRU cache with updated_at validation to avoid redundant JSON parsing.

---

### Concurrency & Goroutine Patterns

#### HIGH (P1)

1. **HTTP server WriteTimeout = 0**
   - `internal/server/bootstrap/server.go`
   - Status: ✅ mitigated with non-stream request timeout middleware while keeping SSE safe.

2. **SSE handler per-connection caches are unbounded**
   - `internal/server/http/sse_handler.go` (`sentAttachments`, `finalAnswerCache`)
   - Status: ✅ capped with LRU caches to bound memory per connection.

3. **Startup migration without timeout**
   - `internal/server/bootstrap/server.go` + session migration
   - Status: ✅ migration now guarded by a startup timeout.

4. **Event broadcaster lock order complexity**
   - `internal/server/app/event_broadcaster.go`
   - Multiple locks are used; no deadlock observed, but contention and ordering complexity are risks.
   - Status: ✅ removed hot-path RW locks via copy-on-write client map.

5. **MCP process restart loses monitor goroutines**
   - `internal/mcp/process.go`
   - Status: ✅ reinitializes `stopChan` and wait channel on Start; restart-safe.

---

### API Serialization & Response

#### HIGH (P1)

1. **SSE per-event json.Marshal overhead**
   - `internal/server/http/sse_handler.go`
   - Fix: reuse encoder/buffer pool or batch events.

2. **No response compression middleware**
   - Status: ✅ gzip middleware added for non-stream responses.

3. **Pagination without upper bounds**
   - `internal/server/http/api_handler.go` (limit param accepted without cap)
   - Status: ✅ capped session/task/snapshot/evaluation limits with sane maxima.

---

### Web Bundle & Build Optimization

#### HIGH (P1)

1. **Unused dependency: react-syntax-highlighter**
   - `web/package.json`
   - Status: ✅ removed dependency and types; lockfiles updated.

2. **Large unmemoized components**
   - `web/components/agent/ArtifactPreviewCard.tsx` (1016 lines)
   - `web/app/conversation/components/ConversationMainArea.tsx` (30+ props)
   - Status: ✅ memoized heavy components and reduced prop churn.

3. **Minimal memo usage in web UI**
   - Only 2 memoized components found (EventLine, ToolCallCard)
   - Fix: add memo for high-frequency render paths.

4. **Duplicate/overlapping static assets**
   - `web/public/elephant.jpg` and `web/public/elephant.jpeg`
   - Status: ✅ consolidated to a single asset.

#### MEDIUM (P2)

5. **No bundle analyzer in CI**
   - Bundle regressions are not tracked.
   - Status: ✅ added bundle analyzer job in CI.

---

## Updated Remediation Priority Matrix

| Priority | Issue | Files | Est. Effort | Impact | Status |
|----------|-------|-------|-------------|--------|--------|
| P0 | Configure pgxpool | internal/di/container.go | 30m | Connection stability | ✅ Done |
| P0 | Add io.LimitReader | anthropic_client.go, openai_client.go | 30m | OOM prevention | ✅ Done |
| P0 | Cap SSE per-connection caches | sse_handler.go | 1h | Memory stability | ✅ Done |
| P0 | Add startup migration timeout | server.go, session_migration.go | 15m | Startup reliability | ✅ Done |
| P0 | Remove react-syntax-highlighter | web/package.json | 15m | Smaller deps | ✅ Done |
| P1 | Reduce scanner max buffer / pool | openai_client.go | 1h | Lower RSS under load | ✅ Done |
| P1 | Event broadcaster lock strategy | event_broadcaster.go | 2h | Contention avoidance | ✅ Done |
| P1 | GIN trigram index for memory content | migrations | 30m | Recall speed | ✅ Done |
| P1 | Memo heavy components | ArtifactPreviewCard.tsx | 30m | Re-render reduction | ✅ Done |
| P1 | Gzip compression middleware | server/http | 1h | Bandwidth reduction | ✅ Done |
| P2 | Cache Intl formatters | web/lib/utils.ts | 30m | Minor CPU | ✅ Done |
| P2 | Session LRU cache | internal/session/postgresstore/store.go | 1h | DB load reduction | ✅ Done |
| P2 | Shared http.Client for web_fetch | web_fetch.go | 30m | Connection reuse | ✅ Done |
| P2 | Prepared statements for hot paths | DB stores | 2h | Query CPU | ✅ Done |
| P2 | Bundle analyzer in CI | web/ci | 1h | Visibility | ✅ Done |
