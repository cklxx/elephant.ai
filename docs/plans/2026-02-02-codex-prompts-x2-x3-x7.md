# Codex Prompts: X2, X3, X7

Created: 2026-02-02
Purpose: Ready-to-use prompts for unblocking C12, C14, C22.

---

## X2: Checkpoint Write/Restore + Tool-in-flight Recovery

```
Context: elephant.ai Go project.

Codebase references (read these first):
- internal/agent/domain/react/checkpoint.go — Checkpoint schema, CheckpointStore interface, FileCheckpointStore (already implemented by C11)
- internal/agent/domain/react/runtime.go:180 — runIteration() loop: think → planTools → executeTools → observeTools → finish
- internal/agent/domain/react/solve.go — SolveTask entry point
- internal/agent/domain/react/engine.go — ReactEngine struct, fields, configuration
- internal/agent/domain/react/types.go — TaskState, TaskResult, ToolCall, ToolResult

Task: Wire checkpoint persistence into the ReAct engine's iteration loop and implement tool-in-flight recovery on restore.

Pre-work done by Claude Code:
- Checkpoint schema at internal/agent/domain/react/checkpoint.go (Checkpoint, MessageState, ToolCallState structs)
- CheckpointStore interface with Save/Load/Delete
- FileCheckpointStore implementation for local dev

Requirements:

1. Add a `CheckpointStore` field to the engine (ReactEngine or reactRuntime):
   - If nil, checkpointing is disabled (no-op)
   - Injected via constructor/option

2. Checkpoint write: After each successful observeTools() call in runIteration():
   - Build a Checkpoint from current state (messages, iteration, pending tools)
   - Call CheckpointStore.Save()
   - On save error: log warning, continue (don't fail the run)

3. Checkpoint restore: Add a `ResumeFromCheckpoint(ctx, sessionID)` method:
   - Load checkpoint from store
   - If nil (no checkpoint), return false (start fresh)
   - Reconstruct TaskState from checkpoint (messages, iteration counter)
   - Handle tool-in-flight recovery:
     a. For ToolCallState with Status="completed" — inject the result as an observation
     b. For Status="pending" or "running" — re-execute the tool call
     c. For Status="failed" — skip (treat as observation with error)
   - Delete the checkpoint after successful restore
   - Return true (resumed)

4. Conversion helpers:
   - `checkpointFromState(state *TaskState) *Checkpoint` — maps TaskState → Checkpoint
   - `stateFromCheckpoint(cp *Checkpoint) *TaskState` — maps Checkpoint → TaskState
   - Handle the MessageState ↔ ports.Message conversion (MessageState is simplified: role+content only)

5. Tests:
   - TestCheckpointWriteAfterObserve — verify checkpoint file is written after tools complete
   - TestResumeFromCheckpoint — save checkpoint, create new engine, verify resume continues from correct iteration
   - TestToolInFlightRecovery_Completed — pending tool with result → injected as observation
   - TestToolInFlightRecovery_Pending — pending tool without result → re-executed
   - TestNoCheckpointStore — verify nil store is no-op (no panics)
   - TestCheckpointDeleteAfterResume — verify checkpoint is cleaned up

Constraints:
- Follow existing patterns in runtime.go exactly
- Run `go vet ./...` and `go test ./internal/agent/domain/react/... -count=1` before delivering
- No unnecessary defensive code; trust context invariants
- Do NOT modify checkpoint.go (schema is frozen)
```

---

## X3: Retry Middleware (Exponential Backoff + Circuit Breaker + Context Propagation)

```
Context: elephant.ai Go project.

Codebase references (read these first):
- internal/tools/policy.go — ToolRetryConfig (MaxRetries, InitialBackoff, MaxBackoff, BackoffFactor), ToolPolicy interface, ResolvedPolicy
- internal/toolregistry/registry.go:140-238 — wrapper chain: wrapTool → ensureApprovalWrapper → idAwareExecutor → approvalExecutor
- internal/agent/ports/tools.go — ToolExecutor interface: Execute(ctx, ToolCall) → (*ToolResult, error)
- internal/agent/ports/tools.go:129 — ToolMetadata (Name, Dangerous, Category, Tags)

Task: Implement a retryExecutor middleware that wraps any ToolExecutor with exponential backoff, optional circuit breaker, and correct context propagation.

Pre-work done by Claude Code:
- ToolRetryConfig schema at internal/tools/policy.go
- ToolPolicy.RetryConfigFor(toolName, dangerous) returns per-tool retry config
- DefaultPolicyRules() with built-in rules (dangerous=no retry, web=3 retries, etc.)
- ToolPolicy is loaded via config at internal/config/load.go

Requirements:

1. Create `internal/toolregistry/retry.go`:
   - `retryExecutor` struct wrapping a delegate ToolExecutor + ToolRetryConfig
   - `Execute(ctx, call)`: retry loop with exponential backoff
     a. Call delegate.Execute(ctx, call)
     b. If success (err==nil AND result.Error==nil): return immediately
     c. If MaxRetries==0: return immediately (no retry)
     d. On failure: wait with jittered exponential backoff (InitialBackoff * BackoffFactor^attempt)
     e. Respect ctx.Done() — if context cancelled during backoff, return immediately
     f. Cap backoff at MaxBackoff
     g. After MaxRetries failures: return last error
   - Add jitter: ±20% randomization on backoff to prevent thundering herd
   - `Definition()` and `Metadata()` delegate to wrapped executor

2. Circuit breaker (lightweight, per-tool):
   - Track consecutive failures per tool name
   - After 5 consecutive failures: open circuit for 30 seconds
   - During open circuit: return error immediately without calling delegate
   - After 30s: half-open — allow one probe call
   - On probe success: close circuit; on probe failure: re-open
   - Use sync.Map for thread-safe per-tool state
   - Export CircuitBreakerConfig struct for tunability (ConsecutiveFailures, OpenDuration)

3. Integration point in wrapTool():
   - The retry wrapper should sit BETWEEN approvalExecutor and idAwareExecutor:
     Chain: idAwareExecutor → retryExecutor → approvalExecutor → delegate
   - wrapTool needs a ToolPolicy to resolve retry config per tool
   - Accept ToolPolicy as parameter to wrapTool (or NewRegistry)

4. Tests in `internal/toolregistry/retry_test.go`:
   - TestRetryExecutor_NoRetry — dangerous tool, MaxRetries=0
   - TestRetryExecutor_SuccessOnRetry — fails twice, succeeds on 3rd
   - TestRetryExecutor_ExhaustsRetries — fails MaxRetries+1 times
   - TestRetryExecutor_ContextCancelled — cancel during backoff
   - TestRetryExecutor_BackoffProgression — verify increasing delays
   - TestCircuitBreaker_Opens — 5 consecutive failures → circuit open
   - TestCircuitBreaker_HalfOpen — after timeout → allows probe
   - TestCircuitBreaker_Closes — successful probe → circuit closed
   - TestRetryExecutor_ResultError — result.Error (not Go error) is retried

Constraints:
- Use `math/rand` for jitter (NOT crypto/rand)
- Circuit breaker state must be goroutine-safe
- Run `go vet ./...` and `go test ./internal/toolregistry/... -count=1`
- Do NOT modify policy.go (schema is frozen)
- Follow the existing wrapper pattern (implement ToolExecutor interface exactly)
```

---

## X7: JobStore Enhancement — Cooldown + Concurrency Control + Failure Recovery

```
Context: elephant.ai Go project.

Codebase references (read these first):
- internal/scheduler/jobstore.go — Job struct, JobStore interface, JobStatus enum (already implemented by C21)
- internal/scheduler/jobstore_file.go — FileJobStore implementation with RWMutex
- internal/scheduler/scheduler.go — Scheduler struct, Start/Stop/Drain, executeTrigger
- internal/lifecycle/drainable.go — Drainable interface (already implemented)

Task: Enhance the scheduler with cooldown enforcement, concurrency control, and failure recovery for persistent jobs.

Pre-work done by Claude Code:
- JobStore interface (Save/Load/List/Delete/UpdateStatus) at internal/scheduler/jobstore.go
- FileJobStore implementation at internal/scheduler/jobstore_file.go (25 tests passing)
- Job struct with ID, Name, CronExpr, Trigger, Payload, Status, LastRun, NextRun, CreatedAt, UpdatedAt

Requirements:

1. Cooldown enforcement (`internal/scheduler/cooldown.go`):
   - `CooldownManager` struct tracking last execution time per job ID
   - `CanExecute(jobID string, cooldown time.Duration) bool` — returns false if last execution was within cooldown
   - `RecordExecution(jobID string)` — records current time as last execution
   - Thread-safe with sync.RWMutex
   - Configurable default cooldown (e.g., 5 minutes) and per-job overrides

2. Concurrency control (`internal/scheduler/concurrency.go`):
   - `ConcurrencyLimiter` struct with configurable max concurrent executions
   - `Acquire(ctx context.Context) error` — blocks until slot available or ctx cancelled
   - `Release()` — returns slot to pool
   - Use buffered channel as semaphore (simpler than sync.Semaphore)
   - Default max concurrency: 5

3. Failure recovery (`internal/scheduler/recovery.go`):
   - `RecoveryManager` struct that works with JobStore
   - `RecoverStaleJobs(ctx context.Context, store JobStore, timeout time.Duration) ([]Job, error)`:
     a. List all jobs with Status=active
     b. If UpdatedAt is older than timeout → mark as pending (stale/crashed job)
     c. Return recovered jobs for re-scheduling
   - `MarkRunning(ctx, store, jobID) error` — atomically set status=active + update timestamp
   - `MarkCompleted(ctx, store, jobID) error` — set status=completed + update LastRun
   - `MarkFailed(ctx, store, jobID, err) error` — set status=pending + record error in payload

4. Wire into Scheduler (`internal/scheduler/scheduler.go`):
   - Add CooldownManager, ConcurrencyLimiter, RecoveryManager fields
   - In Start(): call RecoverStaleJobs to rescue crashed jobs
   - In executeTrigger(): check cooldown → acquire concurrency → mark running → execute → mark completed/failed → release → record cooldown
   - Add `JobStore` field to Scheduler; if nil, persistence features are disabled

5. Tests:
   - TestCooldownManager_BasicCooldown — enforce 5min cooldown
   - TestCooldownManager_Concurrent — thread safety
   - TestConcurrencyLimiter_Basic — acquire/release cycle
   - TestConcurrencyLimiter_MaxReached — blocks when full, unblocks on release
   - TestConcurrencyLimiter_ContextCancelled — acquire returns error on cancel
   - TestRecoverStaleJobs — marks stale active jobs as pending
   - TestRecoverStaleJobs_NoStale — active jobs within timeout are untouched
   - TestScheduler_CooldownPreventsExecution — job skipped if within cooldown
   - TestScheduler_ConcurrencyLimit — verify max parallel executions

Constraints:
- No external dependencies beyond stdlib
- Thread-safe implementations required
- Run `go vet ./...` and `go test ./internal/scheduler/... -count=1`
- Do NOT modify jobstore.go or jobstore_file.go (schemas are frozen)
- Follow existing scheduler patterns exactly
```
