import { z } from 'zod';

import type { AnyAgentEvent } from '@/lib/types';
import { safeValidateEvent } from '@/lib/schemas';
import { isEventType } from '@/lib/events/matching';

export type NormalizedEventResult =
  | { status: 'valid'; event: AnyAgentEvent }
  | { status: 'coerced'; event: AnyAgentEvent; error: z.ZodError }
  | { status: 'invalid'; event: null; error: z.ZodError };

export function normalizeAgentEvents(rawEvents: unknown[]): AnyAgentEvent[] {
  const normalized: AnyAgentEvent[] = [];
  rawEvents.forEach((raw) => {
    const result = normalizeAgentEvent(raw);
    if (result.status !== 'invalid') {
      normalized.push(result.event);
    }
  });

  return dedupeFinalEvents(normalized);
}

export function normalizeAgentEvent(raw: unknown): NormalizedEventResult {
  const validation = safeValidateEvent(raw);
  if (validation.success) {
    return { status: 'valid', event: validation.data };
  }

  const fallback = coerceEvent(raw);
  if (fallback) {
    return { status: 'coerced', event: fallback, error: validation.error };
  }

  return { status: 'invalid', event: null, error: validation.error };
}

function dedupeFinalEvents(events: AnyAgentEvent[]): AnyAgentEvent[] {
  const seenTasks = new Set<string>();
  const result: AnyAgentEvent[] = [];

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const evt = events[i];
    const key = buildFinalEventKey(evt);
    if (key) {
      if (seenTasks.has(key)) {
        continue;
      }
      seenTasks.add(key);
    }
    result.push(evt);
  }

  result.reverse();
  return result;
}

function buildFinalEventKey(event: AnyAgentEvent): string | null {
  if (!isEventType(event, 'workflow.result.final')) {
    return null;
  }
  const taskId = 'task_id' in event ? event.task_id : undefined;
  const sessionId = 'session_id' in event ? event.session_id : undefined;
  if (!taskId || !sessionId) {
    return null;
  }
  return `${sessionId}|${taskId}`;
}

function coerceEvent(raw: unknown): AnyAgentEvent | null {
  if (!raw || typeof raw !== 'object') {
    return null;
  }

  const obj = raw as Record<string, any>;
  const rawPayload = obj.payload;
  const payloadObject =
    rawPayload && typeof rawPayload === 'object' && !Array.isArray(rawPayload)
      ? (rawPayload as Record<string, any>)
      : null;
  const merged: Record<string, any> = payloadObject ? { ...payloadObject, ...obj } : { ...obj };
  merged.payload = payloadObject ?? obj.payload ?? null;

  if (payloadObject && merged.duration === undefined && typeof payloadObject.duration_ms === 'number') {
    merged.duration = payloadObject.duration_ms;
  }

  const eventType = merged.event_type;
  if (typeof eventType !== 'string' || !eventType.trim()) {
    return null;
  }

  if (!merged.timestamp) {
    merged.timestamp = new Date().toISOString();
  }
  if (!merged.agent_level) {
    merged.agent_level = 'core';
  }
  if (merged.session_id === undefined) {
    merged.session_id = '';
  }
  if (merged.version === undefined || merged.version === null) {
    merged.version = 1;
  }

  try {
    z
      .object({
        event_type: z.string(),
        timestamp: z.string(),
        agent_level: z.string(),
      })
      .passthrough()
      .parse(merged);
    return merged as AnyAgentEvent;
  } catch {
    return merged as AnyAgentEvent;
  }
}
