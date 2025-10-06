// Agent event stream state management with Zustand + Immer
// Features:
// - LRU event caching (max 1000 events)
// - Event aggregation (tool calls, iterations, research steps)
// - Browser snapshot tracking
// - Performant selectors for UI components

import { create } from 'zustand';
import { AnyAgentEvent } from '@/lib/types';
import {
  EventLRUCache,
  aggregateToolCalls,
  groupByIteration,
  extractResearchSteps,
  extractBrowserSnapshots,
  AggregatedToolCall,
  IterationGroup,
  ResearchStep,
  BrowserSnapshot,
} from '@/lib/eventAggregation';

/**
 * Store state shape
 */
interface AgentStreamState {
  // Raw event storage with LRU eviction
  eventCache: EventLRUCache;

  // Aggregated data structures (computed from events)
  toolCalls: Map<string, AggregatedToolCall>;
  iterations: Map<number, IterationGroup>;
  researchSteps: ResearchStep[];
  browserSnapshots: BrowserSnapshot[];

  // Current context tracking
  currentIteration: number | null;
  currentResearchStep: number | null;
  activeToolCall: string | null;

  // Task metadata
  taskAnalysis: {
    action_name?: string;
    goal?: string;
  };
  taskStatus: 'idle' | 'analyzing' | 'running' | 'completed' | 'error';
  finalAnswer?: string;
  totalIterations?: number;
  totalTokens?: number;
  errorMessage?: string;

  // Actions
  addEvent: (event: AnyAgentEvent) => void;
  addEvents: (events: AnyAgentEvent[]) => void;
  clearEvents: () => void;
  recomputeAggregations: () => void;
}

/**
 * Create Zustand store with Immer middleware for immutable updates
 */
// Create event cache outside of Zustand store
const globalEventCache = new EventLRUCache(1000);

export const useAgentStreamStore = create<AgentStreamState>()((set, get) => ({
    // Initial state
    eventCache: globalEventCache,
    toolCalls: new Map(),
    iterations: new Map(),
    researchSteps: [],
    browserSnapshots: [],
    currentIteration: null,
    currentResearchStep: null,
    activeToolCall: null,
    taskAnalysis: {},
    taskStatus: 'idle',

    // Add single event
    addEvent: (event: AnyAgentEvent) => {
      // Add to global event cache
      globalEventCache.add(event);

      const state = get();
      const updates: Partial<AgentStreamState> = {};

      // Update task status based on event type
      switch (event.event_type) {
        case 'task_analysis':
          updates.taskStatus = 'analyzing';
          updates.taskAnalysis = {
            action_name: event.action_name,
            goal: event.goal,
          };
          break;

        case 'iteration_start':
          updates.taskStatus = 'running';
          updates.currentIteration = event.iteration;
          break;

        case 'tool_call_start':
          updates.activeToolCall = event.call_id;
          break;

        case 'tool_call_complete':
          if (state.activeToolCall === event.call_id) {
            updates.activeToolCall = null;
          }
          break;

        case 'task_complete':
          updates.taskStatus = 'completed';
          updates.finalAnswer = event.final_answer;
          updates.totalIterations = event.total_iterations;
          updates.totalTokens = event.total_tokens;
          updates.currentIteration = null;
          updates.activeToolCall = null;
          break;

        case 'error':
          updates.taskStatus = 'error';
          updates.errorMessage = event.error;
          break;

        case 'step_started':
          updates.currentResearchStep = event.step_index;
          break;

        case 'step_completed':
          if (state.currentResearchStep === event.step_index) {
            updates.currentResearchStep = null;
          }
          break;
      }

      // Recompute aggregations
      const allEvents = globalEventCache.getAll();
      updates.toolCalls = aggregateToolCalls(allEvents);
      updates.iterations = groupByIteration(allEvents);
      updates.researchSteps = extractResearchSteps(allEvents);
      updates.browserSnapshots = extractBrowserSnapshots(allEvents);

      set(updates);
    },

    // Add multiple events (batch operation)
    addEvents: (events: AnyAgentEvent[]) => {
      // Add to global event cache
      globalEventCache.addMany(events);

      // Recompute aggregations
      const allEvents = globalEventCache.getAll();
      set({
        toolCalls: aggregateToolCalls(allEvents),
        iterations: groupByIteration(allEvents),
        researchSteps: extractResearchSteps(allEvents),
        browserSnapshots: extractBrowserSnapshots(allEvents),
      });
    },

    // Clear all events and reset state
    clearEvents: () => {
      // Clear global event cache
      globalEventCache.clear();

      set({
        toolCalls: new Map(),
        iterations: new Map(),
        researchSteps: [],
        browserSnapshots: [],
        currentIteration: null,
        currentResearchStep: null,
        activeToolCall: null,
        taskAnalysis: {},
        taskStatus: 'idle',
        finalAnswer: undefined,
        totalIterations: undefined,
        totalTokens: undefined,
        errorMessage: undefined,
      });
    },

    // Recompute all aggregations from raw events
    recomputeAggregations: () => {
      const events = globalEventCache.getAll();

      set({
        toolCalls: aggregateToolCalls(events),
        iterations: groupByIteration(events),
        researchSteps: extractResearchSteps(events),
        browserSnapshots: extractBrowserSnapshots(events),
      });
    },
  }));

/**
 * Selectors for efficient component access
 */

// Get current research step (if any)
export const useCurrentResearchStep = () => {
  return useAgentStreamStore((state) => {
    if (state.currentResearchStep === null) return null;
    return state.researchSteps.find((s) => s.step_index === state.currentResearchStep);
  });
};

// Get completed research steps
export const useCompletedResearchSteps = () => {
  return useAgentStreamStore((state) => state.researchSteps.filter((s) => s.status === 'completed'));
};

// Get in-progress research steps
export const useInProgressResearchSteps = () => {
  return useAgentStreamStore((state) => state.researchSteps.filter((s) => s.status === 'in_progress'));
};

// Get active tool call (currently executing)
export const useActiveToolCall = () => {
  return useAgentStreamStore((state) => {
    if (!state.activeToolCall) return null;
    return state.toolCalls.get(state.activeToolCall);
  });
};

// Get all tool calls for an iteration
export const useIterationToolCalls = (iteration: number) => {
  return useAgentStreamStore((state) => {
    const iterGroup = state.iterations.get(iteration);
    return iterGroup?.tool_calls || [];
  });
};

// Get current iteration data
export const useCurrentIteration = () => {
  return useAgentStreamStore((state) => {
    if (state.currentIteration === null) return null;
    return state.iterations.get(state.currentIteration);
  });
};

// Get all completed iterations
export const useCompletedIterations = () => {
  return useAgentStreamStore((state) => {
    return Array.from(state.iterations.values())
      .filter((iter) => iter.status === 'complete')
      .sort((a, b) => a.iteration - b.iteration);
  });
};

// Get error states
export const useErrorStates = () => {
  return useAgentStreamStore((state) => ({
    hasError: state.taskStatus === 'error',
    errorMessage: state.errorMessage,
    iterationErrors: Array.from(state.iterations.values())
      .filter((iter) => iter.errors.length > 0)
      .flatMap((iter) => iter.errors),
  }));
};

// Get task summary for header display
export const useTaskSummary = () => {
  return useAgentStreamStore((state) => ({
    actionName: state.taskAnalysis.action_name,
    goal: state.taskAnalysis.goal,
    status: state.taskStatus,
    currentIteration: state.currentIteration,
    totalIterations: state.totalIterations,
    totalTokens: state.totalTokens,
    finalAnswer: state.finalAnswer,
  }));
};

// Get memory usage stats
export const useMemoryStats = () => {
  return useAgentStreamStore((state) => ({
    ...state.eventCache.getMemoryUsage(),
    toolCallCount: state.toolCalls.size,
    iterationCount: state.iterations.size,
    researchStepCount: state.researchSteps.length,
    browserSnapshotCount: state.browserSnapshots.length,
  }));
};

// Get latest browser snapshot
export const useLatestBrowserSnapshot = () => {
  return useAgentStreamStore((state) => {
    const snapshots = state.browserSnapshots;
    return snapshots.length > 0 ? snapshots[snapshots.length - 1] : null;
  });
};

// Get all iterations as sorted array (for virtualized list)
export const useIterationsArray = () => {
  return useAgentStreamStore((state) => {
    return Array.from(state.iterations.values()).sort((a, b) => a.iteration - b.iteration);
  });
};

// Get raw events (for debugging or export)
export const useRawEvents = () => {
  // Access global cache directly to avoid triggering unnecessary re-renders
  // But we still subscribe to the store to get notified when events change
  useAgentStreamStore((state) => state.taskStatus); // Subscribe to any state change
  return globalEventCache.getAll();
};
