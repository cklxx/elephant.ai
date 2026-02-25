# Async Session Save Per Turn

**Date**: 2026-02-11
**Status**: ✅ Completed

## Goal

Implement async session saving after each ReAct loop iteration so that:
1. Lark sessions appear in web diagnostics even during ongoing conversations
2. Sessions are persisted progressively instead of only at task completion
3. Concurrent saves are properly serialized with mutex protection

## Problem

Currently:
- Sessions only save when `ExecuteTask` completes (via `SaveSessionAfterExecution`)
- During a conversation, session exists only in Lark process memory
- Web diagnostics (`/dev/diagnostics`) queries database, can't see in-progress sessions
- User confirmed: "没结束就不存吗" (doesn't save until finished?) → Yes

## Solution Design

### Architecture

```
ReactEngine.run()
  for each iteration:
    runIteration()
      → think()
      → planTools()
      → executeTools()
      → observeTools()
      → saveCheckpoint()
      → finish()
      → [NEW] Call sessionPersister(session, state) asynchronously
```

### Components

1. **Add SessionPersister callback to ReactEngine**
   - Optional callback: `func(ctx context.Context, session *storage.Session, state *TaskState)`
   - Passed via `ReactEngineConfig`
   - Called after each `runIteration()` completes successfully

2. **Coordinator provides implementation**
   - Wraps `sessionStore.Save()` with mutex protection
   - Runs async (goroutine) to avoid blocking ReAct loop
   - Logs errors but doesn't fail the iteration

3. **Mutex protection in coordinator**
   - Add `sessionSaveMu sync.Mutex` to `AgentCoordinator`
   - Protects against concurrent saves of same session
   - Works with final `SaveSessionAfterExecution` call

## Implementation Steps

### 1. Add SessionPersister to react domain
- [ ] Add `SessionPersister` type to `internal/domain/agent/ports/agent/types.go`
- [ ] Add field to `ReactEngineConfig` in `internal/domain/agent/react/factory.go`
- [ ] Add field to `ReactEngine` in `internal/domain/agent/react/engine.go`

### 2. Call persister after each iteration
- [ ] In `runtime.go`, after `runIteration()` succeeds, call persister asynchronously
- [ ] Pass current `session` from ExecutionEnvironment and `state`
- [ ] Handle nil persister gracefully

### 3. Implement persister in coordinator
- [ ] Add `sessionSaveMu sync.Mutex` to `AgentCoordinator`
- [ ] Create `asyncSaveSession()` method with mutex
- [ ] Pass persister to ReactEngine via config

### 4. Update ExecutionEnvironment
- [ ] Ensure `env.Session` is accessible to runtime
- [ ] Session reference needs to flow through to runtime

### 5. Testing
- [ ] Add test for async save being called
- [ ] Verify mutex prevents concurrent saves
- [ ] Verify existing `SaveSessionAfterExecution` still works

## Files to Modify

1. `internal/domain/agent/ports/agent/types.go` - Add SessionPersister type
2. `internal/domain/agent/react/factory.go` - Add to config
3. `internal/domain/agent/react/engine.go` - Add field
4. `internal/domain/agent/react/runtime.go` - Call persister after iteration
5. `internal/app/agent/coordinator/coordinator.go` - Implement persister + mutex
6. `internal/app/agent/coordinator/coordinator_test.go` - Add tests

## Safety Considerations

- **Non-blocking**: Async save must not block ReAct loop
- **Error handling**: Save failures logged but don't break conversation
- **Mutex**: Prevent race between per-turn saves and final save
- **Memory**: Session might be large after truncation; clone if needed

## Progress

- [x] Step 1: Add SessionPersister type and config
- [x] Step 2: Wire through ReactEngine
- [x] Step 3: Implement in coordinator
- [x] Step 4: Test - all tests passing
- [ ] Step 5: Verify with Lark session visibility (pending manual test)

## Implementation Summary

### Changes Made

1. **Added SessionPersister callback type** (`internal/domain/agent/ports/agent/types.go`):
   - `SessionPersister func(ctx context.Context, session *storage.Session, state *TaskState)`
   - Optional callback for async session persistence

2. **Extended ReactEngineConfig** (`internal/domain/agent/react/engine.go`):
   - Added `SessionPersister` field to config
   - Added `sessionPersister` field to `ReactEngine` struct
   - Wired through in factory (`factory.go`)

3. **Call persister after each iteration** (`internal/domain/agent/react/runtime.go`):
   - Added `persistSessionAfterIteration()` method
   - Called after each successful `runIteration()`
   - Non-blocking, doesn't fail iteration on error

4. **Coordinator implementation** (`internal/app/agent/coordinator/coordinator.go`):
   - Added `sessionSaveMu sync.Mutex` for concurrent save protection
   - Added `asyncSaveSession()` method - goroutine-based async save
   - Provided SessionPersister closure to ReactEngine that captures `env.Session`
   - Mutex prevents race between per-turn saves and final save

### Test Results

All tests passing:
- `internal/domain/agent/react/...` ✅
- `internal/app/agent/coordinator/...` ✅
- Full test suite (short mode) ✅

## Notes

- Existing `SaveSessionAfterExecution` remains for final save
- Truncation logic (1000 message limit) applies to all saves
- Lark standalone still has no EventBroadcaster, but DB sync is enough for diagnostics
