# Plan: Lark Progress Display + Memory Mechanism

**Status**: Completed
**Date**: 2026-01-29

## Task 1: Intermediate Execution Progress Display

### Goal
Show tool execution progress in Lark via a single text message that updates in-place as tools start/complete.

### Implementation
- **`progress_listener.go`**: `progressListener` wraps `EventListener`, intercepts `WorkflowToolStartedEvent`/`WorkflowToolCompletedEvent`, rate-limits flushes (2s min), builds progress text format
- **`larkProgressSender`**: Concrete implementation using `sendMessageTypedWithID` / `updateMessage`
- **Gateway methods added**: `sendMessageTypedWithID`, `replyMessageTypedWithID`, `updateMessage` (using Lark `Im.Message.Update` PUT API)
- **Wired in `handleMessage`**: After emoji interceptor, before `ExecuteTask`

### Progress text format
```
[处理中...]
> web_search [done 1.2s]
> seedream [running 3s]
> bad_tool [error 0.5s]
```

## Task 2: Moltbot Memory Mechanism Replication

### Goal
Auto-save learnings after each task, auto-recall relevant context before each task.

### Implementation
- **`memory.go`**: `larkMemoryManager` with `SaveFromResult()` and `RecallForTask()` methods
- **Config**: `MemoryEnabled` added to `Config`, `LarkGatewayConfig`, `LarkChannelConfig`
- **Bootstrap**: Wires `MemoryService` from DI container when enabled
- **Gateway integration**: Recall before `ExecuteTask`, save after

## Validation
- All 30 tests pass (`go test ./internal/channels/lark/... -race -count=1`)
- Full project builds cleanly (`go build ./...`, `go vet ./...`)
- No data races detected
