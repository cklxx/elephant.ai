// Hook to convert agent events to timeline steps for ResearchTimeline

import { useMemo } from 'react';
import {
  AnyAgentEvent,
  IterationStartEvent,
  IterationCompleteEvent,
  StepStartedEvent,
  StepCompletedEvent,
  ErrorEvent,
} from '@/lib/types';
import { TimelineStep, StepStatus } from '@/components/agent/ResearchTimeline';

export function useTimelineSteps(events: AnyAgentEvent[]): TimelineStep[] {
  return useMemo(() => {
    const steps: TimelineStep[] = [];
    const activeIterations = new Map<number, Partial<TimelineStep>>();
    const activeSteps = new Map<number, Partial<TimelineStep>>();

    events.forEach((event) => {
      // Research plan steps
      if (event.event_type === 'step_started') {
        const e = event as StepStartedEvent;
        activeSteps.set(e.step_index, {
          id: `step-${e.step_index}`,
          title: `Step ${e.step_index + 1}`,
          description: e.step_description,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
        });
      }

      if (event.event_type === 'step_completed') {
        const e = event as StepCompletedEvent;
        const step = activeSteps.get(e.step_index);
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
          activeSteps.delete(e.step_index);
        }
      }

      // Iteration-based steps (fallback when no explicit steps)
      if (event.event_type === 'iteration_start') {
        const e = event as IterationStartEvent;
        activeIterations.set(e.iteration, {
          id: `iteration-${e.iteration}`,
          title: `Iteration ${e.iteration}/${e.total_iters}`,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
          toolsUsed: [],
        });
      }

      if (event.event_type === 'iteration_complete') {
        const e = event as IterationCompleteEvent;
        const iteration = activeIterations.get(e.iteration);
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
            tokensUsed: e.tokens_used,
          });
          activeIterations.delete(e.iteration);
        }
      }

      // Track tools used in iterations
      if (event.event_type === 'tool_call_start') {
        const e = event as any;
        if (e.iteration !== undefined) {
          const iteration = activeIterations.get(e.iteration);
          if (iteration && iteration.toolsUsed) {
            iteration.toolsUsed.push(e.tool_name);
          }
        }
      }

      // Handle errors
      if (event.event_type === 'error') {
        const e = event as ErrorEvent;
        const iteration = activeIterations.get(e.iteration);
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
            error: e.error,
          });
          activeIterations.delete(e.iteration);
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
