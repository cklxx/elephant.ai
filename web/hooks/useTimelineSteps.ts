// Hook to convert agent events to timeline steps for plan/timeline summaries

import { useMemo } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import {
  isStepStartedEvent,
  isStepCompletedEvent,
  isIterationStartEvent,
  isIterationCompleteEvent,
  isErrorEvent,
  isToolCallStartEvent,
  isResearchPlanEvent,
} from '@/lib/typeGuards';
import { TimelineStep } from '@/lib/planTypes';

export function useTimelineSteps(events: AnyAgentEvent[]): TimelineStep[] {
  return useMemo(() => {
    const steps = new Map<string, TimelineStep>();
    const iterationFallback = new Map<number, TimelineStep>();

    events.forEach((event, index) => {
      if (isResearchPlanEvent(event)) {
        event.plan_steps.forEach((description, idx) => {
          const id = `step-${idx}`;
          if (!steps.has(id)) {
            steps.set(id, {
              id,
              title: `Step ${idx + 1}`,
              description,
              status: 'planned',
            });
          }
        });
      }

      if (isStepStartedEvent(event)) {
        const id = `step-${event.step_index}`;
        const step = steps.get(id) ?? {
          id,
          title: `Step ${event.step_index + 1}`,
          status: 'planned',
        };
        steps.set(id, {
          ...step,
          description: event.step_description ?? step.description,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
          anchorEventIndex: index,
        });
      }

      if (isStepCompletedEvent(event)) {
        const id = `step-${event.step_index}`;
        const prev = steps.get(id);
        const endTime = new Date(event.timestamp).getTime();
        steps.set(id, {
          id,
          title: prev?.title ?? `Step ${event.step_index + 1}`,
          description: event.step_description ?? prev?.description,
          status: 'done',
          startTime: prev?.startTime,
          endTime,
          duration: prev?.startTime ? endTime - prev.startTime : undefined,
          result: event.step_result,
          anchorEventIndex: prev?.anchorEventIndex ?? index,
        });
      }

      if (isIterationStartEvent(event)) {
        const id = `iteration-${event.iteration}`;
        iterationFallback.set(event.iteration, {
          id,
          title: `Iteration ${event.iteration}/${event.total_iters}`,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
          toolsUsed: [],
          anchorEventIndex: index,
        });
      }

      if (isToolCallStartEvent(event)) {
        const iteration = iterationFallback.get(event.iteration);
        if (iteration) {
          const tools = iteration.toolsUsed ?? [];
          if (!tools.includes(event.tool_name)) {
            tools.push(event.tool_name);
          }
          iterationFallback.set(event.iteration, {
            ...iteration,
            toolsUsed: tools,
          });
        }
      }

      if (isIterationCompleteEvent(event)) {
        const iteration = iterationFallback.get(event.iteration);
        if (iteration) {
          const endTime = new Date(event.timestamp).getTime();
          iterationFallback.set(event.iteration, {
            ...iteration,
            status: 'done',
            endTime,
            duration: iteration.startTime ? endTime - iteration.startTime : undefined,
            tokensUsed: event.tokens_used,
          });
        }
      }

      if (isErrorEvent(event)) {
        const iteration = iterationFallback.get(event.iteration);
        if (iteration) {
          const endTime = new Date(event.timestamp).getTime();
          iterationFallback.set(event.iteration, {
            ...iteration,
            status: 'failed',
            endTime,
            duration: iteration.startTime ? endTime - iteration.startTime : undefined,
            error: event.error,
          });
        }
      }
    });

    // Promote iteration fallback entries if no explicit steps exist
    if (steps.size === 0 && iterationFallback.size > 0) {
      iterationFallback.forEach((iteration, key) => {
        steps.set(`iteration-${key}`, iteration);
      });
    }

    return Array.from(steps.values()).sort((a, b) => (a.startTime || 0) - (b.startTime || 0));
  }, [events]);
}
