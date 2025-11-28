// Agent event stream state management with Zustand + Immer
// New store focuses on incremental aggregation with normalized data structures.

import { create } from 'zustand';
import { produce } from 'immer';
import {
  AnyAgentEvent,
  ToolCallStartEvent,
  ToolCallStreamEvent,
  ToolCallCompleteEvent,
  ResearchPlanEvent,
  StepStartedEvent,
  StepCompletedEvent,
  IterationStartEvent,
  IterationCompleteEvent,
  TaskCancelledEvent,
  TaskCompleteEvent,
  ErrorEvent,
  BrowserInfoEvent,
  AttachmentPayload,
} from '@/lib/types';
import { EventLRUCache } from '@/lib/eventAggregation';

const MAX_EVENT_COUNT = 1000;

type ToolCallStatus = 'pending' | 'running' | 'streaming' | 'done' | 'error';

type UnifiedStepStatus = 'planned' | 'active' | 'done' | 'failed';

interface ToolCallState {
  id: string;
  call_id: string;
  tool_name: string;
  arguments: Record<string, any>;
  arguments_preview?: string;
  status: ToolCallStatus;
  stream_chunks: string[];
  result?: string;
  error?: string;
  duration?: number;
  started_at: string;
  completed_at?: string;
  last_stream_at?: string;
  iteration?: number;
}

interface IterationState {
  id: string;
  iteration: number;
  total_iters?: number;
  status: 'running' | 'done';
  started_at: string;
  completed_at?: string;
  tokens_used?: number;
  tools_run?: number;
  errors: string[];
}

export interface NormalizedResearchStep {
  id: string;
  step_index: number;
  description: string;
  status: UnifiedStepStatus;
  started_at?: string;
  completed_at?: string;
  result?: string;
  iteration?: number;
  last_event_at?: string;
  error?: string;
}

interface BrowserDiagnosticsEntry {
  id: string;
  timestamp: string;
  captured: string;
  success?: boolean;
  message?: string;
  user_agent?: string;
  cdp_url?: string;
  vnc_url?: string;
  viewport_width?: number;
  viewport_height?: number;
}

interface AgentStreamState {
  eventCache: EventLRUCache;
  toolCalls: Map<string, ToolCallState>;
  iterations: Map<number, IterationState>;
  steps: Map<string, NormalizedResearchStep>;
  stepOrder: string[];
  researchSteps: NormalizedResearchStep[];
  browserDiagnostics: BrowserDiagnosticsEntry[];
  currentIteration: number | null;
  activeToolCallId: string | null;
  activeResearchStepId: string | null;
  taskStatus: 'idle' | 'running' | 'completed' | 'cancelled' | 'error';
  finalAnswer?: string;
  finalAnswerAttachments?: Record<string, AttachmentPayload>;
  totalIterations?: number;
  totalTokens?: number;
  errorMessage?: string;
  addEvent: (event: AnyAgentEvent) => void;
  addEvents: (events: AnyAgentEvent[]) => void;
  clearEvents: () => void;
  recomputeAggregations: () => void;
}

type AgentStreamDraft = AgentStreamState;

const createInitialState = (): Omit<AgentStreamState, 'addEvent' | 'addEvents' | 'clearEvents' | 'recomputeAggregations'> => ({
  eventCache: new EventLRUCache(MAX_EVENT_COUNT),
  toolCalls: new Map(),
  iterations: new Map(),
  steps: new Map(),
  stepOrder: [],
  researchSteps: [],
  browserDiagnostics: [],
  currentIteration: null,
  activeToolCallId: null,
  activeResearchStepId: null,
  taskStatus: 'idle',
  finalAnswer: undefined,
  finalAnswerAttachments: undefined,
  totalIterations: undefined,
  totalTokens: undefined,
  errorMessage: undefined,
});

const syncResearchSteps = (draft: AgentStreamDraft) => {
  draft.researchSteps = draft.stepOrder
    .map((id) => draft.steps.get(id))
    .filter((step): step is NormalizedResearchStep => Boolean(step));
};

const ensureIteration = (draft: AgentStreamDraft, event: IterationStartEvent): IterationState => {
  const existing = draft.iterations.get(event.iteration);
  if (existing) {
    existing.total_iters = event.total_iters ?? existing.total_iters;
    return existing;
  }

  const iteration: IterationState = {
    id: `iteration-${event.iteration}`,
    iteration: event.iteration,
    total_iters: event.total_iters,
    status: 'running',
    started_at: event.timestamp,
    errors: [],
  };
  draft.iterations.set(event.iteration, iteration);
  return iteration;
};

const ensureStep = (draft: AgentStreamDraft, index: number, description?: string): NormalizedResearchStep => {
  const id = String(index);
  if (!draft.steps.has(id)) {
    const newStep: NormalizedResearchStep = {
      id,
      step_index: index,
      description: description ?? `Step ${index + 1}`,
      status: 'planned',
    };
    draft.steps.set(id, newStep);
    if (!draft.stepOrder.includes(id)) {
      draft.stepOrder.push(id);
    }
    return newStep;
  }

  const existing = draft.steps.get(id)!;
  if (description) {
    existing.description = description;
  }
  return existing;
};

const applyResearchPlan = (draft: AgentStreamDraft, event: ResearchPlanEvent) => {
  draft.steps.clear();
  draft.stepOrder = [];
  event.plan_steps.forEach((description, index) => {
    const id = String(index);
    draft.stepOrder.push(id);
    draft.steps.set(id, {
      id,
      step_index: index,
      description,
      status: 'planned',
    });
  });
  draft.activeResearchStepId = null;
  syncResearchSteps(draft);
};

const applyStepStarted = (draft: AgentStreamDraft, event: StepStartedEvent) => {
  const step = ensureStep(draft, event.step_index, event.step_description);
  step.status = 'active';
  step.started_at = event.timestamp;
  step.iteration = event.iteration ?? step.iteration;
  step.last_event_at = event.timestamp;
  draft.activeResearchStepId = step.id;
  syncResearchSteps(draft);
};

const applyStepCompleted = (draft: AgentStreamDraft, event: StepCompletedEvent) => {
  const step = ensureStep(draft, event.step_index, event.step_description);
  step.status = 'done';
  step.completed_at = event.timestamp;
  step.result = event.step_result;
  step.iteration = event.iteration ?? step.iteration;
  step.last_event_at = event.timestamp;
  step.error = undefined;
  if (draft.activeResearchStepId === step.id) {
    draft.activeResearchStepId = null;
  }
  syncResearchSteps(draft);
};

const applyErrorEvent = (draft: AgentStreamDraft, event: ErrorEvent) => {
  draft.taskStatus = 'error';
  draft.errorMessage = event.error;
  if (draft.currentIteration !== null) {
    const iteration = draft.iterations.get(draft.currentIteration);
    iteration?.errors.push(event.error);
  }
  if (draft.activeResearchStepId) {
    const step = draft.steps.get(draft.activeResearchStepId);
    if (step) {
      step.status = 'failed';
      step.error = event.error;
      step.last_event_at = event.timestamp;
    }
  }
  if (draft.activeToolCallId) {
    const toolCall = draft.toolCalls.get(draft.activeToolCallId);
    if (toolCall) {
      toolCall.status = 'error';
      toolCall.error = event.error;
      toolCall.completed_at = event.timestamp;
    }
  }
  syncResearchSteps(draft);
};

const applyToolCallStart = (draft: AgentStreamDraft, event: ToolCallStartEvent) => {
  const toolCall: ToolCallState = {
    id: event.call_id,
    call_id: event.call_id,
    tool_name: event.tool_name,
    arguments: event.arguments,
    arguments_preview: event.arguments_preview,
    status: 'running',
    stream_chunks: [],
    started_at: event.timestamp,
    iteration: event.iteration,
  };
  draft.toolCalls.set(event.call_id, toolCall);
  draft.activeToolCallId = event.call_id;
};

const applyToolCallStream = (draft: AgentStreamDraft, event: ToolCallStreamEvent) => {
  const existing = draft.toolCalls.get(event.call_id);
  if (!existing) return;
  existing.status = 'streaming';
  existing.stream_chunks.push(event.chunk);
  existing.last_stream_at = event.timestamp;
};

const applyToolCallComplete = (draft: AgentStreamDraft, event: ToolCallCompleteEvent) => {
  const existing = draft.toolCalls.get(event.call_id);
  if (!existing) {
    draft.toolCalls.set(event.call_id, {
      id: event.call_id,
      call_id: event.call_id,
      tool_name: event.tool_name,
      arguments: {},
      status: event.error ? 'error' : 'done',
      stream_chunks: [],
      result: event.result,
      error: event.error,
      duration: event.duration,
      started_at: event.timestamp,
      completed_at: event.timestamp,
    });
    return;
  }
  existing.status = event.error ? 'error' : 'done';
  existing.result = event.result;
  existing.error = event.error;
  existing.duration = event.duration;
  existing.completed_at = event.timestamp;
  draft.activeToolCallId = existing.status === 'error' ? existing.call_id : null;
};

const applyIterationComplete = (draft: AgentStreamDraft, event: IterationCompleteEvent) => {
  const iteration = draft.iterations.get(event.iteration);
  if (iteration) {
    iteration.status = 'done';
    iteration.completed_at = event.timestamp;
    iteration.tokens_used = event.tokens_used;
    iteration.tools_run = event.tools_run;
  }
  if (draft.currentIteration === event.iteration) {
    draft.currentIteration = null;
  }
};

const applyBrowserInfo = (draft: AgentStreamDraft, event: BrowserInfoEvent) => {
  draft.browserDiagnostics = [
    ...draft.browserDiagnostics,
    {
      id: `browser-${event.timestamp}`,
      timestamp: event.timestamp,
      captured: event.captured,
      success: event.success,
      message: event.message,
      user_agent: event.user_agent,
      cdp_url: event.cdp_url,
      vnc_url: event.vnc_url,
      viewport_width: event.viewport_width,
      viewport_height: event.viewport_height,
    },
  ];
};

const applyEventToDraft = (draft: AgentStreamDraft, event: AnyAgentEvent) => {
  switch (event.event_type) {
    case 'iteration_start': {
      const iterationEvent = event as IterationStartEvent;
      ensureIteration(draft, iterationEvent);
      draft.currentIteration = iterationEvent.iteration;
      draft.taskStatus = 'running';
      break;
    }
    case 'tool_call_start':
      applyToolCallStart(draft, event as ToolCallStartEvent);
      break;
    case 'tool_call_stream':
      applyToolCallStream(draft, event as ToolCallStreamEvent);
      break;
    case 'tool_call_complete':
      applyToolCallComplete(draft, event as ToolCallCompleteEvent);
      if (draft.activeToolCallId === (event as ToolCallCompleteEvent).call_id) {
        draft.activeToolCallId = null;
      }
      break;
    case 'iteration_complete':
      applyIterationComplete(draft, event as IterationCompleteEvent);
      draft.totalTokens = (event as IterationCompleteEvent).tokens_used ?? draft.totalTokens;
      break;
    case 'user_task': {
      // New task -> reset final answer state and attachments
      draft.taskStatus = 'running';
      draft.finalAnswer = undefined;
      draft.finalAnswerAttachments = undefined;
      draft.errorMessage = undefined;
      draft.totalIterations = undefined;
      draft.totalTokens = undefined;
      draft.currentIteration = null;
      draft.activeToolCallId = null;
      draft.toolCalls.clear();
      draft.iterations.clear();
      draft.steps.clear();
      draft.stepOrder = [];
      draft.researchSteps = [];
      break;
    }
    case 'task_complete': {
      const complete = event as TaskCompleteEvent;
      const isStreaming = complete.is_streaming === true;
      const streamFinished = complete.stream_finished !== false;

      if (isStreaming && !streamFinished) {
        draft.taskStatus = draft.taskStatus === 'idle' ? 'running' : draft.taskStatus;
        const prevAnswer = draft.finalAnswer ?? '';
        draft.finalAnswer = prevAnswer + (complete.final_answer ?? '');
      } else {
        draft.taskStatus = 'completed';
        const prevAnswer = draft.finalAnswer ?? '';
        const nextAnswer =
          complete.final_answer !== undefined && complete.final_answer !== null
            ? complete.final_answer
            : prevAnswer;
        draft.finalAnswer = nextAnswer;
      }

      if (complete.attachments !== undefined) {
        draft.finalAnswerAttachments = complete.attachments as
          | Record<string, AttachmentPayload>
          | undefined;
      }
      draft.totalIterations = complete.total_iterations;
      draft.totalTokens = complete.total_tokens;
      draft.currentIteration = null;
      draft.activeToolCallId = null;
      break;
    }
    case 'task_cancelled': {
      const cancelled = event as TaskCancelledEvent;
      draft.taskStatus = 'cancelled';
      draft.currentIteration = null;
      draft.activeToolCallId = null;
      draft.errorMessage = cancelled.reason && cancelled.reason !== 'cancelled' ? cancelled.reason : undefined;
      draft.finalAnswer = undefined;
      draft.finalAnswerAttachments = undefined;
      draft.totalIterations = undefined;
      draft.totalTokens = undefined;
      break;
    }
    case 'error':
      applyErrorEvent(draft, event as ErrorEvent);
      break;
    case 'research_plan':
      applyResearchPlan(draft, event as ResearchPlanEvent);
      break;
    case 'step_started':
      applyStepStarted(draft, event as StepStartedEvent);
      break;
    case 'step_completed':
      applyStepCompleted(draft, event as StepCompletedEvent);
      break;
    case 'browser_info':
      applyBrowserInfo(draft, event as BrowserInfoEvent);
      break;
    default:
      break;
  }
};

export const useAgentStreamStore = create<AgentStreamState>()((set, get) => ({
  ...createInitialState(),

  addEvent: (event: AnyAgentEvent) => {
    set((state) =>
      produce(state, (draft: AgentStreamDraft) => {
        if (event.event_type === 'task_complete') {
          const complete = event as TaskCompleteEvent;
          const matcher = (existing: AnyAgentEvent) =>
            existing.event_type === 'task_complete' &&
            existing.session_id === complete.session_id &&
            existing.task_id === complete.task_id;
          const replaced = draft.eventCache.replaceLastIf(matcher, event);
          if (!replaced) {
            draft.eventCache.add(event);
          }
        } else {
          draft.eventCache.add(event);
        }
        applyEventToDraft(draft, event);
      }),
    );
  },

  addEvents: (events: AnyAgentEvent[]) => {
    set((state) =>
      produce(state, (draft: AgentStreamDraft) => {
        events.forEach((event) => {
          if (event.event_type === 'task_complete') {
            const complete = event as TaskCompleteEvent;
            const matcher = (existing: AnyAgentEvent) =>
              existing.event_type === 'task_complete' &&
              existing.session_id === complete.session_id &&
              existing.task_id === complete.task_id;
            const replaced = draft.eventCache.replaceLastIf(matcher, event);
            if (!replaced) {
              draft.eventCache.add(event);
            }
          } else {
            draft.eventCache.add(event);
          }
          applyEventToDraft(draft, event);
        });
      }),
    );
  },

  clearEvents: () => {
    set(() => ({
      ...createInitialState(),
      eventCache: new EventLRUCache(MAX_EVENT_COUNT),
    }));
  },

  recomputeAggregations: () => {
    const events = get().eventCache.getAll();
    set(() =>
      produce(createInitialState(), (draft: AgentStreamDraft) => {
        draft.eventCache.addMany(events);
        events.forEach((event) => applyEventToDraft(draft, event));
      }),
    );
  },
}));

export const useCurrentResearchStep = () => {
  return useAgentStreamStore((state) => {
    if (!state.activeResearchStepId) return null;
    return state.steps.get(state.activeResearchStepId) ?? null;
  });
};

export const useCompletedResearchSteps = () => {
  return useAgentStreamStore((state) => state.researchSteps.filter((step) => step.status === 'done'));
};

export const useActiveResearchSteps = () => {
  return useAgentStreamStore((state) => state.researchSteps.filter((step) => step.status === 'active'));
};

export const usePlannedResearchSteps = () => {
  return useAgentStreamStore((state) => state.researchSteps.filter((step) => step.status === 'planned'));
};

export const useFailedResearchSteps = () => {
  return useAgentStreamStore((state) => state.researchSteps.filter((step) => step.status === 'failed'));
};

export const useActiveToolCall = () => {
  return useAgentStreamStore((state) => {
    if (!state.activeToolCallId) return null;
    return state.toolCalls.get(state.activeToolCallId) ?? null;
  });
};

export const useIterationToolCalls = (iteration: number) => {
  return useAgentStreamStore((state) => {
    const calls = Array.from(state.toolCalls.values()).filter((toolCall) => toolCall.iteration === iteration);
    return calls.sort((a, b) => a.started_at.localeCompare(b.started_at));
  });
};

export const useCurrentIteration = () => {
  return useAgentStreamStore((state) => {
    if (state.currentIteration === null) return null;
    return state.iterations.get(state.currentIteration) ?? null;
  });
};

export const useCompletedIterations = () => {
  return useAgentStreamStore((state) =>
    Array.from(state.iterations.values())
      .filter((iter) => iter.status === 'done')
      .sort((a, b) => a.iteration - b.iteration),
  );
};

export const useErrorStates = () => {
  return useAgentStreamStore((state) => ({
    hasError: state.taskStatus === 'error',
    errorMessage: state.errorMessage,
    iterationErrors: Array.from(state.iterations.values())
      .filter((iter) => iter.errors.length > 0)
      .flatMap((iter) => iter.errors),
  }));
};

export const useTaskSummary = () => {
  return useAgentStreamStore((state) => ({
    status: state.taskStatus,
    currentIteration: state.currentIteration,
    totalIterations: state.totalIterations,
    totalTokens: state.totalTokens,
    finalAnswer: state.finalAnswer,
    finalAnswerAttachments: state.finalAnswerAttachments,
  }));
};

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

export const useLatestBrowserDiagnostics = () => {
  return useAgentStreamStore((state) => {
    const diagnostics = state.browserDiagnostics;
    return diagnostics.length > 0 ? diagnostics[diagnostics.length - 1] : null;
  });
};

export const useIterationsArray = () => {
  return useAgentStreamStore((state) =>
    Array.from(state.iterations.values()).sort((a, b) => a.iteration - b.iteration),
  );
};

export const useRawEvents = () => {
  const eventCache = useAgentStreamStore((state) => state.eventCache);
  return eventCache.getAll();
};
