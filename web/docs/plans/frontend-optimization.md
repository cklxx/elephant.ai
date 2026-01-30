# Frontend Code Optimization Plan

**Date:** 2026-01-30
**Status:** In Progress

---

## P0 - Quick Wins (Low effort, high impact)

### 1. Dedup cache shift() -> circular buffer
- **File:** `hooks/useSSE/useSSEDeduplication.ts:48-53`
- **Problem:** Array `.shift()` is O(n) per eviction, 2000 elements per shift under high-frequency events
- **Fix:** Replace order array + shift with circular buffer or index-based eviction
- **Status:** [ ] Pending

### 2. Reconnection jitter
- **File:** `hooks/useSSE/useSSEConnection.ts:202-218`
- **Problem:** Pure exponential backoff without jitter causes thundering herd
- **Fix:** Add `Math.random() * 1000` jitter to delay calculation
- **Status:** [ ] Pending

### 3. Restore Zustand useShallow() selectors
- **File:** `hooks/useAgentStreamStore.ts:110`
- **Problem:** Selector hooks removed; consumers subscribe to entire store, causing cascading re-renders
- **Fix:** Add memoized selectors with `useShallow()` for common read patterns
- **Status:** [ ] Pending

### 4. Event buffer maxBufferSize
- **File:** `hooks/useSSE/useSSEEventBuffer.ts:103-109`
- **Problem:** Unbounded buffer growth when RAF is delayed (background tab, GC pause)
- **Fix:** Add early flush when buffer exceeds threshold (e.g., 50 events)
- **Status:** [ ] Pending

### 5. Lazy-load prism-react-renderer
- **Files:** `components/ui/markdown/components/MarkdownCode.tsx`, `components/agent/DocumentCanvas.tsx`, `components/agent/WebViewport.tsx`
- **Problem:** ~80KB+ syntax highlighting library loaded in main bundle at module level
- **Fix:** Dynamic import with loading fallback
- **Status:** [ ] Pending

---

## P1 - Medium effort, long-term value

### 6. Unified event matchers
- **Files:** `components/agent/ConversationEventStream.tsx`, `hooks/useSSE/useSSE.ts`, `lib/typeGuards.ts`
- **Problem:** 10+ functions doing similar event type checking, scattered across files
- **Fix:** Consolidate into `lib/events/eventMatchers.ts`
- **Status:** [ ] Pending

### 7. Markdown component type safety
- **Files:** `components/ui/markdown/hooks/useMarkdownComponents.tsx`, `components/ui/markdown/components/MarkdownTable.tsx`
- **Problem:** ~20 locations using `any` for component props
- **Fix:** Define `MarkdownComponentProps` interface hierarchy
- **Status:** [ ] Pending

### 8. Streaming delta truncation fix
- **File:** `hooks/useSSE/useSSE.ts:35`
- **Problem:** `slice(-10000)` truncates at character boundary, can break markdown structure (unclosed code blocks)
- **Fix:** Truncate at event boundary, not character boundary
- **Status:** [ ] Pending

### 9. Streaming buffer timeout GC
- **File:** `hooks/useSSE/useStreamingAnswerBuffer.ts`
- **Problem:** Network timeout leaves buffer entries until next session change
- **Fix:** Add TTL-based cleanup (e.g., 30s timeout)
- **Status:** [ ] Pending

---

## P2 - Architecture level (as needed)

### 10. Split large components
- `ArtifactPreviewCard.tsx` (1064 lines), `EventLine/index.tsx` (710 lines), etc.
- Extract sub-renderers

### 11. API endpoint registry + Zod response validation
- Centralize endpoints, add runtime validation

### 12. Structured logging
- Replace console.warn/error with logging service

### 13. Merge dual deduplication
- Consolidate EventPipeline (4000) + useSSEDeduplication (2000) into single layer

---

## Validation

After each change:
- `npx eslint . --no-warn-ignored`
- `npx vitest run`
- `npx tsc --noEmit` (check no new errors)
