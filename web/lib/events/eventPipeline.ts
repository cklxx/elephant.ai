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
  private readonly maxEntries = 4000;
  private seen: Set<string> = new Set();
  private order: string[] = new Array(this.maxEntries);
  private orderHead = 0;
  private orderSize = 0;

  constructor(options: EventPipelineOptions) {
    this.bus = options.bus;
    this.registry = options.registry;
    this.onInvalidEvent = options.onInvalidEvent;
  }

  reset() {
    this.seen.clear();
    this.orderHead = 0;
    this.orderSize = 0;
  }

  private rememberSignature(signature: string) {
    if (this.orderSize < this.maxEntries) {
      const insertIndex = (this.orderHead + this.orderSize) % this.maxEntries;
      this.order[insertIndex] = signature;
      this.orderSize += 1;
      return;
    }

    const oldest = this.order[this.orderHead]!;
    this.seen.delete(oldest);
    this.order[this.orderHead] = signature;
    this.orderHead = (this.orderHead + 1) % this.maxEntries;
  }

  private isDuplicate(event: AnyAgentEvent): boolean {
    const signature = buildEventSignature(event);
    if (this.seen.has(signature)) {
      return true;
    }

    this.seen.add(signature);
    this.rememberSignature(signature);

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
