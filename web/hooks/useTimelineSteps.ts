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
    let latestTaskText: string | null = null;
    let latestPrepareIdea: string | null = null;

    const shorten = (text: string, max: number) => {
      const trimmed = text.trim();
      if (trimmed.length <= max) return trimmed;
      return `${trimmed.slice(0, max)}…`;
    };

    const toStageKey = (value: unknown) =>
      typeof value === 'string' ? value.trim().toLowerCase() : '';

    const extractIdea = (raw: unknown): string | null => {
      if (!raw || typeof raw !== 'object') return null;
      const obj = raw as Record<string, unknown>;
      const candidates = [obj.approach, obj.goal, obj.action_name, obj.idea];
      for (const cand of candidates) {
        if (typeof cand === 'string' && cand.trim()) return cand.trim();
      }
      return null;
    };

    const resolveStageTitle = (stageKey: string): string | null => {
      if (stageKey === 'execute') {
        if (latestTaskText && latestTaskText.trim()) return shorten(latestTaskText, 48);
        return '执行';
      }
      if (stageKey === 'prepare') {
        if (latestPrepareIdea && latestPrepareIdea.trim()) return shorten(latestPrepareIdea, 48);
        return '想法';
      }
      if (stageKey === 'summarize') return '总结';
      if (stageKey === 'persist') return '保存';
      return null;
    };

    events.forEach((event, index) => {
      if (event.event_type === 'workflow.input.received' && typeof (event as any).task === 'string') {
        latestTaskText = String((event as any).task);
      }

      if (isWorkflowNodeStartedEvent(event)) {
        const stageKey = toStageKey((event as any).step_description);
        const id = `step-${event.step_index}`;
        const step = steps.get(id) ?? {
          id,
          title: event.step_description ?? `Step ${event.step_index + 1}`,
          status: 'planned',
        };
        const stageTitle = stageKey ? resolveStageTitle(stageKey) : null;
        steps.set(id, {
          ...step,
          title: stageTitle ?? event.step_description ?? step.title,
          status: 'active',
          startTime: new Date(event.timestamp).getTime(),
          anchorEventIndex: index,
        });
      }

      if (isWorkflowNodeCompletedEvent(event)) {
        const stageKey = toStageKey((event as any).step_description);
        const id = `step-${event.step_index}`;
        const prev = steps.get(id);
        const endTime = new Date(event.timestamp).getTime();
        const rawStepResult = (event as any).step_result as unknown;
        if (stageKey === 'prepare') {
          const idea = extractIdea(rawStepResult);
          if (idea) latestPrepareIdea = idea;
        }
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
        const stageTitle = stageKey ? resolveStageTitle(stageKey) : null;
        steps.set(id, {
          id,
          title: stageTitle ?? event.step_description ?? prev?.title ?? `Step ${event.step_index + 1}`,
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
