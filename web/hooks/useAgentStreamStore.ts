// Agent event stream state management with Zustand + Immer
// New store focuses on incremental aggregation with normalized data structures.

import { create } from "zustand";
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

const isWorkflowResultFinalEvent = (
  event: AnyAgentEvent,
): event is WorkflowResultFinalEvent =>
  isEventType(event, "workflow.result.final");

const ingestEvent = (draft: AgentStreamDraft, event: AnyAgentEvent) => {
  if (isWorkflowResultFinalEvent(event)) {
    const matcher = (existing: AnyAgentEvent) =>
      isWorkflowResultFinalEvent(existing) &&
      existing.session_id === event.session_id &&
      existing.run_id === event.run_id;
    const replaced = draft.eventCache.replaceLastIf(matcher, event);
    if (!replaced) {
      draft.eventCache.add(event);
    }
  } else if (event.event_type !== "connected") {
    draft.eventCache.add(event);
  }

  applyEventToDraft(draft, event);
};

export const useAgentStreamStore = create<AgentStreamState>()((set, get) => ({
  ...createInitialState(),

  addEvent: (event: AnyAgentEvent) => {
    set((state) =>
      produce(state, (draft: AgentStreamDraft) => {
        ingestEvent(draft, event);
      }),
    );
  },

  addEvents: (events: AnyAgentEvent[]) => {
    set((state) =>
      produce(state, (draft: AgentStreamDraft) => {
        events.forEach((event) => {
          ingestEvent(draft, event);
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

// Selector hooks removed - use useAgentStreamStore directly with inline selectors
// Example: const researchSteps = useAgentStreamStore(state => state.researchSteps)
