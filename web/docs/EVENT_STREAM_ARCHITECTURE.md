# Event Stream State Management Architecture

## Overview

This document describes the robust event stream state management system for the ALEX web UI. The architecture addresses memory accumulation, performance, and research step tracking using Zustand + Immer for state management and TanStack Virtual for efficient rendering.

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
│  │  • extractBrowserSnapshots()                     │           │
│  └───────────────────────────────────────────────────┘           │
│                            │                                      │
│                            ▼                                      │
│  ┌───────────────────────────────────────────────────┐           │
│  │         Computed State Structures                │           │
│  │  • toolCalls: Map<string, AggregatedToolCall>   │           │
│  │  • iterations: Map<number, IterationGroup>      │           │
│  │  • researchSteps: ResearchStep[]                │           │
│  │  • browserSnapshots: BrowserSnapshot[]          │           │
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

**Purpose**: Merge `tool_call_start`, `tool_call_stream`, and `tool_call_complete` events into a single `AggregatedToolCall` object.

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
// 1. tool_call_start → Create entry with status='running'
// 2. tool_call_stream → Append to stream_chunks, status='streaming'
// 3. tool_call_complete → Update status, set result/error, duration
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
  thinking?: string;
  tool_calls: AggregatedToolCall[];
  tokens_used?: number;
  tools_run?: number;
  errors: string[];
}

// Process:
// 1. iteration_start → Create group with status='running'
// 2. think_complete → Add thinking content
// 3. tool_call_* → Add to tool_calls array
// 4. error → Append to errors array
// 5. iteration_complete → Update status, metrics
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
// 1. research_plan → Create steps from plan_steps array
// 2. step_started → Update status to 'in_progress'
// 3. step_completed → Update status to 'completed', add result
```

**Benefits**:
- Manus-style research timeline
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
| **Browser Snapshots** | ~10 KB (with base64 image) | ~100 KB (10 snapshots) |
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

function ResearchTimeline() {
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
// Add new event types
type ResearchPlanEvent struct {
    BaseEvent
    PlanSteps          []string `json:"plan_steps"`
    EstimatedIterations int     `json:"estimated_iterations"`
}

type StepStartedEvent struct {
    BaseEvent
    StepIndex       int    `json:"step_index"`
    StepDescription string `json:"step_description"`
}

type StepCompletedEvent struct {
    BaseEvent
    StepIndex  int    `json:"step_index"`
    StepResult string `json:"step_result"`
}

type BrowserSnapshotEvent struct {
    BaseEvent
    URL            string `json:"url"`
    ScreenshotData string `json:"screenshot_data,omitempty"` // base64
    HTMLPreview    string `json:"html_preview,omitempty"`
}
```

**File**: `internal/agent/domain/react_engine.go`

```go
// Emit research plan before execution
func (e *ReactEngine) SolveTask(ctx context.Context, task string) error {
    // Analyze and create plan
    plan := e.analyzeTask(ctx, task)
    e.eventBus.Emit(ResearchPlanEvent{
        PlanSteps: plan.Steps,
        EstimatedIterations: plan.EstimatedIterations,
    })

    // Execute plan steps
    for i, step := range plan.Steps {
        e.eventBus.Emit(StepStartedEvent{
            StepIndex: i,
            StepDescription: step,
        })

        result := e.executeStep(ctx, step)

        e.eventBus.Emit(StepCompletedEvent{
            StepIndex: i,
            StepResult: result,
        })
    }
}
```

### SSE Event Registration

**File**: `cmd/server/handlers/sse.go`

```go
// Add new event types to SSE stream
eventTypes := []string{
    "task_analysis",
    "iteration_start",
    "thinking",
    "think_complete",
    "tool_call_start",
    "tool_call_stream",
    "tool_call_complete",
    "iteration_complete",
    "task_complete",
    "error",
    // New event types
    "research_plan",
    "step_started",
    "step_completed",
    "browser_snapshot",
}
```

## Testing

### Unit Tests

```typescript
// Test event aggregation
describe('aggregateToolCalls', () => {
  it('should merge start, stream, and complete events', () => {
    const events = [
      { event_type: 'tool_call_start', call_id: '123', tool_name: 'bash', ... },
      { event_type: 'tool_call_stream', call_id: '123', chunk: 'output', ... },
      { event_type: 'tool_call_complete', call_id: '123', result: 'done', ... },
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
      const event = { event_type: 'task_analysis', action_name: 'Test', ... };
      result.current.onEvent(event);
    });

    // Check store state
    const storeState = useAgentStreamStore.getState();
    expect(storeState.taskAnalysis.action_name).toBe('Test');
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
