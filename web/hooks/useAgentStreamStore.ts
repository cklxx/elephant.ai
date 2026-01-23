// Agent event stream state management with Zustand + Immer
// New store focuses on incremental aggregation with normalized data structures.

import { create } from "zustand";
import { useShallow } from "zustand/react/shallow";
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
    useShallow((state) =>
      state.researchSteps.filter((step) => step.status === "done"),
    ),
  );
};

export const useActiveResearchSteps = () => {
  return useAgentStreamStore(
    useShallow((state) =>
      state.researchSteps.filter((step) => step.status === "active"),
    ),
  );
};

export const usePlannedResearchSteps = () => {
  return useAgentStreamStore(
    useShallow((state) =>
      state.researchSteps.filter((step) => step.status === "planned"),
    ),
  );
};

export const useFailedResearchSteps = () => {
  return useAgentStreamStore(
    useShallow((state) =>
      state.researchSteps.filter((step) => step.status === "failed"),
    ),
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
    useShallow((state) => {
      const calls = Array.from(state.toolCalls.values()).filter(
        (toolCall) => toolCall.iteration === iteration,
      );
      return calls.sort((a, b) => a.started_at.localeCompare(b.started_at));
    }),
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
    useShallow((state) =>
      Array.from(state.iterations.values())
        .filter((iter) => iter.status === "done")
        .sort((a, b) => a.iteration - b.iteration),
    ),
  );
};

export const useErrorStates = () => {
  return useAgentStreamStore(
    useShallow((state) => ({
      hasError: state.taskStatus === "error",
      errorMessage: state.errorMessage,
      iterationErrors: Array.from(state.iterations.values())
        .filter((iter) => iter.errors.length > 0)
        .flatMap((iter) => iter.errors),
    })),
  );
};

export const useTaskSummary = () => {
  return useAgentStreamStore(
    useShallow((state) => ({
      status: state.taskStatus,
      currentIteration: state.currentIteration,
      totalIterations: state.totalIterations,
      totalTokens: state.totalTokens,
      finalAnswer: state.finalAnswer,
      finalAnswerAttachments: state.finalAnswerAttachments,
    })),
  );
};

export const useMemoryStats = () => {
  return useAgentStreamStore(
    useShallow((state) => {
      const memUsage = state.eventCache.getMemoryUsage();
      return {
        eventCount: memUsage.eventCount,
        estimatedBytes: memUsage.estimatedBytes,
        toolCallCount: state.toolCalls.size,
        iterationCount: state.iterations.size,
        researchStepCount: state.researchSteps.length,
      };
    }),
  );
};

export const useIterationsArray = () => {
  return useAgentStreamStore(
    useShallow((state) =>
      Array.from(state.iterations.values()).sort(
        (a, b) => a.iteration - b.iteration,
      ),
    ),
  );
};

export const useRawEvents = () => {
  const eventCache = useAgentStreamStore((state) => state.eventCache);
  return eventCache.getAll();
};
