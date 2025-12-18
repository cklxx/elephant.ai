// Hook to convert agent events to timeline steps for plan/timeline summaries

import { useMemo } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import {
  isIterationNodeStartedEvent,
  isIterationNodeCompletedEvent,
  isWorkflowNodeStartedEvent,
  isWorkflowNodeCompletedEvent,
  isWorkflowNodeFailedEvent,
  isWorkflowToolStartedEvent,
} from '@/lib/typeGuards';
import { TimelineStep } from '@/lib/planTypes';

export function useTimelineSteps(events: AnyAgentEvent[]): TimelineStep[] {
  return useMemo(() => {
    const hasExplicitSteps = events.some((event) => {
      if (event.event_type === 'workflow.plan.created') return true;
      return (
        (event as any).event_type === 'workflow.node.started' &&
        typeof (event as any).step_index === 'number'
      ) || (
        (event as any).event_type === 'workflow.node.completed' &&
        typeof (event as any).step_index === 'number'
      );
    });

    if (!hasExplicitSteps) {
      return [];
    }

    const steps = new Map<string, TimelineStep>();
    const iterationFallback = new Map<number, TimelineStep>();

    events.forEach((event, index) => {
      if (event.event_type === 'workflow.plan.created') {
        const planned = (event as any).steps as unknown;
        if (Array.isArray(planned)) {
          planned.forEach((raw, stepIndex) => {
            const description = typeof raw === 'string' ? raw : '';
            const id = `step-${stepIndex}`;
            const prev = steps.get(id);
            steps.set(id, {
              id,
              title: description || prev?.title || `Step ${stepIndex + 1}`,
              description: undefined,
              status: prev?.status ?? 'planned',
              anchorEventIndex: prev?.anchorEventIndex ?? index,
            });
          });
        }
      }

      if (isWorkflowNodeStartedEvent(event)) {
        const id = `step-${event.step_index}`;
        const step = steps.get(id) ?? {
          id,
          title: event.step_description ?? `Step ${event.step_index + 1}`,
          status: 'planned',
        };
        steps.set(id, {
          ...step,
          title: event.step_description ?? step.title,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
          anchorEventIndex: index,
        });
      }

      if (isWorkflowNodeCompletedEvent(event)) {
        const id = `step-${event.step_index}`;
        const prev = steps.get(id);
        const endTime = new Date(event.timestamp).getTime();
        const stepResult =
          typeof event.step_result === 'string'
            ? event.step_result
            : event.step_result !== undefined
              ? JSON.stringify(event.step_result)
              : undefined;
        const status = (() => {
          const raw = typeof (event as any).status === 'string' ? String((event as any).status).toLowerCase() : '';
          return raw === 'failed' ? 'failed' : 'done';
        })();
        steps.set(id, {
          id,
          title: event.step_description ?? prev?.title ?? `Step ${event.step_index + 1}`,
          description: undefined,
          status,
          startTime: prev?.startTime,
          endTime,
          duration: prev?.startTime ? endTime - prev.startTime : undefined,
          result: stepResult,
          anchorEventIndex: prev?.anchorEventIndex ?? index,
        });
      }

      if (isIterationNodeStartedEvent(event)) {
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

      if (isWorkflowToolStartedEvent(event) && typeof event.iteration === 'number') {
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

      if (isIterationNodeCompletedEvent(event)) {
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

      if (isWorkflowNodeFailedEvent(event) && typeof event.iteration === 'number') {
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

    const parseStepIndex = (id: string) => {
      const match = id.match(/^step-(\d+)$/);
      return match ? Number(match[1]) : Number.POSITIVE_INFINITY;
    };

    return Array.from(steps.values()).sort((a, b) => {
      const aStarted = typeof a.startTime === 'number';
      const bStarted = typeof b.startTime === 'number';
      if (aStarted && bStarted) return (a.startTime ?? 0) - (b.startTime ?? 0);
      if (aStarted !== bStarted) return aStarted ? -1 : 1;

      const aAnchor = typeof a.anchorEventIndex === 'number' ? a.anchorEventIndex : Number.POSITIVE_INFINITY;
      const bAnchor = typeof b.anchorEventIndex === 'number' ? b.anchorEventIndex : Number.POSITIVE_INFINITY;
      if (aAnchor !== bAnchor) return aAnchor - bAnchor;

      return parseStepIndex(a.id) - parseStepIndex(b.id);
    });
  }, [events]);
}
