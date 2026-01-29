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
            existing.run_id === complete.run_id;
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
              existing.run_id === complete.run_id;
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

// Selector hooks removed - use useAgentStreamStore directly with inline selectors
// Example: const researchSteps = useAgentStreamStore(state => state.researchSteps)
