import { z } from 'zod';

// Event validation schema
export const VisualizerEventSchema = z.object({
  timestamp: z.string(),
  event: z.string(),
  tool: z.string(),
  path: z.string().optional().default(''),
  status: z.enum(['started', 'completed', 'error', 'info']),
  details: z.record(z.string(), z.any()).optional().default({}),
});

export type VisualizerEvent = z.infer<typeof VisualizerEventSchema>;

// In-memory event storage (up to 200 events)
const MAX_EVENTS = 200;
const events: VisualizerEvent[] = [];

// SSE listeners
type EventListener = (event: VisualizerEvent) => void;
const listeners = new Set<EventListener>();

// Event deduplication cache
const eventHashes = new Set<string>();
const MAX_HASH_SIZE = 500;

export function hashEvent(event: VisualizerEvent): string {
  const timestampKey = event.timestamp.slice(0, -1);
  return `${event.tool}-${event.path}-${event.status}-${timestampKey}`;
}

export function registerListener(listener: EventListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getRecentEvents(limit = 50): VisualizerEvent[] {
  return events.slice(-limit);
}

export function addEvent(event: VisualizerEvent): boolean {
  const hash = hashEvent(event);
  if (eventHashes.has(hash)) {
    return false; // deduplicated
  }

  eventHashes.add(hash);
  if (eventHashes.size > MAX_HASH_SIZE) {
    const firstHash = eventHashes.values().next().value;
    if (firstHash) eventHashes.delete(firstHash);
  }

  events.push(event);
  if (events.length > MAX_EVENTS) {
    events.shift();
  }

  listeners.forEach((listener) => {
    try {
      listener(event);
    } catch (err) {
      console.error('[Visualizer] Error broadcasting event:', err);
    }
  });

  return true;
}

export function getEventCount(): number {
  return events.length;
}
