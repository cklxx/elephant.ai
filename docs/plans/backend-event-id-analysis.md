# Backend Event ID Design Analysis

**Updated**: 2026-01-29 — Wiring gaps fixed, all fields now populate at runtime.

## Current ID Structure (Post-Redesign)

### BaseEvent Fields (backend)
```go
type BaseEvent struct {
    eventID       string           // Unique per event: "evt-{ksuid}"
    seq           uint64           // Monotonic within a run (via SeqCounter)

    timestamp     time.Time
    agentLevel    agent.AgentLevel // "core" or "subagent"
    sessionID     string           // Conversation scope
    runID         string           // This agent execution: "run-{12-char-suffix}"
    parentRunID   string           // Parent agent's runID (empty for core)

    correlationID string           // Root runID of the causal chain
    causationID   string           // call_id that spawned this run

    logID         string           // Log correlation
}
```

### ID Relationships in Subagent Flow

```
Core Agent Run (runID: "run-abcdef123456", correlationID: "run-abcdef123456")
    │
    ├── executes subagent tool (call_id: "call-subagent-789")
    │
    └── Subagent 1 (runID: "run-sub1...", parentRunID: "run-abcdef123456")
    │       correlationID: "run-abcdef123456"  ← root of chain
    │       causationID:   "call-subagent-789" ← tool call that spawned
    │       ├── tool call (call_id: "call-tool-1", seq: 1)
    │       └── result.final (seq: 5)
    │
    └── Subagent 2 (runID: "run-sub2...", parentRunID: "run-abcdef123456")
            correlationID: "run-abcdef123456"
            causationID:   "call-subagent-789"
            ├── tool call (call_id: "call-tool-2", seq: 1)
            └── result.final (seq: 4)
```

## Wiring Status

| Field            | Status | Location                                     |
|------------------|--------|----------------------------------------------|
| `event_id`       | Wired  | `domain/events.go:newBaseEventWithIDs`        |
| `seq`            | Wired  | `react/events.go:newBaseEvent` → `SeqCounter` |
| `correlation_id` | Wired  | Coordinator sets root; subagent inherits       |
| `causation_id`   | Wired  | Subagent tool sets `call.ID` on context        |
| `run_id`         | Wired  | 12-char suffix (was 6), ~71 bits entropy       |

## Changes Made (2026-01-29)

### P0: correlation_id / causation_id wiring
- **`coordinator.go:ExecuteTask`**: Core runs now set `correlationID = ensuredRunID` on context
  when no correlationID is present (root of causal chain).
- **`subagent.go:Execute`**: Sets `causationID = call.ID` (the subagent tool call's ID) on
  context before delegating to `executeSubtask`.
- **`subagent.go:executeSubtask`**: Propagates `correlationID` from parent context; falls back
  to parent's `runID` if parent has no correlationID.
- **`react/events.go:newBaseEvent`**: Reads `CorrelationIDFromContext` and
  `CausationIDFromContext` and stamps them on every emitted event.

### P1: SeqCounter wiring
- **`react/engine.go`**: Added `seq domain.SeqCounter` field to `ReactEngine`.
- **`react/events.go:newBaseEvent`**: Calls `e.seq.Next()` and sets the result via
  `base.SetSeq()` on every event.
- `SeqCounter` is `atomic.Uint64`-backed; each engine instance (one per `ExecuteTask` call)
  gets a fresh counter starting from 1.

### P2: RunID entropy increase
- **`generator.go`**: Changed `runIDSuffixLength` from 6 → 12.
- 12 chars of base-62 KSUID ≈ 71 bits of entropy → collision probability < 1 in 10^9 at
  10^6 IDs.

## Frontend Adaptation (Previously Implemented)

Frontend already consumes all fields via `WorkflowEnvelope`:

```typescript
interface WorkflowEnvelope<T> {
  event_type: T;
  timestamp: string;
  session_id: string;
  agent_level: 'core' | 'subagent';
  run_id?: string;
  parent_run_id?: string;
  event_id?: string;
  seq?: number;
  correlation_id?: string;
  causation_id?: string;
}
```

These fields are emitted in `sse_render.go:buildEventData` and now carry populated values.

## References

- [Event-Driven Multi-Agent Systems - Confluent](https://www.confluent.io/blog/event-driven-multi-agent-systems/)
- [Agentic AI Frontend Patterns - LogRocket](https://blog.logrocket.com/agentic-ai-frontend-patterns/)
