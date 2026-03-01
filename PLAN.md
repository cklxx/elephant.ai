# Elephant.AI Kernel Dispatch Loop: Deep Analysis & Highest-Value Change Recommendation

## Executive Summary

After thorough analysis of the kernel dispatch engine (`internal/app/agent/kernel/`), dispatch store (`internal/infra/kernel/`), domain types (`internal/domain/kernel/`), and bootstrap wiring (`internal/delivery/server/bootstrap/kernel.go`), this document identifies the **single highest-value code change** and provides concrete file-level patch guidance for three interrelated improvement areas: stale recovery, cycle_history, and hardcoded configurability.

---

## 1. Architecture Overview (PODATA Loop)

```
Engine.Run() → runLoop() → runCycleWithLogging() → RunCycle()
                                                     │
                ┌────────────────────────────────────┘
                ▼
    1. PERCEIVE   ─ Read STATE.md (opaque markdown)
    2. ORIENT     ─ RecoverStaleRunning() + ListRecentByAgent()
    3. DECIDE     ─ planner.Plan(state, recentByAgent) → []DispatchSpec
    4. ACT        ─ executeDispatches() with bounded concurrency
    5. UPDATE     ─ persistCycleRuntimeState() → upsert cycle_history in STATE.md
```

**Key insight**: The kernel is a *self-healing autonomous loop* that uses markdown state (STATE.md) as its single source of truth. The dispatch store (dispatches.json) is the operational queue. These two persistence layers serve different purposes but share timing/recovery semantics that are currently coupled through hardcoded constants.

---

## 2. Critical Analysis: Three Problem Domains

### 2.1 Stale Recovery

**Current Implementation** (file_store.go:364-404):
- `RecoverStaleRunning()` scans all dispatches for `status=running` with `UpdatedAt` older than `leaseDuration`
- Called at the START of every cycle (engine.go:142-149)
- Recovery = mark as failed with descriptive error string
- `leaseDuration` defaults to 5 minutes (file_store.go:38-39)

**Non-obvious Issues**:

1. **Lease/Recovery Duration Mismatch**: `FileStore.leaseDuration` = 5 min, but `KernelConfig.LeaseSeconds` = 1800 (30 min). The `NewFileStore()` in bootstrap receives `leaseDuration` as a parameter, but the default fallback in `NewFileStore` is 5 min — far too aggressive for dispatches that can run 15+ minutes (`DefaultKernelTimeoutSeconds` = 900). If the bootstrap passes `LeaseSeconds` correctly this is fine, but the **fallback** creates a silent failure mode where valid running dispatches get incorrectly recovered as stale.

2. **No Recovery Backoff/Retry**: Stale dispatches are marked `failed` permanently. The planner sees them as failed history and may re-dispatch the same work, but there's no structured retry-with-backoff mechanism. If the failure was transient (process restart, OOM), the same agent will be re-dispatched without learning from the failure mode.

3. **Recovery Only Runs Once Per Cycle**: If a cycle takes 15 minutes and another dispatch goes stale during execution, it won't be recovered until the next cycle's ORIENT phase. This is acceptable for the current 30-min schedule but becomes problematic with shorter schedules.

4. **No Notification on Recovery**: Stale recovery is logged at WARN level but doesn't trigger the CycleNotifier. Operators only see it in logs, not in Lark/notification channels.

**Second-Order Effects**:
- Aggressive recovery (5min default) + long dispatch timeout (15min) = phantom failures when `FileStore` is created without explicit lease duration
- The LLM planner sees "failed: recovered stale" in history → may penalize the agent or skip it, causing cascading underutilization

### 2.2 Cycle History

**Current Implementation** (engine.go:214-226, 305-463):
- Rolling history table embedded as markdown in STATE.md
- Parsed with string splitting (not a structured format)
- Prepended (newest first), truncated to `MaxCycleHistory` (default 5)
- Summary truncated to 120 chars for table cells, 80 chars for error messages, 180 chars for agent summaries

**Non-obvious Issues**:

1. **Unbounded Dispatch Store Growth**: While `cycle_history` in STATE.md is bounded, `dispatches.json` grows monotonically. Every cycle appends 1-5 dispatches. At 48 cycles/day (every 30 min), that's 240 dispatch records/day. After 30 days: 7,200+ records, all held in-memory. `ListRecentByAgent()` and `RecoverStaleRunning()` scan ALL dispatches every cycle. **This is the most impactful operational risk in the codebase.**

2. **History Parse Fragility**: `parseCycleHistory()` uses string splitting on `|` characters. The summary field replaces `|` with `/` (engine.go:386), but if an agent summary contains the literal string `### cycle_history` or `<!-- KERNEL_RUNTIME:END -->`, parsing breaks silently.

3. **No History Linkage to Dispatch Store**: Cycle history entries contain `cycle_id` but the dispatch store also indexes by `cycle_id`. There's no GC mechanism that says "all dispatches from cycles older than the history window can be pruned." This is the missing link between the two persistence layers.

4. **Stateful Summary Truncation Chain**: The truncation pipeline is `500 chars (extractKernelExecutionSummary) → 180 chars (renderStateAgentSummary) → 120 chars (buildCycleHistoryEntry) → 80 chars (error only)`. Each layer independently truncates, which can create misleading partial messages. The LLM planner receives these truncated summaries as planning context.

**Second-Order Effects**:
- Growing `dispatches.json` causes increasingly slow cycle startup as `ListRecentByAgent` scans all records
- Memory footprint grows linearly with uptime (no eviction)
- LLM planner receives degraded (over-truncated) history signal → worse dispatch decisions over time

### 2.3 Hardcoded Configurability

**Inventory of Hardcoded Values by Severity**:

| Severity | Value | Location | Current | Impact |
|----------|-------|----------|---------|--------|
| **CRITICAL** | `leaseDuration` fallback | file_store.go:39 | 5 min | Phantom stale recovery for valid dispatches |
| **HIGH** | `absenceAlertThreshold` | engine.go:844 | 2 hours | Cannot tune alerting for different environments |
| **HIGH** | `minBackoff/maxBackoff` | engine.go:721-722 | 5s/5m | Cannot tune restart behavior |
| **HIGH** | `alert repeat frequency` | engine.go:871 | every 10 failures | Not adaptable to schedule frequency |
| **MEDIUM** | GOAL.md char limit | llm_planner.go:194 | 3000 runes | May truncate rich goal documents |
| **MEDIUM** | LLM temperature | llm_planner.go:147 | 0.3 | Cannot experiment without code change |
| **MEDIUM** | LLM max_tokens | llm_planner.go:148 | 8192 | Fixed regardless of model capability |
| **MEDIUM** | Summary truncation (80/120/180/500) | engine.go, executor.go | Various | Cannot tune signal quality to planner |
| **LOW** | Health threshold fallback | engine.go:921 | 70 min | Conservative but reasonable |
| **LOW** | `defaultKernelAttemptCount` | executor.go:96 | 1 | Semantic constant, rarely needs change |

---

## 3. Highest-Value Change: Dispatch Store GC with Configurable Retention

### Why This Is #1

The **unbounded dispatch store growth** is the intersection of all three problem domains:

1. **Stale Recovery**: GC prevents recovery from scanning ancient records (performance)
2. **Cycle History**: GC creates a clean retention boundary between STATE.md history and dispatch store
3. **Configurability**: Retention period must be configurable per environment

This single change provides:
- **O(active) instead of O(all-time) scan** for `ListRecentByAgent` and `RecoverStaleRunning`
- **Bounded memory** regardless of uptime
- **Natural alignment** between cycle_history retention and dispatch retention
- **Foundation** for future improvements (structured retry, historical analytics)

### Concrete Implementation Plan

#### Step 1: Add Retention Config to FileStore

**File**: `internal/infra/kernel/file_store.go`

```go
// BEFORE (line 26-32):
type FileStore struct {
    mu            sync.RWMutex
    dispatches    map[string]kernel.Dispatch
    filePath      string
    leaseDuration time.Duration
    now           func() time.Time
}

// AFTER:
type FileStore struct {
    mu               sync.RWMutex
    dispatches       map[string]kernel.Dispatch
    filePath         string
    leaseDuration    time.Duration
    retentionPeriod  time.Duration  // NEW: terminal dispatches older than this are purged
    now              func() time.Time
}
```

**File**: `internal/infra/kernel/file_store.go` — Modify `NewFileStore`

```go
// BEFORE (line 37-47):
func NewFileStore(dir string, leaseDuration time.Duration) *FileStore {
    if leaseDuration <= 0 {
        leaseDuration = 5 * time.Minute
    }
    ...
}

// AFTER:
func NewFileStore(dir string, leaseDuration time.Duration, retentionPeriod time.Duration) *FileStore {
    if leaseDuration <= 0 {
        leaseDuration = 30 * time.Minute  // FIX: align default with DefaultKernelLeaseSeconds
    }
    if retentionPeriod <= 0 {
        retentionPeriod = 24 * time.Hour  // Default: keep 24h of terminal dispatches
    }
    return &FileStore{
        dispatches:      make(map[string]kernel.Dispatch),
        filePath:        filepath.Join(dir, "dispatches.json"),
        leaseDuration:   leaseDuration,
        retentionPeriod: retentionPeriod,
        now:             time.Now,
    }
}
```

#### Step 2: Add GC Method

**File**: `internal/infra/kernel/file_store.go` — New method after `RecoverStaleRunning`

```go
// PurgeTerminalDispatches removes terminal (done/failed/cancelled) dispatches
// older than the configured retention period. Returns count of purged records.
// Called at the end of each cycle to bound store growth.
func (s *FileStore) PurgeTerminalDispatches(ctx context.Context, kernelID string) (int, error) {
    if err := ctx.Err(); err != nil {
        return 0, err
    }

    cutoff := s.now().Add(-s.retentionPeriod)

    s.mu.Lock()
    defer s.mu.Unlock()

    var purged int
    for id, d := range s.dispatches {
        if d.KernelID != kernelID {
            continue
        }
        if !isTerminalDispatchStatus(d.Status) {
            continue
        }
        if d.UpdatedAt.Before(cutoff) {
            delete(s.dispatches, id)
            purged++
        }
    }

    if purged > 0 {
        if err := s.persistLocked(); err != nil {
            return 0, fmt.Errorf("persist after purge: %w", err)
        }
    }
    return purged, nil
}
```

#### Step 3: Add Domain Interface

**File**: `internal/domain/kernel/store.go` — The GC should be optional (like stale recovery)

No change to the `Store` interface. Instead, use the same pattern as `staleDispatchRecoverer`:

**File**: `internal/app/agent/kernel/engine.go` — Add interface

```go
// AFTER the staleDispatchRecoverer interface (line 66-68):
type terminalDispatchPurger interface {
    PurgeTerminalDispatches(ctx context.Context, kernelID string) (int, error)
}
```

#### Step 4: Invoke GC in Cycle Lifecycle

**File**: `internal/app/agent/kernel/engine.go` — In `persistCycleRuntimeState` or as a new post-cycle step

The cleanest placement is in `RunCycle`, after `executeDispatches` returns but before `persistCycleRuntimeState` runs (via defer). Add after line 180:

```go
// 7. Purge old terminal dispatches to bound store growth.
if purger, ok := e.store.(terminalDispatchPurger); ok {
    purged, purgeErr := purger.PurgeTerminalDispatches(ctx, e.config.KernelID)
    if purgeErr != nil {
        e.logger.Warn("Kernel: purge terminal dispatches failed: %v", purgeErr)
    } else if purged > 0 {
        e.logger.Info("Kernel: purged %d old terminal dispatch(es)", purged)
    }
}
```

#### Step 5: Add Retention Config to KernelConfig

**File**: `internal/app/agent/kernel/config.go`

```go
// Add to RuntimeSettings:
type RuntimeSettings struct {
    // ... existing fields ...
    DispatchRetentionHours int  // NEW: hours to retain terminal dispatches; default 24
}

// Add to KernelConfig:
type KernelConfig struct {
    // ... existing fields ...
    DispatchRetentionHours int  // NEW
}

// Add default constant:
const DefaultKernelDispatchRetentionHours = 24
```

#### Step 6: Wire in Bootstrap

**File**: `internal/delivery/server/bootstrap/kernel.go` (or wherever `NewFileStore` is called)

Pass the new retention parameter when constructing `FileStore`:

```go
retentionPeriod := time.Duration(settings.DispatchRetentionHours) * time.Hour
if retentionPeriod <= 0 {
    retentionPeriod = 24 * time.Hour
}
store := kernel.NewFileStore(storeDir, leaseDuration, retentionPeriod)
```

#### Step 7: Add Tests

**File**: `internal/infra/kernel/file_store_test.go` — New tests

```go
func TestFileStore_PurgeTerminalDispatches_RemovesOldTerminal(t *testing.T) {
    now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
    store := &FileStore{
        dispatches:      make(map[string]kerneldomain.Dispatch),
        retentionPeriod: 24 * time.Hour,
        now:             func() time.Time { return now },
    }

    // Old terminal dispatch (48h ago) — should be purged
    store.dispatches["old-done"] = kerneldomain.Dispatch{
        DispatchID: "old-done", KernelID: "k1",
        Status: kerneldomain.DispatchDone,
        UpdatedAt: now.Add(-48 * time.Hour),
    }
    // Recent terminal dispatch (1h ago) — should be kept
    store.dispatches["recent-done"] = kerneldomain.Dispatch{
        DispatchID: "recent-done", KernelID: "k1",
        Status: kerneldomain.DispatchDone,
        UpdatedAt: now.Add(-1 * time.Hour),
    }
    // Old but still running — should NOT be purged
    store.dispatches["old-running"] = kerneldomain.Dispatch{
        DispatchID: "old-running", KernelID: "k1",
        Status: kerneldomain.DispatchRunning,
        UpdatedAt: now.Add(-48 * time.Hour),
    }

    purged, err := store.PurgeTerminalDispatches(context.Background(), "k1")
    if err != nil { t.Fatalf("purge: %v", err) }
    if purged != 1 { t.Fatalf("expected 1 purged, got %d", purged) }
    if _, ok := store.dispatches["old-done"]; ok { t.Fatal("old-done should be purged") }
    if _, ok := store.dispatches["recent-done"]; !ok { t.Fatal("recent-done should be kept") }
    if _, ok := store.dispatches["old-running"]; !ok { t.Fatal("old-running should be kept") }
}

func TestFileStore_PurgeTerminalDispatches_NoPurgeSameAsNoPersist(t *testing.T) {
    now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
    store := &FileStore{
        dispatches:      make(map[string]kerneldomain.Dispatch),
        retentionPeriod: 24 * time.Hour,
        now:             func() time.Time { return now },
    }
    // Only recent dispatches
    store.dispatches["recent"] = kerneldomain.Dispatch{
        DispatchID: "recent", KernelID: "k1",
        Status: kerneldomain.DispatchDone,
        UpdatedAt: now.Add(-1 * time.Hour),
    }

    purged, err := store.PurgeTerminalDispatches(context.Background(), "k1")
    if err != nil { t.Fatalf("purge: %v", err) }
    if purged != 0 { t.Fatalf("expected 0 purged, got %d", purged) }
}
```

**File**: `internal/app/agent/kernel/engine_test.go` — Add GC integration test

```go
func TestEngine_RunCycle_PurgesTerminalDispatches(t *testing.T) {
    // Use a memStore that also implements terminalDispatchPurger
    // Verify purge is called after dispatch execution
}
```

#### Step 8: Fix the Lease Duration Default (Bonus Critical Fix)

**File**: `internal/infra/kernel/file_store.go` line 38-39

```go
// BEFORE:
if leaseDuration <= 0 {
    leaseDuration = 5 * time.Minute
}

// AFTER:
if leaseDuration <= 0 {
    leaseDuration = 30 * time.Minute  // Align with DefaultKernelLeaseSeconds (1800s)
}
```

This prevents the silent failure mode where dispatches are recovered as stale while still legitimately running.

---

## 4. Additional Recommendations (Lower Priority)

### 4.1 Extract Engine Tuning Config (Medium Priority)

**File**: `internal/app/agent/kernel/config.go`

Add an `EngineTuning` struct to `RuntimeSettings`:

```go
type EngineTuning struct {
    MinRestartBackoffSeconds int  // default 5
    MaxRestartBackoffSeconds int  // default 300
    AbsenceAlertMinutes      int  // default 120
    AlertRepeatEveryNFailures int // default 10
}
```

Then replace hardcoded constants in `engine.go:720-722` and `engine.go:844,871` with config reads.

**Trade-off**: More configuration surface = more operational complexity. Only do this if you have multiple deployment environments with different SLA requirements.

### 4.2 Expose LLM Planner Tuning (Medium Priority)

**File**: `internal/app/agent/kernel/llm_planner.go`

Add to `LLMPlannerConfig`:
```go
type LLMPlannerConfig struct {
    // ... existing ...
    Temperature     float64  // default 0.3
    MaxTokens       int      // default 8192
    GoalMaxRunes    int      // default 3000
}
```

Replace hardcoded values at lines 147-148 and 194.

**Trade-off**: Different models have different optimal temperature/token settings. Making this configurable enables model swapping without code changes, but adds fields that most operators will never touch.

### 4.3 Structured Stale Recovery Retry (Lower Priority)

Instead of marking stale dispatches as permanently failed, add a retry count:

```go
// In Dispatch type:
RetryCount int `json:"retry_count,omitempty"`
MaxRetries int `json:"max_retries,omitempty"`
```

Recovery would increment `RetryCount` and re-set status to `pending` if `RetryCount < MaxRetries`. This requires changes to:
- `internal/domain/kernel/types.go` (add fields)
- `internal/infra/kernel/file_store.go` (recovery logic)
- `internal/app/agent/kernel/llm_planner.go` (respect retry context)

**Trade-off**: Adds complexity to dispatch lifecycle. The current model (mark failed, let planner re-decide) is simpler and relies on the planner's intelligence. Structured retry is better for deterministic tasks but worse for tasks that need prompt redesign after failure.

---

## 5. Decision Matrix

| Change | Effort | Risk | Value | Dependencies |
|--------|--------|------|-------|-------------|
| **Dispatch Store GC** | Medium (2-3 days) | Low | **Very High** | None |
| Lease Duration Fix | Trivial (1 line) | Very Low | High | None |
| Engine Tuning Config | Low (1 day) | Low | Medium | None |
| LLM Planner Tuning | Low (1 day) | Low | Medium | None |
| Structured Retry | High (3-5 days) | Medium | Medium | GC should go first |

---

## 6. Files Modified (Complete Changeset for Recommended Change)

| File | Change Type | Lines Affected |
|------|-------------|---------------|
| `internal/infra/kernel/file_store.go` | Add `retentionPeriod` field, modify `NewFileStore`, add `PurgeTerminalDispatches`, fix lease default | ~40 new/modified lines |
| `internal/app/agent/kernel/engine.go` | Add `terminalDispatchPurger` interface, add GC call in `RunCycle` | ~15 new lines |
| `internal/app/agent/kernel/config.go` | Add `DispatchRetentionHours` to `RuntimeSettings` and `KernelConfig`, add default constant | ~5 new lines |
| `internal/delivery/server/bootstrap/kernel.go` | Wire retention period to `NewFileStore` constructor | ~5 modified lines |
| `internal/infra/kernel/file_store_test.go` | Add GC unit tests | ~50 new lines |
| `internal/app/agent/kernel/engine_test.go` | Add GC integration test, update `NewFileStore` calls | ~30 new lines |

**Total**: ~145 lines changed across 6 files. No breaking interface changes. Fully backward compatible.

---

## 7. Risk Assessment

**What could go wrong**:
1. **Aggressive retention purges dispatch data needed by planner** — Mitigated by 24h default (48 cycles of history preserved)
2. **NewFileStore signature change breaks callers** — Audit all `NewFileStore` call sites (bootstrap only). Consider using options pattern if more params are expected.
3. **GC during cycle adds latency** — Purge is O(n) where n = total dispatches, but occurs after execution completes. At 7200 records the scan is sub-millisecond.
4. **Lease duration default change affects existing deployments** — Only affects cases where `leaseDuration` is not explicitly passed. If bootstrap always passes it explicitly, this is a no-op safety net.

**What to monitor after deployment**:
- `dispatches.json` file size over 7 days (should stabilize at ~24h worth)
- "purged N old terminal dispatch(es)" log frequency
- Planner dispatch quality (no regression in dispatch decisions)
