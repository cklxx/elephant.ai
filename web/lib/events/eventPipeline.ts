import { AnyAgentEvent } from '@/lib/types';
import { safeValidateEvent } from '@/lib/schemas';
import { AgentEventBus } from './eventBus';
import { EventRegistry } from './eventRegistry';
import { buildEventSignature } from './signature';
import { z } from 'zod';

export interface EventPipelineOptions {
  bus: AgentEventBus;
  registry: EventRegistry;
  onInvalidEvent?: (raw: unknown, error: unknown) => void;
}

export class EventPipeline {
  private bus: AgentEventBus;
  private registry: EventRegistry;
  private onInvalidEvent?: (raw: unknown, error: unknown) => void;
  private seen: Set<string> = new Set();
  private order: string[] = [];
  private readonly maxEntries = 4000;

  constructor(options: EventPipelineOptions) {
    this.bus = options.bus;
    this.registry = options.registry;
    this.onInvalidEvent = options.onInvalidEvent;
  }

  reset() {
    this.seen.clear();
    this.order = [];
  }

  private isDuplicate(event: AnyAgentEvent): boolean {
    const signature = buildEventSignature(event);
    if (this.seen.has(signature)) {
      return true;
    }

    this.seen.add(signature);
    this.order.push(signature);
    if (this.order.length > this.maxEntries) {
      const oldest = this.order.shift();
      if (oldest) {
        this.seen.delete(oldest);
      }
    }

    return false;
  }

  process(raw: unknown) {
    try {
      const validationResult = safeValidateEvent(raw);
      if (!validationResult.success) {
        const fallback = coerceEvent(raw);
        if (fallback) {
          if (this.isDuplicate(fallback)) {
            return;
          }
          this.registry.run(fallback);
          this.bus.emit(fallback);
        } else {
          this.onInvalidEvent?.(raw, validationResult.error);
        }
        return;
      }
      const event = validationResult.data as AnyAgentEvent;
      if (this.isDuplicate(event)) {
        return;
      }
      this.registry.run(event);
      this.bus.emit(event);
    } catch (error) {
      this.onInvalidEvent?.(raw, error);
    }
  }
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
    // Keep validation lightweight; this prevents obviously malformed payloads
    // while still accepting envelopes that drift from the strict schema.
    z.object({
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
