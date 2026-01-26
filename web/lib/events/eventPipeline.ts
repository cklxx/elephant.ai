import { AnyAgentEvent } from '@/lib/types';
import { AgentEventBus } from './eventBus';
import { EventRegistry } from './eventRegistry';
import { buildEventSignature } from './signature';
import { normalizeAgentEvent } from './normalize';

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
      const normalized = normalizeAgentEvent(raw);
      if (normalized.status === 'invalid') {
        this.onInvalidEvent?.(raw, normalized.error);
        return;
      }
      const event = normalized.event;
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
