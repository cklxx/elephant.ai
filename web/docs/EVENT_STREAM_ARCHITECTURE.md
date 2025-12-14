# Event Stream State Management Architecture

## Overview

This document describes the robust event stream state management system for the ALEX web UI. The architecture addresses memory accumulation, performance, and research step tracking using Zustand + Immer for state management and TanStack Virtual for efficient rendering.

## Workflow-first Event IDL (migration spec)

The goal is a single workflow-originated event stream with semantic, namespaced `event_type` values. Every message uses a shared envelope and an explicit version for compatibility.

- **Envelope (all events)**: `version` (e.g., `1`), `event_type`, `timestamp` (RFC3339), `agent_level`, `workflow_id`, `run_id` (or `task_id`), `parent_task_id`, `session_id`, `node_id`, `node_kind`, `payload`, optional `attachments`.
- **Namespaces** (examples):
  - `workflow.lifecycle.updated` – full workflow snapshot (phase, summary, nodes).
  - `workflow.node.started|completed|failed` – node lifecycle (replaces `step_started/step_completed`).
  - `workflow.node.output.delta` – token stream (replaces `workflow.node.output.delta`).
  - `workflow.node.output.summary` – completed turn/thought (replaces `workflow.node.output.summary`).
  - `workflow.tool.started|progress|completed` – tool runs (replaces `workflow.tool.started/stream/complete`).
  - `workflow.subflow.progress|completed` – delegated agent/subagent aggregation (replaces frontend-synthesized `subagent_*`).
  - `workflow.result.final` – final answer/attachments (replaces `workflow.result.final`).
  - `workflow.result.cancelled` – cancellation (replaces `workflow.result.cancelled`).
  - `workflow.diagnostic.*` – `workflow.diagnostic.context_compression`, `workflow.diagnostic.context_snapshot`, `workflow.diagnostic.tool_filtering`, `workflow.diagnostic.environment_snapshot`.
- **Compatibility strategy**:
  - No legacy dual-emission; only workflow.* event types are streamed. Tracking/analytics lists must mirror the new names; tests enforce parity (see `internal/analytics/tracking_plan_test.go`).

### Field conventions

- `workflow_id` is stable for the run; `run_id` may alias `task_id` until the runtime supplies a dedicated ID.
- `node_id`/`node_kind` describe the workflow node (e.g., `prepare`, `execute`, `tool`, `delegate`).
- `agent_level` remains `core|subagent`; delegated streams still carry `parent_task_id`.
- Payloads keep type-specific fields but avoid implicit inference; streaming deltas (tokens/final answer) must be additive or include `stream_finished`.

### Required backend emissions (to align with UI expectations)

- Emit `workflow.subflow.progress|completed` from the delegating agent instead of frontend synthesis.
- Emit `workflow.tool.progress` for streaming tool output instead of overloading `workflow.tool.progress`.
- Ensure diagnostics (`workflow.diagnostic.context_compression`, `workflow.diagnostic.context_snapshot`, `workflow.diagnostic.tool_filtering`, `workflow.diagnostic.environment_snapshot`) use the `workflow.diagnostic.*` namespace.
## Cleanup and validation (post-migration checklist)

- **Remove legacy-only code**: drop `event_type` inference branches in `web/lib/schemas.ts`; delete `subagentDeriver` once backend emits subflow events.
- **Schema/test parity**: keep Go/TS event lists in sync (`internal/analytics/tracking_plan_test.go` ↔ `web/lib/schemas.ts`/`types.ts`); add a golden SSE sample covering every new `event_type`.
- **E2E validation**: run SSE → store → UI flow with a scripted event fixture (node lifecycle, tool stream, subflow, diagnostics, final/cancel). Confirm rendering and aggregation paths (iterations, tools, steps, result).
- **Telemetry**: update analytics/tracking plan to the new names; remove legacy labels once dual-read window closes.
- **Docs**: keep this spec as the single source; reference it from implementation notes/PRs to avoid drift.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         SSE Connection                           │
│                      (Server-Sent Events)                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     useSSE Hook                                  │
│  • Connection management                                         │
│  • Auto-reconnection with exponential backoff                    │
│  • Event parsing and validation                                  │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              useAgentStreamStore (Zustand + Immer)               │
│  ┌───────────────────────────────────────────────────┐           │
│  │           EventLRUCache (max 1000 events)        │           │
│  │  • Automatic eviction of oldest events           │           │
│  │  • Memory-bounded storage                        │           │
│  └───────────────────────────────────────────────────┘           │
│                            │                                      │
│                            ▼                                      │
│  ┌───────────────────────────────────────────────────┐           │
│  │         Event Aggregation Layer                  │           │
│  │  • aggregateToolCalls()                          │           │
│  │  • groupByIteration()                            │           │
│  │  • extractResearchSteps()                        │           │
│  └───────────────────────────────────────────────────┘           │
│                            │                                      │
│                            ▼                                      │
│  ┌───────────────────────────────────────────────────┐           │
│  │         Computed State Structures                │           │
│  │  • toolCalls: Map<string, AggregatedToolCall>   │           │
│  │  • iterations: Map<number, IterationGroup>      │           │
│  │  • researchSteps: ResearchStep[]                │           │
│  └───────────────────────────────────────────────────┘           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Store Selectors                                │
│  • useCurrentResearchStep()                                      │
│  • useCompletedResearchSteps()                                   │
│  • useActiveToolCall()                                           │
│  • useIterationToolCalls(iteration)                              │
│  • useCurrentIteration()                                         │
│  • useErrorStates()                                              │
│  • useTaskSummary()                                              │
│  • useMemoryStats()                                              │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              VirtualizedEventList Component                      │
│  • TanStack Virtual for windowed rendering                       │
│  • Renders only visible items (~10-15 at a time)                 │
│  • Smooth scrolling with auto-scroll to bottom                   │
│  • Dynamic height measurement                                    │
└─────────────────────────────────────────────────────────────────┘
```

## Event Aggregation Logic

### 1. Tool Call Aggregation

**Purpose**: Merge `workflow.tool.started`, `workflow.tool.progress`, and `workflow.tool.completed` events (legacy: `workflow.tool.started/stream/complete`) into a single `AggregatedToolCall` object.

**Algorithm**:
```typescript
interface AggregatedToolCall {
  id: string;
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
  status: 'running' | 'streaming' | 'complete' | 'error';
  stream_chunks: string[];
  result?: string;
  error?: string;
  duration?: number;
  timestamp: string;
  iteration: number;
}

// Process:
// 1. workflow.tool.started → Create entry with status='running'
// 2. workflow.tool.progress → Append to stream_chunks, status='streaming'
// 3. workflow.tool.completed → Update status, set result/error, duration
```

**Benefits**:
- Single card per tool execution (not 3 separate cards)
- Stream chunks accumulated for progressive rendering
- Clear status tracking (running → streaming → complete/error)

### 2. Iteration Grouping

**Purpose**: Group all events within a single ReAct iteration into `IterationGroup`.

**Algorithm**:
```typescript
interface IterationGroup {
  id: string;
  iteration: number;
  total_iters: number;
  status: 'running' | 'complete';
  started_at: string;
  completed_at?: string;
  workflow.node.output.delta?: string;
  tool_calls: AggregatedToolCall[];
  tokens_used?: number;
  tools_run?: number;
  errors: string[];
}

// Process:
// 1. workflow.node.started → Create group with status='running'
// 2. workflow.node.output.summary → Add workflow.node.output.delta content
// 3. tool_call_* → Add to tool_calls array
// 4. error → Append to errors array
// 5. workflow.node.completed → Update status, metrics
```

**Benefits**:
- Collapsible iteration view (show/hide details)
- Timeline visualization (iteration progress)
- Metrics rollup (tokens, tool count)

### 3. Research Step Extraction

**Purpose**: Track high-level research plan and step completion.

**Algorithm**:
```typescript
interface ResearchStep {
  id: string;
  step_index: number;
  description: string;
  status: 'pending' | 'in_progress' | 'completed';
  started_at?: string;
  completed_at?: string;
  result?: string;
  iterations: number[];
}

// Process:
// 1. workflow.node.started → Update status to 'in_progress'
// 2. workflow.node.completed → Update status to 'completed', add result
```

**Benefits**:
- research console-style timeline
- Step progress tracking
- Iteration-to-step mapping

## Virtual Scrolling Performance

### Configuration

```typescript
const virtualizer = useVirtualizer({
  count: events.length,           // Total items
  getScrollElement: () => parentRef.current,
  estimateSize: () => 200,        // Estimated height per item (px)
  overscan: 5,                    // Render 5 extra items above/below viewport
});
```

### Performance Metrics (Estimated)

| Metric | Without Virtualization | With Virtualization |
|--------|------------------------|---------------------|
| **DOM Nodes (1000 events)** | ~15,000 nodes | ~150 nodes (10x visible) |
| **Memory Usage** | ~50MB | ~5MB |
| **Render Time** | ~500ms | ~50ms |
| **Scroll FPS** | 20-30 FPS | 60 FPS |
| **Time to Interactive** | 2-3s | 200-300ms |

### Key Optimizations

1. **Windowed Rendering**: Only render visible items + overscan buffer
2. **Dynamic Measurement**: Automatically measure actual item heights
3. **Smooth Scrolling**: Hardware-accelerated transforms
4. **Auto-scroll**: Maintain position on new events

## Memory Usage Analysis

### LRU Cache (1000 Events)

```typescript
class EventLRUCache {
  private maxSize = 1000;
  private events: AnyAgentEvent[] = [];

  add(event: AnyAgentEvent): void {
    this.events.push(event);
    if (this.events.length > this.maxSize) {
      const evictCount = this.events.length - this.maxSize;
      this.events.splice(0, evictCount); // Remove oldest
    }
  }
}
```

### Memory Breakdown

| Component | Memory per Item | Total (1000 events) |
|-----------|-----------------|---------------------|
| **Raw Events** | ~500 bytes | ~500 KB |
| **Aggregated Tool Calls** | ~300 bytes | ~30 KB (100 calls) |
| **Iteration Groups** | ~1 KB | ~10 KB (10 iterations) |
| **Research Steps** | ~200 bytes | ~2 KB (10 steps) |
| **Browser Diagnostics** | ~1 KB (connection metadata) | ~10 KB (10 diagnostics) |
| **Total Estimated** | - | **~650 KB** |

### Eviction Strategy

- **Hard limit**: 1000 events maximum
- **FIFO eviction**: Oldest events removed first
- **Atomic operations**: Batch additions avoid thrashing
- **Recomputation**: Aggregations rebuild after eviction

## Migration Guide

### For Existing Components

#### Before (Direct useSSE)

```typescript
import { useSSE } from '@/hooks/useSSE';

function MyComponent({ sessionId }) {
  const { events, isConnected, error } = useSSE(sessionId);

  return (
    <div>
      {events.map((event, idx) => (
        <EventCard key={idx} event={event} />
      ))}
    </div>
  );
}
```

#### After (Store-based with Selectors)

```typescript
import { useAgentStreamIntegration } from '@/hooks/useAgentStreamIntegration';
import { useCompletedIterations, useActiveToolCall } from '@/hooks/useAgentStreamStore';

function MyComponent({ sessionId }) {
  const { events, isConnected, error } = useAgentStreamIntegration(sessionId);
  const completedIterations = useCompletedIterations();
  const activeToolCall = useActiveToolCall();

  return (
    <div>
      {/* Use virtualized list for better performance */}
      <VirtualizedEventList events={events} />

      {/* Or access aggregated data directly */}
      <div>
        <h3>Active Tool: {activeToolCall?.tool_name}</h3>
        <p>Completed Iterations: {completedIterations.length}</p>
      </div>
    </div>
  );
}
```

### For New Components (Direct Store Access)

```typescript
import {
  useCurrentResearchStep,
  useCompletedResearchSteps,
  useTaskSummary,
  useMemoryStats,
} from '@/hooks/useAgentStreamStore';

function TimelineStepListExample() {
  const currentStep = useCurrentResearchStep();
  const completedSteps = useCompletedResearchSteps();
  const taskSummary = useTaskSummary();

  return (
    <div>
      <h2>{taskSummary.actionName}</h2>
      <p>{taskSummary.goal}</p>

      {completedSteps.map((step) => (
        <div key={step.id}>
          <h3>✅ {step.description}</h3>
          <p>{step.result}</p>
        </div>
      ))}

      {currentStep && (
        <div>
          <h3>🔄 {currentStep.description}</h3>
        </div>
      )}
    </div>
  );
}
```

### Breaking Changes

**None**. The system is backwards compatible:

1. `useSSE` hook still works as before
2. Existing `AgentOutput` component upgraded internally
3. New store provides opt-in enhancements
4. Components can migrate incrementally

### Recommended Migration Path

1. **Phase 1**: Keep existing `useSSE` usage, add store integration
2. **Phase 2**: Migrate performance-critical components to virtual scrolling
3. **Phase 3**: Add research step timeline components
4. **Phase 4**: Full migration to store-based selectors

## Backend Integration

### Required Backend Changes (Future)

To emit new event types, update Go backend:

**File**: `internal/agent/domain/events.go`

```go
type WorkflowNodeStartedEvent struct {
    BaseEvent
    StepIndex       int    `json:"step_index"`
    StepDescription string `json:"step_description"`
    Iteration       int    `json:"iteration,omitempty"`
}

type WorkflowNodeCompletedEvent struct {
    BaseEvent
    StepIndex       int    `json:"step_index"`
    StepResult      string `json:"step_result"`
    StepDescription string `json:"step_description,omitempty"`
    Iteration       int    `json:"iteration,omitempty"`
}
```

### SSE Event Registration

**File**: `cmd/server/handlers/sse.go`

```go
// Add new event types to SSE stream
eventTypes := []string{
    "connected",
    "workflow.lifecycle.updated",
    "workflow.node.started",
    "workflow.node.completed",
    "workflow.node.failed",
    "workflow.node.output.delta",
    "workflow.node.output.summary",
    "workflow.tool.started",
    "workflow.tool.progress",
    "workflow.tool.completed",
    "workflow.subflow.progress",
    "workflow.subflow.completed",
    "workflow.result.final",
    "workflow.result.cancelled",
    "workflow.diagnostic.environment_snapshot",
    "workflow.diagnostic.context_compression",
    "workflow.diagnostic.tool_filtering",
    "workflow.diagnostic.context_snapshot",
    // Transitional legacy registrations (remove after dual-emission window)
    "workflow.tool.started",
    "workflow.tool.progress",
    "workflow.tool.completed",
}
```

## Testing

### Unit Tests

```typescript
// Test event aggregation
describe('aggregateToolCalls', () => {
  it('should merge start, stream, and complete events', () => {
    const events = [
      { event_type: 'workflow.tool.started', call_id: '123', tool_name: 'bash', ... },
      { event_type: 'workflow.tool.progress', call_id: '123', chunk: 'output', ... },
      { event_type: 'workflow.tool.completed', call_id: '123', result: 'done', ... },
    ];

    const result = aggregateToolCalls(events);
    expect(result.get('123')).toMatchObject({
      tool_name: 'bash',
      status: 'complete',
      stream_chunks: ['output'],
      result: 'done',
    });
  });
});

// Test LRU eviction
describe('EventLRUCache', () => {
  it('should evict oldest events when over limit', () => {
    const cache = new EventLRUCache(3);
    cache.add({ id: 1 });
    cache.add({ id: 2 });
    cache.add({ id: 3 });
    cache.add({ id: 4 }); // Should evict id: 1

    expect(cache.getAll()).toHaveLength(3);
    expect(cache.getAll()[0].id).toBe(2);
  });
});
```

### Integration Tests

```typescript
// Test store with SSE integration
describe('useAgentStreamIntegration', () => {
  it('should sync SSE events to store', async () => {
    const { result } = renderHook(() => useAgentStreamIntegration('session-123'));

    // Simulate SSE event
    act(() => {
      const event = { event_type: 'workflow.node.started', iteration: 1, total_iters: 3, ... };
      result.current.onEvent(event);
    });

    const storeState = useAgentStreamStore.getState();
    expect(storeState.currentIteration).toBe(1);
  });
});
```

## Performance Monitoring

### Built-in Memory Stats

```typescript
const memoryStats = useMemoryStats();
console.log({
  eventCount: memoryStats.eventCount,
  estimatedBytes: memoryStats.estimatedBytes,
  toolCallCount: memoryStats.toolCallCount,
  iterationCount: memoryStats.iterationCount,
});
```

### Chrome DevTools Integration

```javascript
// Add to browser console
window.ALEX_DEBUG = {
  getStoreState: () => useAgentStreamStore.getState(),
  getMemoryStats: () => useAgentStreamStore.getState().eventCache.getMemoryUsage(),
  clearEvents: () => useAgentStreamStore.getState().clearEvents(),
};
```

## Future Enhancements

1. **Persistence**: LocalStorage backup for event history
2. **Export**: JSON/CSV export for analysis
3. **Search**: Full-text search across events
4. **Filters**: Filter by event type, iteration, tool
5. **Time Travel**: Replay events step-by-step
6. **Compression**: LZ-string compression for older events
7. **Chunking**: Pagination for very long sessions (>10k events)

## References

- **Zustand**: https://github.com/pmndrs/zustand
- **Immer**: https://immerjs.github.io/immer/
- **TanStack Virtual**: https://tanstack.com/virtual/latest
- **ALEX Architecture**: `docs/architecture/ALEX_DETAILED_ARCHITECTURE.md`
