import { AnyAgentEvent } from '@/lib/types';
import { safeValidateEvent } from '@/lib/schemas';
import { AgentEventBus } from './eventBus';
import { EventRegistry } from './eventRegistry';
import { SubagentEventDeriver } from './subagentDeriver';

export interface EventPipelineOptions {
  bus: AgentEventBus;
  registry: EventRegistry;
  onInvalidEvent?: (raw: unknown, error: unknown) => void;
}

export class EventPipeline {
  private bus: AgentEventBus;
  private registry: EventRegistry;
  private onInvalidEvent?: (raw: unknown, error: unknown) => void;
  private subagentDeriver: SubagentEventDeriver;

  constructor(options: EventPipelineOptions) {
    this.bus = options.bus;
    this.registry = options.registry;
    this.onInvalidEvent = options.onInvalidEvent;
    this.subagentDeriver = new SubagentEventDeriver();
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

      // Emit synthetic subagent progress/complete events derived from
      // subtask-wrapped streams so the UI renders the aggregated state
      // even when the backend does not emit those event types directly.
      const derivedEvents = this.subagentDeriver.derive(event);
      derivedEvents.forEach((derived) => {
        this.registry.run(derived);
        this.bus.emit(derived);
      });
    } catch (error) {
      this.onInvalidEvent?.(raw, error);
    }
  }
}
