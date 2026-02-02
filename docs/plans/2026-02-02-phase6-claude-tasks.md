# Phase 6: Claude Code Tasks — P1/P2 Execution

Created: 2026-02-02
Purpose: Execute remaining Claude-owned P1/P2 tasks from expanded roadmap.

---

## Actionable Tasks (not blocked)

### Batch 1 (parallel)

| # | Task | Priority | Description |
|---|------|----------|-------------|
| C27 | Tool SLA baseline collection | P1 | SLA wrapper in registry chain: per-tool latency/error-rate/call-count metrics via Prometheus |
| C28 | Memory Flush-before-Compaction (D3) | P2 | Emit event before AutoCompact, MemoryFlushHook saves key info to memory store |
| C29 | Dynamic scheduler job tool | P2 | `scheduler_create/list/delete` tools for Agent to manage jobs from conversation |
| C30 | Provider health detection | P2 | Expose circuit breaker states per LLM provider, health endpoint |
| C31 | Lark Approval API | P2 | Approval API wrapper (query/submit/track) following C3/C4 Lark SDK pattern |

### Batch 2 (parallel, after Batch 1)

| # | Task | Priority | Description |
|---|------|----------|-------------|
| C32 | Context priority sorting | P2 | Rank context fragments by relevance/freshness before compression |
| C33 | Cost-aware context trimming | P2 | Token budget + cost budget drives context retention |
| C34 | Token budget management | P2 | Per-session token quota + auto-downgrade model on overspend |
| C35 | Lark smart card interaction | P2 | Interactive Card builder + button callback handling |
| C36 | Proactive group summary | P2 | Auto-summarize group discussions by message volume/time |

### Batch 3

| # | Task | Priority | Description |
|---|------|----------|-------------|
| C37 | Message type enrichment | P2 | Tables, code blocks, Markdown rendering in Lark messages |
| C38 | Tool result caching | P2 | Semantic dedup cache layer for tool results |
| C39 | Auto degradation chain | P2 | Fallback chain: cache → weaker tool → prompt user |
| C40 | CI evaluation gating | P2 | Finalize CI eval workflow |

### Already Done (discovered during exploration)

| # | Task | Status |
|---|------|--------|
| F | Scheduler startup recovery | Already implemented in `job_runtime.go:loadPersistedJobsLocked()` |

---

## Execution Status

| Task | Status | Commit |
|------|--------|--------|
| C27 | DONE | 2f832f93 |
| C28 | DONE | 1636bcf9 |
| C29 | DONE | 3e7ffec3 |
| C30 | DONE | d05a1a1e |
| C31 | DONE | a1011915 |
| C32 | DONE | 3123fd0a |
| C33 | DONE | 696e48e5 |
| C34 | DONE | 8c7f0e1f |
| C35 | DONE | 1650cee8 |
| C36 | DONE | 75e939cb |
| C37 | DONE | 29e31dc0 |
| C38 | DONE | bbc5a38c |
| C39 | DONE | d8f3fc12 |
| C40 | DONE | 27849e69 |

---

## Summary

All 14 Claude-owned tasks (C27-C40) completed across 3 batches.
3 tasks remain blocked on Codex: C12, C14, C22.
