// Agent event stream state management with Zustand + Immer
// New store focuses on incremental aggregation with normalized data structures.

import { create } from "zustand";
import { shallow } from "zustand/shallow";
import { produce } from "immer";
import {
  AnyAgentEvent,
  WorkflowResultFinalEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import { EventLRUCache } from "@/lib/eventAggregation";
import { applyEventToDraft } from "@/lib/events/reducer";
import type { AgentStreamData } from "@/lib/events/agentStreamTypes";

const MAX_EVENT_COUNT = 1000;

export interface AgentStreamState extends AgentStreamData {
  addEvent: (event: AnyAgentEvent) => void;
  addEvents: (events: AnyAgentEvent[]) => void;
  clearEvents: () => void;
  recomputeAggregations: () => void;
}

type AgentStreamDraft = AgentStreamData;

const createInitialState = (): AgentStreamData => ({
  eventCache: new EventLRUCache(MAX_EVENT_COUNT),
  toolCalls: new Map(),
  iterations: new Map(),
  steps: new Map(),
  stepOrder: [],
  researchSteps: [],
  currentIteration: null,
  activeToolCallId: null,
  activeResearchStepId: null,
  taskStatus: "idle",
  finalAnswer: undefined,
  finalAnswerAttachments: undefined,
  totalIterations: undefined,
  totalTokens: undefined,
  errorMessage: undefined,
});

export const useAgentStreamStore = create<AgentStreamState>()((set, get) => ({
  ...createInitialState(),

  addEvent: (event: AnyAgentEvent) => {
    set((state) =>
      produce(state, (draft: AgentStreamDraft) => {
        if (isEventType(event, "workflow.result.final", "workflow.result.final")) {
          const complete = event as WorkflowResultFinalEvent;
          const matcher = (existing: AnyAgentEvent) =>
            isEventType(existing, "workflow.result.final", "workflow.result.final") &&
            existing.session_id === complete.session_id &&
            existing.task_id === complete.task_id;
          const replaced = draft.eventCache.replaceLastIf(matcher, event);
          if (!replaced) {
            draft.eventCache.add(event);
          }
        } else if (event.event_type !== "connected") {
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
          if (isEventType(event, "workflow.result.final", "workflow.result.final")) {
            const complete = event as WorkflowResultFinalEvent;
            const matcher = (existing: AnyAgentEvent) =>
              isEventType(existing, "workflow.result.final", "workflow.result.final") &&
              existing.session_id === complete.session_id &&
              existing.task_id === complete.task_id;
            const replaced = draft.eventCache.replaceLastIf(matcher, event);
            if (!replaced) {
              draft.eventCache.add(event);
            }
          } else if (event.event_type !== "connected") {
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
  return useAgentStreamStore(
    (state) => state.researchSteps.filter((step) => step.status === "done"),
    shallow,
  );
};

export const useActiveResearchSteps = () => {
  return useAgentStreamStore(
    (state) => state.researchSteps.filter((step) => step.status === "active"),
    shallow,
  );
};

export const usePlannedResearchSteps = () => {
  return useAgentStreamStore(
    (state) => state.researchSteps.filter((step) => step.status === "planned"),
    shallow,
  );
};

export const useFailedResearchSteps = () => {
  return useAgentStreamStore(
    (state) => state.researchSteps.filter((step) => step.status === "failed"),
    shallow,
  );
};

export const useActiveToolCall = () => {
  return useAgentStreamStore((state) => {
    if (!state.activeToolCallId) return null;
    return state.toolCalls.get(state.activeToolCallId) ?? null;
  });
};

export const useIterationToolCalls = (iteration: number) => {
  return useAgentStreamStore(
    (state) => {
      const calls = Array.from(state.toolCalls.values()).filter(
        (toolCall) => toolCall.iteration === iteration,
      );
      return calls.sort((a, b) => a.started_at.localeCompare(b.started_at));
    },
    shallow,
  );
};

export const useCurrentIteration = () => {
  return useAgentStreamStore((state) => {
    if (state.currentIteration === null) return null;
    return state.iterations.get(state.currentIteration) ?? null;
  });
};

export const useCompletedIterations = () => {
  return useAgentStreamStore(
    (state) =>
      Array.from(state.iterations.values())
        .filter((iter) => iter.status === "done")
        .sort((a, b) => a.iteration - b.iteration),
    shallow,
  );
};

export const useErrorStates = () => {
  return useAgentStreamStore(
    (state) => ({
      hasError: state.taskStatus === "error",
      errorMessage: state.errorMessage,
      iterationErrors: Array.from(state.iterations.values())
        .filter((iter) => iter.errors.length > 0)
        .flatMap((iter) => iter.errors),
    }),
    shallow,
  );
};

export const useTaskSummary = () => {
  return useAgentStreamStore(
    (state) => ({
      status: state.taskStatus,
      currentIteration: state.currentIteration,
      totalIterations: state.totalIterations,
      totalTokens: state.totalTokens,
      finalAnswer: state.finalAnswer,
      finalAnswerAttachments: state.finalAnswerAttachments,
    }),
    shallow,
  );
};

export const useMemoryStats = () => {
  return useAgentStreamStore(
    (state) => {
      const memUsage = state.eventCache.getMemoryUsage();
      return {
        eventCount: memUsage.eventCount,
        estimatedBytes: memUsage.estimatedBytes,
        toolCallCount: state.toolCalls.size,
        iterationCount: state.iterations.size,
        researchStepCount: state.researchSteps.length,
      };
    },
    shallow,
  );
};

export const useIterationsArray = () => {
  return useAgentStreamStore(
    (state) =>
      Array.from(state.iterations.values()).sort(
        (a, b) => a.iteration - b.iteration,
      ),
    shallow,
  );
};

export const useRawEvents = () => {
  const eventCache = useAgentStreamStore((state) => state.eventCache);
  return eventCache.getAll();
};
