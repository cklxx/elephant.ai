// Agent event stream state management with Zustand + Immer
// Features:
// - LRU event caching (max 1000 events)
// - Event aggregation (tool calls, iterations, research steps)
// - Browser diagnostics tracking
// - Performant selectors for UI components

import { create } from 'zustand';
import { AnyAgentEvent } from '@/lib/types';
import {
  EventLRUCache,
  aggregateToolCalls,
  groupByIteration,
  extractResearchSteps,
  extractBrowserDiagnostics,
  AggregatedToolCall,
  IterationGroup,
  ResearchStep,
  BrowserDiagnostics,
} from '@/lib/eventAggregation';

const MAX_EVENT_COUNT = 1000;

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
  browserDiagnostics: BrowserDiagnostics[];

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

type AgentStreamBaseState = Omit<AgentStreamState, 'addEvent' | 'addEvents' | 'clearEvents' | 'recomputeAggregations'>;

const createInitialState = (): AgentStreamBaseState => ({
  eventCache: new EventLRUCache(MAX_EVENT_COUNT),
  toolCalls: new Map(),
  iterations: new Map(),
  researchSteps: [],
  browserDiagnostics: [],
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

const deriveTaskMetadata = (events: AnyAgentEvent[]): Pick<
  AgentStreamBaseState,
  | 'taskAnalysis'
  | 'taskStatus'
  | 'currentIteration'
  | 'currentResearchStep'
  | 'activeToolCall'
  | 'finalAnswer'
  | 'totalIterations'
  | 'totalTokens'
  | 'errorMessage'
> => {
  let taskStatus: AgentStreamState['taskStatus'] = 'idle';
  let taskAnalysis: AgentStreamState['taskAnalysis'] = {};
  let currentIteration: number | null = null;
  let currentResearchStep: number | null = null;
  let activeToolCall: string | null = null;
  let finalAnswer: string | undefined;
  let totalIterations: number | undefined;
  let totalTokens: number | undefined;
  let errorMessage: string | undefined;

  for (const event of events) {
    switch (event.event_type) {
      case 'task_analysis':
        taskStatus = 'analyzing';
        taskAnalysis = {
          action_name: event.action_name,
          goal: event.goal,
        };
        break;
      case 'iteration_start':
        taskStatus = 'running';
        currentIteration = event.iteration ?? currentIteration;
        if (typeof event.total_iters === 'number') {
          totalIterations = event.total_iters;
        }
        break;
      case 'iteration_complete':
        if (currentIteration === event.iteration) {
          currentIteration = null;
        }
        if (typeof event.tokens_used === 'number') {
          totalTokens = event.tokens_used;
        }
        break;
      case 'tool_call_start':
        activeToolCall = event.call_id;
        break;
      case 'tool_call_complete':
        if (activeToolCall === event.call_id) {
          activeToolCall = null;
        }
        break;
      case 'task_complete':
        taskStatus = 'completed';
        finalAnswer = event.final_answer;
        totalIterations = event.total_iterations ?? totalIterations;
        totalTokens = event.total_tokens ?? totalTokens;
        currentIteration = null;
        activeToolCall = null;
        break;
      case 'error':
        taskStatus = 'error';
        errorMessage = event.error;
        break;
      case 'step_started':
        currentResearchStep = event.step_index ?? currentResearchStep;
        break;
      case 'step_completed':
        if (currentResearchStep === event.step_index) {
          currentResearchStep = null;
        }
        break;
    }
  }

  return {
    taskAnalysis,
    taskStatus,
    currentIteration,
    currentResearchStep,
    activeToolCall,
    finalAnswer,
    totalIterations,
    totalTokens,
    errorMessage,
  };
};

const buildStateFromEvents = (events: AnyAgentEvent[]): Partial<AgentStreamBaseState> => {
  const toolCalls = aggregateToolCalls(events);
  const iterations = groupByIteration(events);
  const allSteps = extractResearchSteps(events);
  const browserDiagnostics = extractBrowserDiagnostics(events);
  const taskMetadata = deriveTaskMetadata(events);

  return {
    toolCalls,
    iterations,
    researchSteps: allSteps.filter((step) => step.status !== 'pending'),
    browserDiagnostics,
    ...taskMetadata,
  };
};

export const useAgentStreamStore = create<AgentStreamState>()((set) => ({
  ...createInitialState(),

  addEvent: (event: AnyAgentEvent) => {
    set((state) => {
      state.eventCache.add(event);
      const events = state.eventCache.getAll();
      return buildStateFromEvents(events);
    });
  },

  addEvents: (events: AnyAgentEvent[]) => {
    set((state) => {
      events.forEach((event) => state.eventCache.add(event));
      const allEvents = state.eventCache.getAll();
      return buildStateFromEvents(allEvents);
    });
  },

  clearEvents: () => {
    set(() => ({
      ...createInitialState(),
    }));
  },

  recomputeAggregations: () => {
    set((state) => {
      const events = state.eventCache.getAll();
      return buildStateFromEvents(events);
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
  return useAgentStreamStore((state) => {
    const memUsage = state.eventCache.getMemoryUsage();
    return {
      eventCount: memUsage.eventCount,
      estimatedBytes: memUsage.estimatedBytes,
      toolCallCount: state.toolCalls.size,
      iterationCount: state.iterations.size,
      researchStepCount: state.researchSteps.length,
      browserDiagnosticsCount: state.browserDiagnostics.length,
    };
  });
};

// Get latest browser diagnostics event
export const useLatestBrowserDiagnostics = () => {
  return useAgentStreamStore((state) => {
    const diagnostics = state.browserDiagnostics;
    return diagnostics.length > 0
      ? diagnostics[diagnostics.length - 1]
      : null;
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
  const eventCache = useAgentStreamStore((state) => state.eventCache);
  return eventCache.getAll();
};
