// Hook to convert agent events to timeline steps for ResearchTimeline

import { useMemo } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import {
  isStepStartedEvent,
  isStepCompletedEvent,
  isIterationStartEvent,
  isIterationCompleteEvent,
  isErrorEvent,
  isToolCallStartEvent,
} from '@/lib/typeGuards';
import { TimelineStep } from '@/components/agent/ResearchTimeline';

export function useTimelineSteps(events: AnyAgentEvent[]): TimelineStep[] {
  return useMemo(() => {
    const steps: TimelineStep[] = [];
    const activeIterations = new Map<number, Partial<TimelineStep>>();
    const activeSteps = new Map<number, Partial<TimelineStep>>();

    events.forEach((event) => {
      // Research plan steps
      if (isStepStartedEvent(event)) {
        activeSteps.set(event.step_index, {
          id: `step-${event.step_index}`,
          title: `Step ${event.step_index + 1}`,
          description: event.step_description,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
        });
      }

      if (isStepCompletedEvent(event)) {
        const step = activeSteps.get(event.step_index);
        if (step) {
          const endTime = new Date(event.timestamp).getTime();
          steps.push({
            id: step.id!,
            title: step.title!,
            description: step.description,
            status: 'complete',
            startTime: step.startTime,
            endTime,
            duration: step.startTime ? endTime - step.startTime : undefined,
          });
          activeSteps.delete(event.step_index);
        }
      }

      // Iteration-based steps (fallback when no explicit steps)
      if (isIterationStartEvent(event)) {
        activeIterations.set(event.iteration, {
          id: `iteration-${event.iteration}`,
          title: `Iteration ${event.iteration}/${event.total_iters}`,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
          toolsUsed: [],
        });
      }

      if (isIterationCompleteEvent(event)) {
        const iteration = activeIterations.get(event.iteration);
        if (iteration) {
          const endTime = new Date(event.timestamp).getTime();
          steps.push({
            id: iteration.id!,
            title: iteration.title!,
            description: iteration.description,
            status: 'complete',
            startTime: iteration.startTime,
            endTime,
            duration: iteration.startTime ? endTime - iteration.startTime : undefined,
            toolsUsed: iteration.toolsUsed,
            tokensUsed: event.tokens_used,
          });
          activeIterations.delete(event.iteration);
        }
      }

      // Track tools used in iterations
      if (isToolCallStartEvent(event)) {
        const iteration = activeIterations.get(event.iteration);
        if (iteration && iteration.toolsUsed) {
          iteration.toolsUsed.push(event.tool_name);
        }
      }

      // Handle errors
      if (isErrorEvent(event)) {
        const iteration = activeIterations.get(event.iteration);
        if (iteration) {
          const endTime = new Date(event.timestamp).getTime();
          steps.push({
            id: iteration.id!,
            title: iteration.title!,
            description: iteration.description,
            status: 'error',
            startTime: iteration.startTime,
            endTime,
            duration: iteration.startTime ? endTime - iteration.startTime : undefined,
            toolsUsed: iteration.toolsUsed,
            error: event.error,
          });
          activeIterations.delete(event.iteration);
        }
      }
    });

    // Add pending active iterations/steps at the end
    activeIterations.forEach((iteration) => {
      if (iteration.id) {
        steps.push({
          id: iteration.id,
          title: iteration.title!,
          description: iteration.description,
          status: 'active',
          startTime: iteration.startTime,
          toolsUsed: iteration.toolsUsed,
        });
      }
    });

    activeSteps.forEach((step) => {
      if (step.id) {
        steps.push({
          id: step.id,
          title: step.title!,
          description: step.description,
          status: 'active',
          startTime: step.startTime,
        });
      }
    });

    return steps.sort((a, b) => (a.startTime || 0) - (b.startTime || 0));
  }, [events]);
}
