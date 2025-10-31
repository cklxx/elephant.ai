import { AnyAgentEvent } from '@/lib/types';

type Listener = (event: AnyAgentEvent) => void;

type ListenerMap = Map<string, Set<Listener>>;

/**
 * Lightweight event bus to broadcast agent events to multiple subscribers.
 * This replaces implicit shared state inside hooks and enables
 * independent panels to subscribe to subsets of the stream.
 */
export class AgentEventBus {
  private listeners: ListenerMap = new Map();
  private wildcardListeners: Set<Listener> = new Set();

  emit(event: AnyAgentEvent) {
    const listenersForType = this.listeners.get(event.event_type);
    if (listenersForType) {
      listenersForType.forEach((listener) => listener(event));
    }
    this.wildcardListeners.forEach((listener) => listener(event));
  }

  subscribe(type: AnyAgentEvent['event_type'], listener: Listener): () => void;
  subscribe(listener: Listener): () => void;
  subscribe(typeOrListener: AnyAgentEvent['event_type'] | Listener, maybeListener?: Listener) {
    if (typeof typeOrListener === 'function') {
      const listener = typeOrListener;
      this.wildcardListeners.add(listener);
      return () => {
        this.wildcardListeners.delete(listener);
      };
    }

    const type = typeOrListener;
    const listener = maybeListener!;
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    const bucket = this.listeners.get(type)!;
    bucket.add(listener);
    return () => {
      bucket.delete(listener);
      if (bucket.size === 0) {
        this.listeners.delete(type);
      }
    };
  }

  clear() {
    this.listeners.clear();
    this.wildcardListeners.clear();
  }
}

export const agentEventBus = new AgentEventBus();
