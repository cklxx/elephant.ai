# Backend Event ID Design Analysis

## Current ID Structure

### BaseEvent Fields (backend)
```go
type BaseEvent struct {
    timestamp    time.Time
    agentLevel   agent.AgentLevel  // "core" or "subagent"
    sessionID    string            // Session identifier
    taskID       string            // Current task ID
    parentTaskID string            // Parent task ID (for subagents)
    logID        string            // Log correlation ID
}
```

### ID Relationships in Subagent Flow

```
Core Agent Task (taskID: "task-parent-123")
    │
    ├── executes subagent tool (call_id: "call-subagent-456")
    │
    └── Subagent 1 (taskID: "task-sub-1", parentTaskID: "task-parent-123")
    │       ├── tool call (call_id: "call-tool-1")
    │       └── result.final
    │
    └── Subagent 2 (taskID: "task-sub-2", parentTaskID: "task-parent-123")
            ├── tool call (call_id: "call-tool-2")
            └── result.final
```

## Issues Found

### 1. **Ambiguous parent-child relationship**
- `parentTaskID` is set on subagent events, but the trigger event (subagent tool call) doesn't explicitly link to spawned subagents
- Frontend must infer relationship by matching `parentTaskID` of subagent events with `taskID` of trigger

### 2. **Inconsistent ID naming in JSON serialization**
Backend uses Go conventions (`TaskID`, `ParentTaskID`), but frontend receives:
```typescript
// Sometimes snake_case
parent_task_id: string

// Sometimes camelCase
parentTaskId: string

// Sometimes missing entirely
```

### 3. **call_id vs CallID confusion**
- Backend: `CallID string` (Go struct)
- Frontend: sometimes `call_id`, sometimes missing from type definitions
- Tool events have `call_id`, but other events don't

### 4. **Missing explicit trigger-to-execution link**
```
Problem: 5 subagents spawned, but no explicit "batch ID" to group them
Current: Use parentTaskID as implicit group key
Issue: If multiple subagent calls in same parent task, they get mixed up
```

## Recommended Fixes

### Option A: Add explicit `batch_id` or `invocation_id`
```go
type BaseEvent struct {
    // ... existing fields ...
    InvocationID string  // Unique ID for each subagent tool invocation
}

// Core agent events: InvocationID = ""
// Subagent events:   InvocationID = "inv-subagent-456"
```

### Option B: Add `spawned_by_call_id` to subagent events
```go
type BaseEvent struct {
    // ... existing fields ...
    SpawnedByCallID string  // For subagent events: the call_id that spawned them
}
```

### Option C: Include `subagent_metadata` in trigger event
```go
type WorkflowToolCompletedEvent struct {
    BaseEvent
    ToolName string
    Result   string
    // Add:
    SubagentMetadata struct {
        TaskIDs   []string  // List of subtask IDs spawned
        Count     int
        Mode      string    // "parallel" or "serial"
    }
}
```

## Frontend Adaptation (Implemented)

To work around current backend design, frontend now:

1. **Recognizes trigger events**: `tool_name: "subagent"` + `event_type: "workflow.tool.completed"`
2. **Uses `task_id` as parent identifier**: Groups all subagent events by `parent_task_id`
3. **Binds groups to trigger position**: Subagent cards render after trigger event, not by timestamp

```typescript
function isSubagentTrigger(event: AnyAgentEvent): boolean {
  return (
    event.event_type === "workflow.tool.completed" &&
    event.tool_name?.toLowerCase() === "subagent"
  );
}

// Build unified timeline with subagent groups attached to triggers
function buildUnifiedTimeline(mainStream, subagentGroups) {
  for (const event of mainStream) {
    if (isSubagentTrigger(event)) {
      const group = subagentGroups.get(event.task_id);
      if (group) {
        // Render subagent cards here, at trigger position
        result.push({ kind: "subagentGroup", trigger: event, threads: group });
        continue;
      }
    }
    // Regular event
    result.push({ kind: "event", event });
  }
}
```

## References

- [Event-Driven Multi-Agent Systems - Confluent](https://www.confluent.io/blog/event-driven-multi-agent-systems/)
- [Agentic AI Frontend Patterns - LogRocket](https://blog.logrocket.com/agentic-ai-frontend-patterns/)
