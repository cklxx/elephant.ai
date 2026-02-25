# Agent Event Loop & ID System Redesign

> Status: Draft
> Created: 2026-01-29
> Supersedes: `backend-event-id-analysis.md` (partial)

---

## 1. Industry Research Summary

### 1.1 Four-Level ID Hierarchy (All Production Systems Converge)

```
Session/Thread  →  Run/Execution  →  Step/Span  →  Call/Action
```

| Level | Scope | OpenAI Assistants | LangGraph | Temporal | OTel GenAI |
|-------|-------|-------------------|-----------|----------|------------|
| Session | Conversation | `thread_abc123` | `thread_id` | Namespace | `gen_ai.conversation.id` |
| Run | Single agent invocation | `run_abc123` | `run_id` (UUID) | Workflow Run ID | `trace_id` |
| Step | One discrete operation | `step_abc123` | `langgraph_step` | Activity ID | `span_id` |
| Call | Individual tool call | `call_abc123` | - | - | `gen_ai.tool.call.id` |

### 1.2 Cross-Cutting Correlation (Event Sourcing Pattern)

```
correlation_id  ← links entire causal chain (copied from trigger)
causation_id    ← direct parent event (set to trigger's event_id)
```

Every event carries both. Enables two key projections:
- `correlation-{id}` stream: all events in a logical conversation
- `causation-{id}` stream: only direct children of a specific event

### 1.3 Subagent Grouping Patterns

| System | Mechanism |
|--------|-----------|
| OpenAI Assistants | One `RunStep` contains array of `tool_call` objects with `call_id` |
| OpenAI Agents SDK | `group_id` on Trace; parent `AgentSpan` contains child `HandoffSpan` |
| LangGraph | `parent_ids` list; `checkpoint_ns` pipe-separated path (`"parent\|child"`) |
| Temporal | Parent Workflow starts Child Workflows with distinct IDs; parent holds refs |
| Prefect | Bidirectional: parent has `child_flow_run_id`, child has `parent_task_run_id` |

### 1.4 Key Design Decisions from Industry

1. **Typed span data** (OpenAI Agents SDK) > generic payloads — self-documenting schema
2. **Large payloads on events, not span attributes** — OTel recommendation, backends choke on multi-KB attributes
3. **Bidirectional parent-child references** (Prefect) — prevents dangling references
4. **Namespace paths** (LangGraph `checkpoint_ns`) — elegant for hierarchical nesting
5. **Separate correlation_id from run_id** — correlation tracks causal chains across runs

---

## 2. Current Architecture Gap Analysis

### 2.1 Current BaseEvent

```go
type BaseEvent struct {
    timestamp    time.Time
    agentLevel   agent.AgentLevel  // "core" | "subagent"
    sessionID    string
    taskID       string
    parentTaskID string
    logID        string
}
```

### 2.2 Gap Matrix

| Aspect | Industry Standard | Current State | Gap |
|--------|-------------------|---------------|-----|
| **Event identity** | Unique `event_id` per event | None | No dedup, no causation tracking |
| **Run-level ID** | Dedicated `run_id` per agent invocation | `taskID` conflates run + task | Cannot distinguish re-runs of same task |
| **Step tracking** | `step_index` or `span_id` | Iteration count in payload only | No first-class step identity |
| **Causal chain** | `correlation_id` + `causation_id` | None | Cannot trace event lineage |
| **Trigger-to-child link** | Explicit (call_id → spawned tasks) | Implicit (parent_task_id reverse lookup) | Multi-batch subagent calls collide |
| **Hierarchy depth** | Arbitrary nesting (namespace path) | 2 levels only (core/subagent) | Cannot support nested subagents |
| **Event ordering** | Monotonic sequence + event_id | Timestamp only | Unreliable ordering under concurrency |

### 2.3 Concrete Failure Scenarios

**Scenario 1: Same parent, multiple subagent batches**
```
Core task-A:
  iter 3 → subagent tool → spawns [sub-1, sub-2]    (batch 1)
  iter 5 → subagent tool → spawns [sub-3, sub-4]    (batch 2)
```
All four subagents have `parent_task_id = task-A`. Frontend cannot distinguish batch 1 from batch 2.

**Scenario 2: Event replay ordering**
SSE reconnect replays events by history order. Without `event_id` + monotonic sequence,
duplicate detection relies on timestamp + event_type heuristic — brittle under clock skew.

**Scenario 3: Debug tracing**
A subagent calls a tool that fails. To find the root cause, need to walk:
`tool error → subagent invocation → parent tool call that spawned it → parent LLM decision`
Without causation_id chain, this requires manual timestamp correlation across logs.

---

## 3. Target Design

### 3.1 ID Hierarchy

```
session_id          ← conversation scope (existing, no change)
  └── run_id        ← single agent execution (NEW — replaces taskID's run semantics)
        └── seq     ← monotonic event sequence within run (NEW)
```

Cross-cutting:
```
correlation_id      ← root run_id of the causal chain (NEW)
causation_id        ← call_id of the trigger that spawned this run (NEW)
```

### 3.2 Target BaseEvent

```go
type BaseEvent struct {
    // Identity
    eventID   string          // Unique per event: "evt-{ksuid}"
    seq       uint64          // Monotonic within a run, for ordering

    // Temporal
    timestamp time.Time

    // Hierarchy
    sessionID    string       // Conversation: "session-{ksuid}" (unchanged)
    runID        string       // This agent execution: "run-{ksuid}" (NEW, replaces taskID)
    parentRunID  string       // Parent agent's runID (replaces parentTaskID)
    agentLevel   AgentLevel   // "core" | "subagent" (unchanged, derived from parentRunID)

    // Causal chain
    correlationID string      // Root runID of the chain (NEW)
    causationID   string      // call_id that spawned this run (NEW)

    // Operational
    logID string              // Log correlation (unchanged)
}
```

### 3.3 SSE Envelope (wire format)

```json
{
  "event_id":       "evt-2Bx...",
  "event_type":     "workflow.tool.completed",
  "seq":            42,
  "timestamp":      "2026-01-29T10:00:00.123Z",

  "session_id":     "session-abc",
  "run_id":         "run-parent",
  "parent_run_id":  null,
  "agent_level":    "core",

  "correlation_id": "run-parent",
  "causation_id":   null,

  "payload": {
    "call_id":    "call-subagent-1",
    "tool_name":  "subagent",
    "result":     "...",
    "duration":   1234
  }
}
```

Subagent event from the batch above:
```json
{
  "event_id":       "evt-3Cx...",
  "event_type":     "workflow.node.output.delta",
  "seq":            5,
  "timestamp":      "2026-01-29T10:00:01.456Z",

  "session_id":     "session-abc",
  "run_id":         "run-sub-1",
  "parent_run_id":  "run-parent",
  "agent_level":    "subagent",

  "correlation_id": "run-parent",
  "causation_id":   "call-subagent-1"
}
```

**Frontend grouping becomes deterministic:**
- Group by `causation_id` → all subagents from the same tool call
- Anchor to trigger event where `payload.call_id == subagent.causation_id`
- No ambiguity even with multiple subagent batches in same parent run

### 3.4 ID Lifecycle Summary

```
                    ┌─────────────────────────────────────────────────┐
 User submits task  │  session_id = existing or new                  │
                    │  run_id     = NewRunID()                       │
                    │  correlation_id = run_id  (root of chain)      │
                    │  causation_id   = ""      (user-initiated)     │
                    └────────────────────┬────────────────────────────┘
                                         │
                    ┌────────────────────▼────────────────────────────┐
 ReAct loop iter N  │  Each event:                                   │
                    │    event_id = NewEventID()                     │
                    │    seq      = atomicIncr()                     │
                    │    run_id   = parent's run_id                  │
                    └────────────────────┬────────────────────────────┘
                                         │ LLM calls subagent tool
                                         │ (call_id = "call-xyz")
                    ┌────────────────────▼────────────────────────────┐
 Subagent spawned   │  run_id         = NewRunID()                   │
                    │  parent_run_id  = parent's run_id              │
                    │  correlation_id = parent's correlation_id      │
                    │  causation_id   = "call-xyz" (trigger call_id) │
                    └─────────────────────────────────────────────────┘
```

---

## 4. Migration Strategy

### 4.1 Principles

- **No compatibility shims** — refactor from first principles per project rules
- **Backend-first** — backend emits new fields, frontend adapts
- **Single rename pass** — `taskID → runID` across the entire codebase in one commit

### 4.2 Phases

#### Phase 1: Core ID Rename (backend, one atomic commit)

**Scope:** Rename `taskID` → `runID`, `parentTaskID` → `parentRunID` across all Go code.

Files to change:
- `internal/agent/domain/events.go` — BaseEvent fields + accessors
- `internal/agent/domain/envelope.go` — WorkflowEventEnvelope construction
- `internal/agent/domain/react/` — engine, runtime, tooling, tool_batch, observe, world, subagent_snapshot, executor_snapshot
- `internal/agent/ports/agent/` — AgentEvent interface, TaskState, context
- `internal/server/http/sse_render.go` — buildEventData key names
- `internal/server/app/` — event history store, task progress
- `internal/utils/id/` — NewTaskID → NewRunID, context helpers

SSE wire format change:
```diff
- "task_id":        event.GetTaskID(),
- "parent_task_id": event.GetParentTaskID(),
+ "run_id":         event.GetRunID(),
+ "parent_run_id":  event.GetParentRunID(),
```

#### Phase 2: Add event_id + seq (backend)

**Scope:** Generate unique event identity and monotonic ordering.

Changes:
- `BaseEvent` gets `eventID string` + `seq uint64`
- `NewEventID()` function in `internal/utils/id/`
- Atomic sequence counter per run in ReactEngine
- `sse_render.go` emits `"event_id"` and `"seq"` fields
- SSE handler uses `event_id` as SSE `id:` field for reconnect support

#### Phase 3: Add correlation_id + causation_id (backend)

**Scope:** Causal chain tracking.

Changes:
- `BaseEvent` gets `correlationID string` + `causationID string`
- Core run: `correlationID = runID`, `causationID = ""`
- Subagent spawn: propagate parent's `correlationID`, set `causationID = trigger call_id`
- Context propagation: `WithCorrelationID(ctx)`, `WithCausationID(ctx)`
- `sse_render.go` emits `"correlation_id"` and `"causation_id"`

Subagent tool executor changes:
- When spawning child agent, pass `causationID = call.ID` to child's base event factory
- Child's `correlationID` = parent's `correlationID`

#### Phase 4: Frontend adaptation (single commit)

**Scope:** Update all frontend code to new field names and grouping logic.

Changes:
- `web/lib/types/events/base.ts` — `WorkflowEnvelope` fields: `run_id`, `parent_run_id`, `correlation_id`, `causation_id`, `event_id`, `seq`
- `web/lib/subagent.ts` — use `parent_run_id` instead of `parent_task_id`
- `web/components/agent/ConversationEventStream.tsx`:
  - Subagent thread key: `causation_id + ":" + run_id` (deterministic batch grouping)
  - Trigger matching: `payload.call_id === thread.causation_id`
  - No more heuristic parent_task_id fallback
- `web/hooks/useAgentStreamStore.ts` — dedup by `event_id` instead of timestamp heuristic
- SSE reconnect: send `Last-Event-ID` header with latest `event_id`

#### Phase 5: Cleanup

- Remove `agentLevel` field from BaseEvent (derive from `parentRunID != ""`)
- Remove old `isSubagentTrigger()` heuristic
- Remove frontend workaround code (unassigned group fallback, etc.)
- Update `docs/plans/backend-event-id-analysis.md` to reference this plan

---

## 5. Key Design Decisions

### D1: `runID` instead of keeping `taskID`

**Rationale:** `taskID` conflates two concepts — "the task the user gave" (which may be retried) and "this specific execution". Industry uniformly uses `run_id` for the execution instance. The user-facing task is already tracked in the HTTP API layer (`/api/tasks/{id}`), not in the event stream.

### D2: `causation_id = call_id` (not a separate invocation_id)

**Rationale:** The `call_id` already uniquely identifies each subagent tool invocation. Using it directly as `causation_id` avoids introducing yet another ID. This follows the event sourcing pattern where `causation_id = trigger_event.message_id`, and in our case the tool call's `call_id` is the closest analogue.

### D3: No namespace path (LangGraph-style `checkpoint_ns`)

**Rationale:** Our system currently limits nesting to 2 levels (core + subagent), with `MarkSubagentContext` preventing recursion. A namespace path is over-engineering for now. If we support deeper nesting later, `correlation_id + causation_id` chain already enables tree reconstruction without a dedicated path field.

### D4: `seq` as uint64 atomic counter per run

**Rationale:** Timestamp alone is insufficient for ordering concurrent events within a run (parallel tool calls). A monotonic sequence gives deterministic ordering. Per-run scope keeps the counter small and avoids cross-run coordination.

### D5: No `agentLevel` in target state (Phase 5)

**Rationale:** `agentLevel` is derivable from `parentRunID != ""`. Removing it eliminates a field that can go out of sync. Frontend can derive it the same way.

---

## 6. Validation Criteria

- [ ] All existing tests pass after Phase 1 rename
- [ ] SSE output contains `run_id` / `parent_run_id` instead of `task_id` / `parent_task_id`
- [ ] `event_id` is unique across all events in a session (test with parallel subagents)
- [ ] `seq` is strictly monotonic within a run
- [ ] Subagent events carry correct `causation_id` = trigger's `call_id`
- [ ] Frontend groups subagent threads by `causation_id` — multi-batch scenario works
- [ ] SSE reconnect with `Last-Event-ID` replays correctly
- [ ] No remaining references to `taskID` / `parentTaskID` / `task_id` / `parent_task_id` in codebase

---

## References

- [OpenAI Assistants API — Runs](https://platform.openai.com/docs/api-reference/runs)
- [OpenAI Agents SDK — Tracing](https://openai.github.io/openai-agents-python/tracing/)
- [LangGraph — Streaming Concepts](https://github.com/langchain-ai/langgraph/blob/main/docs/docs/concepts/streaming.md)
- [Temporal — Workflow ID and Run ID](https://docs.temporal.io/workflow-execution/workflowid-runid)
- [OTel GenAI — Agent Spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/)
- [Arkency — Correlation ID and Causation ID](https://blog.arkency.com/correlation-id-and-causation-id-in-evented-systems/)
- [Confluent — Event-Driven Multi-Agent Systems](https://www.confluent.io/blog/event-driven-multi-agent-systems/)
