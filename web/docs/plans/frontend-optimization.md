# Frontend Code Optimization Plan

**Date:** 2026-01-30
**Status:** P0/P1 Complete, P2 Backlog

---

## P0 - Quick Wins (Low effort, high impact)

### 1. Dedup cache shift() -> circular buffer
- **File:** `hooks/useSSE/useSSEDeduplication.ts`
- **Problem:** Array `.shift()` is O(n) per eviction, 2000 elements per shift under high-frequency events
- **Fix:** Implemented `RingDedupeCache` class with fixed-size circular buffer, O(1) eviction via head pointer
- **Status:** [x] Done — commit `9fa374c1`

### 2. Reconnection jitter
- **File:** `hooks/useSSE/useSSEConnection.ts:202`
- **Problem:** Pure exponential backoff without jitter causes thundering herd
- **Fix:** Added `+ Math.random() * 1000` jitter. Tests mock `Math.random` to return 0 for determinism.
- **Status:** [x] Done — commit `9fa374c1`

### 3. Restore Zustand useShallow() selectors
- **File:** `hooks/useAgentStreamStore.ts`
- **Problem:** Selector hooks removed; consumers subscribe to entire store
- **Finding:** No React components actually subscribe to the store via hook — only tests use `.getState()`. `useShallow` is already imported for future use.
- **Status:** [x] Not needed — no component-level subscribers exist

### 4. Event buffer maxBufferSize
- **File:** `hooks/useSSE/useSSEEventBuffer.ts`
- **Problem:** Unbounded buffer growth when RAF is delayed (background tab, GC pause)
- **Fix:** Added `MAX_BUFFER_SIZE = 50` constant; `enqueueEvent` flushes immediately when buffer reaches threshold
- **Status:** [x] Done — commit `9fa374c1`

### 5. Lazy-load prism-react-renderer
- **Files:** `MarkdownCode.tsx`, `DocumentCanvas.tsx`, `WebViewport.tsx`
- **Problem:** ~80KB+ syntax highlighting library in main bundle
- **Finding:** Already transitively lazy-loaded — MarkdownCode via `LazyMarkdownRenderer` (dynamic), DocumentCanvas & WebViewport via `ConsoleAgentOutput` (dynamic import at `app/dev/mock-console/page.tsx:11`)
- **Status:** [x] Not needed — already lazy-loaded

---

## P1 - Medium effort, long-term value

### 6. Unified event matchers / eliminate as-any
- **Files:** `lib/typeGuards.ts`, `hooks/useSSE/useSSE.ts`, `components/agent/VirtualizedEventList.tsx`
- **Problem:** 10+ files using `(event as any).property` for event field access
- **Fix:**
  - `typeGuards.ts`: introduced `prop<T>(event, key)` helper using `Record<string, unknown>`, eliminated all `as any`
  - `VirtualizedEventList.tsx`: replaced 4 inline `(event as any)` blocks with imported type guards (`isIterationNodeStartedEvent`, `isIterationNodeCompletedEvent`, `isWorkflowNodeStartedEvent`, `isWorkflowNodeCompletedEvent`)
  - `useSSE.ts` `mergeDeltaEvent`: replaced `(last as any)` / `(incoming as any)` with `Record<string, unknown>` cast
- **Status:** [x] Done — commit `cac1eba1`

### 7. Markdown component type safety
- **Files:** `components/ui/markdown/components/MarkdownTable.tsx`, `components/ui/markdown/hooks/useMarkdownComponents.tsx`
- **Problem:** ~20 locations using `any` for component props
- **Fix:**
  - `MarkdownTable.tsx`: all 6 components typed with `HTMLAttributes<HTMLTableElement>`, `HTMLAttributes<HTMLTableSectionElement>`, `HTMLAttributes<HTMLTableRowElement>`, `ThHTMLAttributes`, `TdHTMLAttributes`
  - `useMarkdownComponents.tsx`: all inline component props typed with `HTMLAttributes<HTMLElement>`, `AnchorHTMLAttributes`, `InputHTMLAttributes`, `ImgHTMLAttributes`. Only the `MdComponentMap` type alias retains `any` (required by react-markdown's component interface).
- **Status:** [x] Done — commit `cac1eba1`

### 8. Streaming delta truncation fix
- **File:** `hooks/useSSE/useSSE.ts:536-539`
- **Problem:** `slice(-10000)` truncates at character boundary, can break markdown structure
- **Fix:** When merged delta exceeds cap, discard history and keep only latest incoming chunk instead of mid-content slice
- **Status:** [x] Done — commit `9fa374c1`

### 9. Streaming buffer timeout GC
- **File:** `hooks/useSSE/useStreamingAnswerBuffer.ts`
- **Problem:** Network timeout leaves buffer entries until next session change
- **Fix:** Added `BUFFER_TTL_MS = 30_000` and `evictStale()` helper. Both `streamingAnswerBufferRef` and `assistantMessageBufferRef` now store timestamps and evict entries older than 30s on each write.
- **Status:** [x] Done — commit `9fa374c1`

---

## P2 - Architecture level (backlog)

### 10. Split large components
- `ArtifactPreviewCard.tsx` (1064 lines), `EventLine/index.tsx` (710 lines), etc.
- Extract sub-renderers
- **Status:** [ ] Backlog

### 11. API endpoint registry + Zod response validation
- Centralize endpoints, add runtime validation
- **Status:** [ ] Backlog

### 12. Structured logging
- Replace console.warn/error with logging service
- **Status:** [ ] Backlog

### 13. Merge dual deduplication
- Consolidate EventPipeline (4000) + useSSEDeduplication (2000) into single layer
- **Status:** [ ] Backlog

---

## Dead code removal (pre-optimization)

Deleted 3 unused components (459 lines total):
- `components/SmartErrorBoundary.tsx` (154 lines) — commit `5247ba89`
- `components/agent/ClarifyTimeline.tsx` (241 lines) — commit `5247ba89`
- `components/effects/MagicBlackHole.tsx` (64 lines) — commit `5247ba89`

---

## Validation

All changes validated:
- ESLint: 0 errors
- Vitest: 51 files, 290 tests passed
- TypeScript: no new errors introduced (pre-existing test type issues unchanged)
