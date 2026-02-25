# Broadcaster Unbounded Session Metrics Memory Leak

**Date:** 2026-02-10
**Severity:** P1 — continuous memory growth in long-running processes
**Component:** `internal/delivery/server/app/event_broadcaster.go`

## Symptom

Backend process memory grows indefinitely over time, eventually leading to OOM. Memory profiling shows `sync.Map` entries in broadcaster metrics accumulating without bounds.

## Root Cause

Two `sync.Map` fields in `broadcasterMetrics` (`dropsPerSession`, `noClientBySession`) tracked per-session counters but never cleaned up abandoned sessions:

```go
type broadcasterMetrics struct {
    // ...
    dropsPerSession   sync.Map  // sessionID -> *atomic.Int64
    noClientBySession sync.Map  // sessionID -> *atomic.Int64
}
```

**Growth pattern**:
- Every new session creates a map entry
- Sessions from background tasks, orphaned processes, or transient requests accumulate
- No TTL or capacity limit → unbounded growth

## Fix

Replaced `sync.Map` with `boundedSessionCounterStore`:
- **Capacity cap**: 2048 entries max
- **TTL**: 30-minute idle expiration
- **LRU eviction**: Oldest sessions dropped when cap exceeded
- **Periodic pruning**: Every 256 operations

```go
type boundedSessionCounterStore struct {
    mu         sync.RWMutex
    entries    map[string]*sessionCounterEntry
    maxEntries int
    ttl        time.Duration
    ops        uint64
}
```

**Key methods**:
- `Increment(sessionID)` — auto-prune on threshold
- `Delete(sessionID)` — explicit cleanup on client disconnect
- `Snapshot()` — lock-free read for metrics export

## Tests

- `TestBoundedSessionCounterStoreCapsEntries` — capacity enforcement
- `TestEventBroadcasterCapsNoClientSessionMetrics` — 2000+ session integration test

## Code Review

**Verdict**: ✅ Approved (0 P0/P1 issues)
**Follow-up** (P2): Consider heap-based LRU if profiling shows `pruneLocked` bottleneck

## Lesson

For long-lived services with unbounded session/connection churn:
1. **Always cap in-memory maps** — use LRU, TTL, or both
2. **Prune proactively** — don't wait for manual cleanup
3. **Test at scale** — integration tests with 1000+ entries catch capacity issues
4. **Profile early** — catch memory leaks before production deployment

## References

- Commit: `36cea002` (fix), `bc6c494e` (plan update)
- Plan: `docs/plans/2026-02-10-startup-memory-oom-investigation.md`
