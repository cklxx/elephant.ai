import { AnyAgentEvent } from '@/lib/types';
import { handleEnvironmentSnapshot } from '@/hooks/useDiagnostics';
import { handleSandboxProgress } from '@/hooks/useSandboxProgress';

type EventSideEffect = (event: AnyAgentEvent) => void;

type Registry = Map<AnyAgentEvent['event_type'], EventSideEffect[]>;

export class EventRegistry {
  private registry: Registry = new Map();

  register(type: AnyAgentEvent['event_type'], sideEffect: EventSideEffect) {
    if (!this.registry.has(type)) {
      this.registry.set(type, []);
    }
    this.registry.get(type)!.push(sideEffect);
  }

  run(event: AnyAgentEvent) {
    const effects = this.registry.get(event.event_type);
    effects?.forEach((effect) => effect(event));
  }

  clear() {
    this.registry.clear();
  }
}

export const defaultEventRegistry = new EventRegistry();

defaultEventRegistry.register('environment_snapshot', handleEnvironmentSnapshot as EventSideEffect);

defaultEventRegistry.register('sandbox_progress', handleSandboxProgress as EventSideEffect);
