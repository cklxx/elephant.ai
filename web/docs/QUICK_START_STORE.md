# Quick Start: Agent Stream Store

## Basic Usage

### 1. Import the Integration Hook

```typescript
import { useAgentStreamIntegration } from '@/hooks/useAgentStreamIntegration';

function MyComponent({ sessionId }) {
  const {
    events,           // All events (for backwards compatibility)
    isConnected,      // SSE connection status
    isReconnecting,   // Reconnection in progress
    error,           // Connection error
    clearEvents,     // Clear all events
  } = useAgentStreamIntegration(sessionId);

  return <VirtualizedEventList events={events} />;
}
```

### 2. Use Store Selectors for Specific Data

```typescript
import {
  useCurrentResearchStep,
  useCompletedIterations,
  useActiveToolCall,
  useTaskSummary,
} from '@/hooks/useAgentStreamStore';

function TaskHeader() {
  const summary = useTaskSummary();

  return (
    <div>
      <h1>{summary.actionName}</h1>
      <p>{summary.goal}</p>
      <span>Iteration {summary.currentIteration} / {summary.totalIterations}</span>
    </div>
  );
}

function ActiveTool() {
  const toolCall = useActiveToolCall();

  if (!toolCall) return null;

  return (
    <div>
      <span>Running: {toolCall.tool_name}</span>
      {toolCall.stream_chunks.map((chunk, i) => (
        <pre key={i}>{chunk}</pre>
      ))}
    </div>
  );
}

function ResearchProgress() {
  const currentStep = useCurrentResearchStep();
  const completedIterations = useCompletedIterations();

  return (
    <div>
      {currentStep && (
        <div>Current Step: {currentStep.description}</div>
      )}
      <div>Completed: {completedIterations.length} iterations</div>
    </div>
  );
}
```

### 3. Add Virtual Scrolling

```typescript
import { VirtualizedEventList } from '@/components/agent/VirtualizedEventList';

function EventStream({ sessionId }) {
  const { events } = useAgentStreamIntegration(sessionId);

  return (
    <VirtualizedEventList
      events={events}
      autoScroll={true}  // Auto-scroll to bottom
    />
  );
}
```

## Available Selectors

| Selector | Returns | Use Case |
|----------|---------|----------|
| `useCurrentResearchStep()` | `ResearchStep \| null` | Show active research step |
| `useCompletedResearchSteps()` | `ResearchStep[]` | Timeline of completed steps |
| `useInProgressResearchSteps()` | `ResearchStep[]` | Currently executing steps |
| `useActiveToolCall()` | `AggregatedToolCall \| null` | Show currently running tool |
| `useIterationToolCalls(n)` | `AggregatedToolCall[]` | Get all tools for iteration N |
| `useCurrentIteration()` | `IterationGroup \| null` | Current iteration details |
| `useCompletedIterations()` | `IterationGroup[]` | All completed iterations |
| `useErrorStates()` | `{ hasError, errorMessage, ... }` | Error tracking |
| `useTaskSummary()` | `{ actionName, goal, status, ... }` | Task metadata |
| `useMemoryStats()` | `{ eventCount, estimatedBytes, ... }` | Memory usage |
| `useLatestBrowserDiagnostics()` | `BrowserDiagnostics \| null` | Latest browser diagnostics |
| `useIterationsArray()` | `IterationGroup[]` | Sorted iterations (for virtualizer) |
| `useRawEvents()` | `AnyAgentEvent[]` | Raw event array (debug/export) |

## Memory Management

The store automatically limits events to **1000 maximum**:

```typescript
const memoryStats = useMemoryStats();

console.log(memoryStats.eventCount);      // Current event count
console.log(memoryStats.estimatedBytes);  // ~500KB for 1000 events
```

### Manual Event Clearing

```typescript
import { useAgentStreamStore } from '@/hooks/useAgentStreamStore';

function ClearButton() {
  const clearEvents = useAgentStreamStore(state => state.clearEvents);

  return <button onClick={clearEvents}>Clear Events</button>;
}
```

## Example: Timeline Step List Component

```typescript
import {
  useTaskSummary,
  useCompletedResearchSteps,
  useCurrentResearchStep,
} from '@/hooks/useAgentStreamStore';

function TimelineStepListExample() {
  const { actionName, goal, status } = useTaskSummary();
  const completedSteps = useCompletedResearchSteps();
  const currentStep = useCurrentResearchStep();

  return (
    <div className="timeline">
      <h2>{actionName}</h2>
      <p>{goal}</p>

      {/* Completed steps */}
      {completedSteps.map((step) => (
        <div key={step.id} className="step completed">
          <h3>✅ Step {step.step_index + 1}: {step.description}</h3>
          <p>{step.result}</p>
          <small>
            Started: {new Date(step.started_at).toLocaleTimeString()}
            - Completed: {new Date(step.completed_at).toLocaleTimeString()}
          </small>
        </div>
      ))}

      {/* Current step */}
      {currentStep && (
        <div className="step in-progress">
          <h3>🔄 Step {currentStep.step_index + 1}: {currentStep.description}</h3>
          <p>In progress...</p>
        </div>
      )}

      {/* Status indicator */}
      <div className="status">
        Status: {status}
      </div>
    </div>
  );
}
```

## Example: Iteration Details with Tool Calls

```typescript
import { useIterationToolCalls, useCompletedIterations } from '@/hooks/useAgentStreamStore';

function IterationList() {
  const iterations = useCompletedIterations();

  return (
    <div>
      {iterations.map((iter) => (
        <IterationCard key={iter.id} iteration={iter} />
      ))}
    </div>
  );
}

function IterationCard({ iteration }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="iteration-card">
      <div onClick={() => setExpanded(!expanded)}>
        <h3>Iteration {iteration.iteration} / {iteration.total_iters}</h3>
        <p>Tokens: {iteration.tokens_used}, Tools: {iteration.tools_run}</p>
      </div>

      {expanded && (
        <div>
          {/* Thinking */}
          {iteration.thinking && (
            <div className="thinking">
              <h4>Thinking:</h4>
              <p>{iteration.thinking}</p>
            </div>
          )}

          {/* Tool calls */}
          {iteration.tool_calls.map((toolCall) => (
            <div key={toolCall.id} className="tool-call">
              <h4>{toolCall.tool_name}</h4>
              <pre>{JSON.stringify(toolCall.arguments, null, 2)}</pre>
              {toolCall.result && <pre>{toolCall.result}</pre>}
              {toolCall.error && <div className="error">{toolCall.error}</div>}
            </div>
          ))}

          {/* Errors */}
          {iteration.errors.length > 0 && (
            <div className="errors">
              {iteration.errors.map((err, i) => (
                <div key={i} className="error">{err}</div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

## Performance Tips

1. **Use Specific Selectors**: Don't subscribe to entire store
   ```typescript
   // ❌ Bad - subscribes to entire store
   const store = useAgentStreamStore();

   // ✅ Good - subscribes only to taskSummary
   const taskSummary = useTaskSummary();
   ```

2. **Virtualize Long Lists**: Always use `VirtualizedEventList` for event rendering

3. **Memoize Expensive Computations**: Use `useMemo` for derived data
   ```typescript
   const errorCount = useMemo(() => {
     return iterations.filter(i => i.errors.length > 0).length;
   }, [iterations]);
   ```

4. **Monitor Memory**: Check `useMemoryStats()` in development

## Debugging

### Store State Inspector

```typescript
// Add to any component
useEffect(() => {
  console.log('Store State:', useAgentStreamStore.getState());
}, []);
```

### Event Tracer

```typescript
import { useAgentStreamStore } from '@/hooks/useAgentStreamStore';

useEffect(() => {
  const unsubscribe = useAgentStreamStore.subscribe((state, prevState) => {
    console.log('Event added:', state.eventCache.getAll().slice(-1)[0]);
  });
  return unsubscribe;
}, []);
```

## Migration Checklist

- [ ] Install dependencies: `@tanstack/react-virtual`, `immer`
- [ ] Replace `useSSE` with `useAgentStreamIntegration`
- [ ] Update event list to use `VirtualizedEventList`
- [ ] Add memory stats display to header
- [ ] Create research timeline component (optional)
- [ ] Update backend to emit new event types (future)
