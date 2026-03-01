# Elephant.ai Kernel Reliability Engineering Backlog

**Date**: 2026-03-01
**Scope**: Kernel engine, dispatch store, scheduler, taskfile, LLM planner
**Workspace State**: 14 modified files, 7 untracked, +964/-183 lines, 30 stash entries, 4 unpushed commits
**Telemetry**: 820 total dispatches, 247 failures (30.12%), 0 stale-running

---

## Executive Summary

The elephant.ai kernel is a cron-driven autonomous agent loop (PODATA: Perceive → Orient → Decide → Act → Update) that manages dispatch lifecycle through a file-backed store (`dispatches.json`) and markdown state artifact (`STATE.md`). Investigation reveals **4 critical**, **7 high**, and **10 medium** reliability risks across the dispatch lifecycle, scheduler recovery, and planning subsystems.

The **highest-impact finding** is a cluster of correlated risks in the dispatch persistence layer that, under specific failure sequences, can cause silent data loss or execution-without-recording. The **highest-probability finding** is that 30% dispatch failure rate is self-reinforcing because the planner lacks failure-signature awareness, causing repeated dispatch to broken upstream paths.

### Key Trade-offs Identified

1. **Simplicity vs. Safety**: The single-file JSON store provides excellent debuggability but creates a serialization bottleneck and whole-store rewrite on every mutation. Moving to a database adds complexity but enables proper transaction isolation.

2. **Autonomy vs. Observability**: The `sanitizeRuntimeSummary()` filter cleans LLM noise from STATE.md but risks destroying critical error signals. The 180→120→80 char truncation chain compounds this.

3. **Lease Duration vs. Dispatch Throughput**: Longer leases prevent false stale recovery but delay detection of truly stuck dispatches. The optimal value depends on the tail latency distribution of agent execution.

4. **Retry Complexity vs. Planner Intelligence**: The current model (mark-failed, let planner re-decide) is simple but creates a feedback loop where the planner penalizes agents for infrastructure failures. Structured retry would add dispatch lifecycle complexity but break the loop.

---

## Risk Matrix

### Severity Definitions
- **P0/Critical**: Data loss, state corruption, or silent failure that loses completed work
- **P1/High**: Reliability degradation under realistic conditions, incorrect recovery
- **P2/Medium**: Observable misbehavior, poor observability, configuration hazards
- **P3/Low**: Edge cases, cosmetic issues, minor log noise

| ID | Severity | Component | Risk | Probability | Impact |
|----|----------|-----------|------|------------|--------|
| K-01 | **P0** | engine.go | MarkRunning failure ignored → execution state mismatch | Medium | Critical |
| K-02 | **P0** | file_store.go | AtomicWrite truncation → silent dispatch data loss | Low | Critical |
| K-03 | **P0** | file_store.go | Prune-without-persist → memory/disk state divergence | High | High |
| K-04 | **P0** | status.go | Stop channel double-close → scheduler crash on shutdown | Medium | Critical |
| K-05 | **P1** | engine.go | sanitizeRuntimeSummary() strips all content → invisible execution | High | High |
| K-06 | **P1** | engine.go | markDispatchDoneResilient context race → potential double-write | Low | High |
| K-07 | **P1** | llm_planner.go | LLM timeout with no fallback → entire cycle aborted | Medium | High |
| K-08 | **P1** | file_store.go | RecoverStaleRunning clock-skew bypass | Low | High |
| K-09 | **P1** | job_runtime.go | inFlight counter leak on panic → job permanently blocked | Low | Critical |
| K-10 | **P1** | llm_planner.go | No failure-signature-aware dispatch → 30% self-reinforcing failure rate | High | High |
| K-11 | **P1** | status.go | writeUnsafe() error silenced → status file inconsistency | High | Medium |
| K-12 | **P2** | config.go | No config interdependency validation | High | Medium |
| K-13 | **P2** | engine.go | Atomicity gap in persistCycleRuntimeState (main vs fallback) | Low | Medium |
| K-14 | **P2** | llm_planner.go | GOAL file unreadable → silent planning context loss | Medium | Medium |
| K-15 | **P2** | llm_planner.go | Weak thinking-content heuristic (bracket match) → parse failures | Low | Medium |
| K-16 | **P2** | llm_planner.go | Prompt injection via user-editable STATE.md | Low | High |
| K-17 | **P2** | scheduler.go | Lock released before cron.Stop() → recovery timer race | Low | Medium |
| K-18 | **P2** | llm_planner.go | Empty AllowedTeamTemplates silently rejects all team dispatch | Medium | Low |
| K-19 | **P2** | job_runtime.go | persistJobLocked swallows all errors | High | Medium |
| K-20 | **P2** | engine.go | Cycle history parsing fragile (pipe-delimited markdown) | Medium | Low |
| K-21 | **P3** | config.go | Multiple hardcoded tuning constants not externalizable | High | Low |

---

## Backlog Items (Prioritized)

### SPRINT 1: Critical Data Integrity (P0) — Estimated 2-3 days

---

#### BL-01: Guard MarkRunning Failure in Dispatch Execution Loop
**Risk**: K-01
**File**: `internal/app/agent/kernel/engine.go` (~line 576)

**Problem**: When `store.MarkDispatchRunning()` fails, the error is logged but execution proceeds. The agent runs successfully, `MarkDispatchDone()` succeeds, but the dispatch was never marked RUNNING — creating an invalid state transition (PENDING → DONE) that confuses stale-recovery heuristics and telemetry.

**Root Cause**: Error handling treats MarkRunning as non-critical.

**Fix**:
```
Option A (Recommended): Fail the dispatch if MarkRunning fails.
  - If MarkRunning returns error, skip execution entirely
  - Mark dispatch as failed with "mark_running_failed" error class
  - Log at ERROR level (not WARN)

Option B: Retry MarkRunning with resilient pattern (like markDispatchDoneResilient)
  - Add markDispatchRunningResilient() with context fallback
  - Still skip execution if retry fails
```

**Acceptance Criteria**:
- [ ] No dispatch executes without successful RUNNING transition
- [ ] Failed MarkRunning produces `failure_class: infra_mark_running`
- [ ] Unit test: mock store returns error from MarkRunning → dispatch not executed
- [ ] Unit test: verify MarkRunning error logged at ERROR level

**Rollback Plan**: Revert to current warn-and-continue behavior (safe, just inconsistent state).

**Second-Order Effects**: May slightly reduce dispatch throughput if store has transient write issues. Mitigated by the resilient retry pattern.

---

#### BL-02: Validate AtomicWrite Integrity in FileStore
**Risk**: K-02
**File**: `internal/infra/kernel/file_store.go` (persistLocked, ~line 338)

**Problem**: If `filestore.AtomicWrite()` fails after truncating the original file but before completing the rename, `load()` sees an empty file and returns `nil` with no error — silently losing all dispatch records.

**Root Cause**: `load()` treats `len(data) == 0` as "no dispatches" rather than "corrupted/empty file."

**Fix**:
```
1. In load(): distinguish "file does not exist" from "file exists but empty"
   - os.IsNotExist(err) → valid empty state
   - File exists, len(data) == 0 → return error "dispatch store file empty, possible corruption"

2. In persistLocked(): add post-write verification
   - After AtomicWrite, stat the file to verify size > 0
   - If size == 0, return error (don't leave corrupt state)

3. Add backup rotation:
   - Before each persist, copy current file to dispatches.json.bak
   - On load failure, attempt recovery from .bak file
```

**Acceptance Criteria**:
- [ ] Empty file on load returns error, not silent nil
- [ ] Post-write size verification in persistLocked
- [ ] Backup file created before each write
- [ ] Recovery from backup tested in unit test
- [ ] Integration test: corrupt file → graceful recovery from backup

**Rollback Plan**: Remove backup rotation and size check; revert to current behavior.

**Trade-off**: Backup adds ~1ms per persist (additional file write). Acceptable for 30-minute cycle frequency.

---

#### BL-03: Persist After Prune in Terminal State Transitions
**Risk**: K-03
**File**: `internal/infra/kernel/file_store.go` (pruneLocked callers)

**Problem**: `MarkDispatchDone()` and `MarkDispatchFailed()` call `pruneLocked(ctx, now, false)` — the `false` flag means pruned dispatches are removed from memory but NOT persisted to disk. If the process crashes before next persist, pruned dispatches reappear on restart.

**Root Cause**: Optimization to avoid double-persist (the caller already persists the status change). But the prune removal and the status change are in the same persist call — so pruned records ARE persisted. The concern is if pruneLocked removes records AFTER the persist in the caller.

**Investigation Needed**: Verify exact call order. If `pruneLocked` runs after `persistLocked` in the same lock scope, the pruned records diverge until next persist.

**Fix**:
```
Option A (Minimal): Change persist flag to true in all pruneLocked calls
  - Cost: one extra file write per terminal transition
  - Simple, correct

Option B (Optimized): Move prune BEFORE persist in the caller
  - Prune first, then persist once with both changes
  - Requires restructuring MarkDispatchDone/Failed
```

**Acceptance Criteria**:
- [ ] After MarkDispatchDone + prune, `dispatches.json` matches in-memory state
- [ ] Unit test: mark done → verify pruned records not in file
- [ ] No file-read after crash shows records that were pruned in-memory

**Rollback Plan**: Revert to `persist=false` (current behavior). Memory/disk divergence only affects crash recovery of old terminal records — low blast radius.

---

#### BL-04: Fix StatusWriter Stop Channel Double-Close
**Risk**: K-04
**File**: `internal/domain/agent/taskfile/status.go` (~line 116)

**Problem**: If `Stop()` is called multiple times (e.g., during graceful shutdown + deferred cleanup), closing an already-closed channel panics in Go.

**Fix**:
```
Use sync.Once for channel close:
  - Add stopOnce sync.Once field to StatusWriter
  - Wrap close(sw.stopCh) in stopOnce.Do()

OR: Use atomic bool + select pattern:
  - Set atomic stopped flag
  - Polling goroutine checks flag instead of channel
```

**Acceptance Criteria**:
- [ ] Double Stop() call does not panic
- [ ] Polling goroutine exits cleanly after Stop()
- [ ] Unit test: concurrent Stop() calls from multiple goroutines
- [ ] Race detector clean (`-race` flag)

**Rollback Plan**: Wrap Stop() in recover() — masks the bug but prevents crash.

---

### SPRINT 2: Reliability Hardening (P1) — Estimated 3-4 days

---

#### BL-05: Add Summary Sanitization Safety Net
**Risk**: K-05
**File**: `internal/app/agent/kernel/engine.go` (sanitizeRuntimeSummary, ~line 490)

**Problem**: When all lines of an agent execution summary are filtered (code blocks, thinking markers, tool calls), the function returns empty string — rendered as "(none)" in STATE.md. This erases evidence of what the agent actually did, degrading planner decision quality in subsequent cycles.

**Fix**:
```
1. If filtered result is empty, return first N chars of raw input (unfiltered)
   with a "[truncated-unfiltered]" prefix
2. Log a WARN when all content was filtered
3. Increase summary char limit from 180 to 300 for error cases
4. Add metric: summary_fully_filtered_count
```

**Acceptance Criteria**:
- [ ] No agent execution produces "(none)" summary when raw output exists
- [ ] Filtered-empty case returns truncated raw content with marker
- [ ] WARN logged when full filter applied
- [ ] Unit test: input with only code blocks → returns truncated raw

**Rollback Plan**: Revert to current filter behavior. Summary loss is non-destructive (just informational).

---

#### BL-06: Add LLM Planner Timeout Retry with Backoff
**Risk**: K-07
**File**: `internal/app/agent/kernel/llm_planner.go` (~line 139)

**Problem**: Single LLM timeout aborts the entire planning phase, causing zero dispatches for that cycle. With 30-minute cycle intervals, this means 30 minutes of lost autonomy from a transient network blip.

**Fix**:
```
1. Add retry loop (max 2 retries) with exponential backoff:
   - Attempt 1: config.Timeout (30s)
   - Attempt 2: config.Timeout * 2 (60s)
   - Attempt 3: config.Timeout * 3 (90s)

2. On final failure, fall back to empty dispatch list (not error)
   - Log at ERROR level
   - Set CycleResult.Status = "planner_timeout" (new status)

3. Ensure HybridPlanner wraps LLMPlanner to fall back to static planner
   - Verify this is the default code path
```

**Acceptance Criteria**:
- [ ] Transient LLM timeout retried up to 2 times
- [ ] Total planning phase bounded by 3 * config.Timeout (180s max)
- [ ] Final timeout returns empty plan, not error
- [ ] Metric: planner_timeout_retries_total
- [ ] Unit test: mock LLM returns timeout → retried → succeeds on attempt 2

**Rollback Plan**: Remove retry loop; revert to single-attempt behavior.

**Trade-off**: 3x retry extends worst-case cycle time by ~3 minutes. Acceptable given 30-minute cycle interval.

---

#### BL-07: Implement Failure-Signature-Aware Dispatch Cooldown
**Risk**: K-10
**File**: `internal/app/agent/kernel/llm_planner.go`

**Problem**: 30% dispatch failure rate (247/820) is concentrated in specific agent lanes (`founder-operator`, `capital-explorer`) with recurring failure signatures (`other: 109`, `upstream_unavailable: 74`, `timeout_or_deadline: 49`). The planner redispatches without considering failure signatures, creating a self-reinforcing failure loop.

**Root Cause**: `shouldSkipAgentCooldown()` only checks time-since-last-dispatch, not failure signature. An agent that failed due to `upstream_unavailable` 5 minutes ago will be redispatched with the same upstream dependency.

**Fix**:
```
1. Add failure_signature field to Dispatch type (already classified in executor)
2. In LLM planner prompt, include per-agent failure signature distribution
3. Implement signature-specific cooldown rules:
   - upstream_unavailable: 60-minute cooldown (wait for upstream recovery)
   - timeout_or_deadline: 15-minute cooldown (transient, retry sooner)
   - other: 30-minute cooldown (default)
4. Export failure_signature_top3 in kernel_runtime block for observability
```

**Acceptance Criteria**:
- [ ] Planner prompt includes recent failure signatures per agent
- [ ] Per-signature cooldown prevents immediate redispatch
- [ ] `upstream_unavailable` agents cooled down for 60 minutes
- [ ] Failure rate trend delta visible in STATE.md
- [ ] Unit test: agent with upstream_unavailable within 60m → skipped

**Rollback Plan**: Remove signature-aware cooldown; revert to time-only cooldown.

**Second-Order Effect**: May reduce dispatch throughput temporarily. Expected net improvement: failure rate drops from 30% to <15% as self-reinforcing loops break.

---

#### BL-08: Fix RecoverStaleRunning Clock-Skew Vulnerability
**Risk**: K-08
**File**: `internal/infra/kernel/file_store.go` (RecoverStaleRunning)

**Problem**: If `Dispatch.UpdatedAt` is in the future (NTP adjustment, VM migration, test mocking), stale recovery is permanently bypassed for that dispatch.

**Fix**:
```
Add clock-skew guard:
  if d.UpdatedAt.After(now) {
      // Future timestamp detected — treat as stale (clock skew)
      d.Status = kernel.DispatchFailed
      d.Error = "recovered: future timestamp detected (clock_skew)"
  }
```

**Acceptance Criteria**:
- [ ] Dispatch with future UpdatedAt is recovered, not skipped
- [ ] Error message indicates clock_skew detection
- [ ] Unit test: UpdatedAt = now + 1 hour → recovered as stale

**Rollback Plan**: Remove clock-skew guard; revert to current behavior.

---

#### BL-09: Fix inFlight Counter Leak in Scheduler
**Risk**: K-09
**File**: `internal/app/scheduler/job_runtime.go` (~line 181)

**Problem**: If `coordinator.ExecuteTask()` panics and the panic is not recovered within the execution goroutine, `finishJob()` is never called. The `inFlight[jobID]` counter stays at 1, and the `maxConcurrent` check in `startJob()` prevents the job from ever running again.

**Fix**:
```
In the cron job callback (executor.go), add defer/recover:
  go func() {
      defer func() {
          if r := recover(); r != nil {
              s.finishJob(jobID, fmt.Errorf("panic: %v", r))
          }
      }()
      // ... existing execution logic ...
  }()
```

**Acceptance Criteria**:
- [ ] Panic in ExecuteTask → finishJob called with panic error
- [ ] inFlight counter decremented after panic recovery
- [ ] Job eligible for re-execution after panic
- [ ] Unit test: mock executor panics → counter resets

**Rollback Plan**: Remove recover wrapper; accept permanent job block on panic (current behavior).

---

#### BL-10: Surface StatusWriter Write Errors
**Risk**: K-11
**File**: `internal/domain/agent/taskfile/status.go` (~line 178)

**Problem**: `_ = sw.writeUnsafe()` discards the error. If disk is full or permissions change, status file becomes permanently stale with no indication.

**Fix**:
```
1. Return error from writeUnsafe and propagate in syncFromDispatcher
2. Log at WARN level on first failure, ERROR on consecutive failures
3. Add consecutive_write_failure counter
4. After N consecutive failures (e.g., 3), stop polling (avoid wasting cycles)
```

**Acceptance Criteria**:
- [ ] Write errors logged at WARN/ERROR level
- [ ] Consecutive failure counter tracked
- [ ] Polling stops after 3 consecutive write failures
- [ ] Unit test: mock file write failure → error logged

**Rollback Plan**: Revert to `_ = sw.writeUnsafe()`.

---

### SPRINT 3: Observability & Configuration (P2) — Estimated 2-3 days

---

#### BL-11: Add Config Interdependency Validation
**Risk**: K-12
**File**: `internal/app/agent/kernel/config.go`

**Problem**: No validation that `LeaseSeconds > TimeoutSeconds + PlannerTimeoutSec`, or that `MaxCycleHistory > 0`, or that `Schedule` is valid cron. Silent defaults mask misconfiguration.

**Fix**:
```
Add Validate() method to RuntimeSettings:
  func (s RuntimeSettings) Validate() error {
      if s.LeaseSeconds <= s.TimeoutSeconds {
          return fmt.Errorf("lease_seconds (%d) must exceed timeout_seconds (%d)", ...)
      }
      if _, err := cronParser.Parse(s.Schedule); err != nil {
          return fmt.Errorf("invalid schedule: %w", err)
      }
      // ... more checks ...
  }
```

**Acceptance Criteria**:
- [ ] Validate() called at Engine construction time
- [ ] Lease < Timeout returns clear error
- [ ] Invalid cron returns clear error
- [ ] Table-driven test covering all validation rules

**Rollback Plan**: Remove Validate() call; revert to silent defaults.

---

#### BL-12: Protect GOAL File Read with Caching
**Risk**: K-14
**File**: `internal/app/agent/kernel/llm_planner.go` (readGoalFile)

**Problem**: If GOAL file becomes unreadable mid-operation, planner silently operates without goals.

**Fix**:
```
1. Cache last-known-good GOAL content
2. On read failure, use cached version + log WARN
3. If no cache exists, include warning in planning prompt
```

**Acceptance Criteria**:
- [ ] Cached GOAL used when file unreadable
- [ ] WARN logged with cache age
- [ ] Planning prompt includes "[GOAL from cache, age=Xm]" marker

**Rollback Plan**: Remove caching; revert to current warn-and-continue.

---

#### BL-13: Externalize Engine Tuning Constants
**Risk**: K-21
**Files**: `config.go`, `engine.go`, `llm_planner.go`

**Problem**: Multiple hardcoded constants prevent per-environment tuning.

**Fix**:
```
Add EngineTuning struct to RuntimeSettings:
  type EngineTuning struct {
      MinRestartBackoffSec   int     // default 5
      MaxRestartBackoffSec   int     // default 300
      AbsenceAlertMinutes    int     // default 120
      AlertRepeatEveryN      int     // default 10
      PlannerTemperature     float64 // default 0.3
      PlannerMaxTokens       int     // default 8192
      GoalMaxRunes           int     // default 3000
      SummaryMaxChars        int     // default 180
  }
```

**Acceptance Criteria**:
- [ ] All previously-hardcoded values read from config
- [ ] Defaults match current behavior exactly
- [ ] No behavioral change without explicit config override

**Rollback Plan**: Hardcode values again if config surface proves too complex.

---

#### BL-14: Scheduler persistJobLocked Error Propagation
**Risk**: K-19
**File**: `internal/app/scheduler/job_runtime.go`

**Problem**: `persistJobLocked()` logs errors but never returns them. Callers cannot distinguish success from failure.

**Fix**:
```
1. Return error from persistJobLocked
2. In critical callers (startJob, finishJob), handle error:
   - startJob: if persist fails, don't increment inFlight
   - finishJob: if persist fails, log ERROR (but still decrement inFlight)
3. Non-critical callers: log and continue (current behavior, explicit)
```

**Acceptance Criteria**:
- [ ] persistJobLocked returns error
- [ ] startJob aborts on persist failure
- [ ] finishJob logs ERROR on persist failure
- [ ] Unit tests cover both paths

**Rollback Plan**: Revert persistJobLocked to void return.

---

## Workspace Hygiene Plan

### Current State
- 14 modified files (kernel, scheduler, planner, DI, taskfile)
- 7 untracked files (tests, plans, reports)
- 30 stash entries (debt)
- 4 unpushed commits to origin/main

### Recommended Commit Strategy

**Commit 1: Dispatch Store GC** (partially complete, ~80%)
```
Files:
  M internal/infra/kernel/file_store.go     (retention, prune, GC)
  M internal/app/agent/kernel/config.go     (RetentionSeconds, defaults)
  M internal/app/di/builder_hooks.go        (DI wiring for retention)
  + internal/infra/kernel/file_store_test.go (GC unit tests)
```

**Commit 2: LLM Planner Hardening**
```
Files:
  M internal/app/agent/kernel/llm_planner.go      (refactored planning)
  M internal/app/agent/kernel/llm_planner_test.go  (expanded tests)
  + internal/app/agent/kernel/config_defaults_test.go
```

**Commit 3: Scheduler Reliability**
```
Files:
  M internal/app/scheduler/executor.go
  M internal/app/scheduler/job_runtime.go
  M internal/app/scheduler/jobstore_file.go
  M internal/app/scheduler/scheduler.go
  M internal/app/scheduler/scheduler_test.go
```

**Commit 4: Engine Dispatch Resilience**
```
Files:
  M internal/app/agent/kernel/engine.go        (resilient marks, sanitizer)
  M internal/app/agent/kernel/executor_test.go
```

**Commit 5: Taskfile & Coordinator**
```
Files:
  M internal/domain/agent/taskfile/status.go
  M internal/app/agent/coordinator/config_resolver_test.go
  + cmd/alex/acp_rpc_additional_test.go
```

### Do NOT Commit
```
  ?? PLAN.md                         (ephemeral design doc)
  ?? STATE.md                        (runtime artifact, not source)
  ?? workspace_health_scan_latest.md (analysis artifact)
  ?? workspace_scan_report.md        (analysis artifact)
```

---

## Non-Obvious Trade-offs & Second-Order Effects

### 1. GC Retention Window vs. Planner Context Quality
**Trade-off**: Short retention (24h) keeps memory bounded but removes historical signal the planner uses. Long retention (14d) preserves signal but bloats store.

**Decision Point**: The LLM planner's `recentByAgent` lookback window must be ≤ retention period. If retention = 24h but planner looks back 7 days, it sees zero history for agents that haven't run recently.

**Recommendation**: Retain 48h minimum (preserves 96 cycles of history). Export compressed weekly summaries to a separate file for long-term planner context.

### 2. Resilient Mark Pattern Creates Hidden Latency
**Trade-off**: `markDispatchDoneResilient()` adds a 3-second fallback timeout on context cancellation. This means cycle completion can be delayed by up to `3s * MaxConcurrent` (9s) in worst case.

**Second-Order Effect**: If the kernel schedule is tight (e.g., every 5 minutes) and multiple dispatches timeout simultaneously, the resilient retry window can push cycle duration past the next scheduled tick, causing cycle overlap.

**Mitigation**: Ensure cycle completion timeout > MaxConcurrent * 3s + execution timeout.

### 3. Failure-Signature Cooldown Reduces Effective Agent Pool
**Trade-off**: Cooling down agents for 60 minutes after `upstream_unavailable` means fewer agents available per cycle. If 3 of 5 agents are cooled down, kernel throughput drops to 40%.

**Second-Order Effect**: Remaining uncooled agents receive higher dispatch load, potentially increasing THEIR failure rate.

**Mitigation**: Implement graduated cooldown (15m → 30m → 60m) rather than fixed 60m. Allow planner to override cooldown for high-priority tasks.

### 4. Atomic Backup Rotation Doubles I/O
**Trade-off**: Backing up `dispatches.json` before each write doubles the file I/O per persist operation.

**Decision Point**: Is 2ms of I/O per cycle (every 30 minutes) worth the crash recovery guarantee? Almost certainly yes, but monitor if cycle frequency increases.

### 5. Config Validation at Construction Time vs. Runtime
**Trade-off**: Validating config at Engine construction fails fast but prevents hot-reloading of config changes. Validating at runtime allows dynamic config but risks invalid config causing mid-cycle failures.

**Recommendation**: Validate at construction time. Config changes require restart (acceptable for a cron-driven system).

---

## Critical Decision Points

### Decision 1: Structured Retry vs. Planner Re-decision
**Context**: When a dispatch fails, should the kernel automatically retry it (structured retry with backoff), or should it let the planner re-evaluate in the next cycle?

**Current**: Planner re-decision (mark failed, planner decides next cycle)
**Alternative**: Add RetryCount/MaxRetries to Dispatch, auto-retry before planner involvement

**Recommendation**: Keep planner re-decision for now. Add failure signature to planner context (BL-07) to improve re-decision quality. Structured retry adds lifecycle complexity that the file-backed store doesn't handle well.

### Decision 2: File Store vs. SQLite for Dispatch Persistence
**Context**: FileStore has inherent atomicity limitations (whole-file rewrite, no transactions).

**Current**: JSON file with RWMutex
**Alternative**: SQLite with WAL mode (single-file, ACID, row-level operations)

**Recommendation**: Defer to post-stability. Current volume (820 dispatches, 48 cycles/day) is well within FileStore capacity. SQLite migration is warranted if:
- Dispatch volume exceeds 10K/day
- Multi-process access needed
- Transaction isolation required

### Decision 3: Commit Strategy for Dirty Tree
**Context**: 14 modified files span 4 subsystems. Broad commit risks mixed regressions.

**Options**:
- A: Single commit (fast, risky)
- B: Per-subsystem commits (5 commits, surgical)
- C: Per-backlog-item commits (most granular, highest overhead)

**Recommendation**: Option B (per-subsystem). Each commit runs the deterministic verification matrix before proceeding. This balances granularity with overhead.

---

## Verification Protocol

Before each commit, run:
```bash
# 1. Targeted kernel tests
go test ./internal/app/agent/kernel/... \
       ./internal/app/scheduler/... \
       ./internal/infra/kernel/... \
       ./internal/domain/agent/taskfile/... -count=1 -race

# 2. Command tests
go test ./cmd/alex/... -count=1

# 3. Static analysis
go vet ./internal/app/agent/kernel/... \
       ./internal/app/scheduler/... \
       ./internal/infra/kernel/...

# 4. Build verification
go build ./...
```

Post-final-commit, run full suite:
```bash
go test ./... -count=1 -race
```

---

## Timeline & Dependencies

```
Week 1 (Sprint 1 — P0 fixes):
  Day 1: BL-01 (MarkRunning guard) + BL-04 (StatusWriter stop)
  Day 2: BL-02 (AtomicWrite validation) + BL-03 (prune persist)
  Day 3: Verification, commit, push

Week 2 (Sprint 2 — P1 hardening):
  Day 1: BL-05 (summary safety) + BL-06 (planner retry)
  Day 2: BL-07 (failure-signature cooldown)
  Day 3: BL-08 (clock-skew) + BL-09 (inFlight leak) + BL-10 (write errors)
  Day 4: Verification, commit, push

Week 3 (Sprint 3 — P2 observability):
  Day 1: BL-11 (config validation) + BL-12 (GOAL caching)
  Day 2: BL-13 (tuning extraction) + BL-14 (persist error propagation)
  Day 3: Workspace hygiene, stash cleanup, verification, push
```

**Dependencies**:
- BL-03 must land before BL-02 (prune fix must precede backup logic)
- BL-07 depends on BL-05 (better summaries improve signature classification)
- BL-13 should land after BL-11 (validation infra needed before new config fields)
- BL-09 and BL-14 can be combined (both touch job_runtime.go)
