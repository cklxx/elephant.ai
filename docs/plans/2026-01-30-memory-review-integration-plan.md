# Memory Plans Review: Integration & Execution

> **Status:** Draft
> **Author:** cklxx
> **Created:** 2026-01-30
> **Updated:** 2026-01-30

## 1. Scope

This plan consolidates review findings for:
- `docs/plans/2026-01-30-memory-architecture-improvement.md`
- `docs/plans/2026-01-30-short-term-multi-turn-memory.md`

It captures the key gaps plus a merged execution sequence that preserves stable short-term context, avoids import cycles, and improves recall quality without introducing redundant systems.

## 2. Review Gaps to Address

- **Type/slot consistency**: plan text mentions `chat_message` vs `chat_turn` (pick one name and standardize on constants).
- **Metadata coverage**: `ConversationCaptureHook` should record `channel`, `chat_id`, `session_id`, `sender_id`, and `thread_id` (when present) so recalls can be filtered/scoped and TTL applied per type.
- **Session staleness reset**: clearing history must also clear any persisted summaries or metadata that `historyMgr` replays.
- **Group chat attribution**: multi-user Lark group history should preserve speaker identity in session turns to reduce coreference ambiguity.
- **TTL for vector index**: expiry must remove/ignore entries in both keyword and vector stores; define a single TTL policy in the store layer, not only query-time filters.
- **Recall budget**: add a per-type quota (e.g., max `chat_turn` 2, `auto_capture` 3, `user_explicit` 2) so recency noise does not dominate.
- **Import-cycle guardrails**: keep memory dependencies out of `agent/ports` to avoid cycles (inject into app/react layer only).

## 3. Integrated Execution Plan

### Phase 0: Lark stable session (short-term memory)
- Change Lark session ID to stable `memoryIDForChat(chatID)`.
- Add `session_stale_after` config and a reset routine that clears session history + summaries.
- Keep `AutoChatContext` only for group chat; ensure duplicates between session history and chat context are removed or tagged.

### Phase 1: Unified memory path (hooks only)
- Remove Lark gateway `larkMemoryManager` save/recall calls.
- Add `ConversationCaptureHook` storing **one turn pair** entry per task completion.
- Enrich memory slots with channel/session metadata.

### Phase 2: Hybrid recall (ranking)
- Enable HybridStore with RRF; verify embedder init + vector persist dir.
- Add explicit per-type recall quotas and a total token budget for recall injection.

### Phase 3: Memory quality & lifecycle
- Add TTL policy per memory type at store level (keyword + vector).
- Strengthen dedupe: use `(type, time window)` partitions and allow in-place updates for near-duplicates.
- Optional LLM summarization for long chat turns (behind config).

### Phase 4: Observability & tuning
- Structured logs and metrics for recall/capture, latency, and dedupe hit rate.
- Weekly review to adjust `max_recalls`, `alpha`, and TTLs based on observed hit rate and latency.

## 4. Test Plan

- Unit: conversation capture (turn pair + metadata + dedupe).
- Unit: session staleness reset (history + summary cleared).
- Unit: HybridStore RRF order with mixed keyword/vector candidates.
- Integration: Lark P2P multi-turn history persists; group chat retains AutoChatContext.
- Regression: ensure no import cycle by keeping memory services out of `agent/ports`.

## 5. Success Metrics

- Recall hit rate > 60% with hybrid search enabled.
- Recall p95 latency < 200ms.
- Dedupe skip rate > 30%.
- Lark P2P multi-turn coherence verified by manual and automated tests.
