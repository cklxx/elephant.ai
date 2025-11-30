import { AnyAgentEvent } from '@/lib/types';
import { safeValidateEvent } from '@/lib/schemas';
import { AgentEventBus } from './eventBus';
import { EventRegistry } from './eventRegistry';

export interface EventPipelineOptions {
  bus: AgentEventBus;
  registry: EventRegistry;
  onInvalidEvent?: (raw: unknown, error: unknown) => void;
}

export class EventPipeline {
  private bus: AgentEventBus;
  private registry: EventRegistry;
  private onInvalidEvent?: (raw: unknown, error: unknown) => void;

  constructor(options: EventPipelineOptions) {
    this.bus = options.bus;
    this.registry = options.registry;
    this.onInvalidEvent = options.onInvalidEvent;
  }

  process(raw: unknown) {
    try {
      const validationResult = safeValidateEvent(raw);
      if (!validationResult.success) {
        this.onInvalidEvent?.(raw, validationResult.error);
        return;
      }
      const event = validationResult.data as AnyAgentEvent;
      this.registry.run(event);
      this.bus.emit(event);
    } catch (error) {
      this.onInvalidEvent?.(raw, error);
    }
  }
}
