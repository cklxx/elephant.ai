import { NextRequest, NextResponse } from 'next/server';
import { z } from 'zod';

// Event validation schema
const VisualizerEventSchema = z.object({
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

function hashEvent(event: VisualizerEvent): string {
  // Hash based on tool, path, status, and timestamp (rounded to second)
  const timestampKey = event.timestamp.slice(0, -1); // Remove last digit
  return `${event.tool}-${event.path}-${event.status}-${timestampKey}`;
}

export function registerListener(listener: EventListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getRecentEvents(limit = 50): VisualizerEvent[] {
  return events.slice(-limit);
}

export async function POST(request: NextRequest) {
  try {
    const rawEvent = await request.json();

    // Validate event structure
    const event = VisualizerEventSchema.parse(rawEvent);

    // Deduplicate events
    const hash = hashEvent(event);
    if (eventHashes.has(hash)) {
      return NextResponse.json({ success: true, deduplicated: true }, { status: 200 });
    }

    // Add to cache
    eventHashes.add(hash);
    if (eventHashes.size > MAX_HASH_SIZE) {
      const firstHash = eventHashes.values().next().value;
      if (firstHash) eventHashes.delete(firstHash);
    }

    // Store event
    events.push(event);
    if (events.length > MAX_EVENTS) {
      events.shift();
    }

    // Broadcast to all listeners
    listeners.forEach((listener) => {
      try {
        listener(event);
      } catch (err) {
        console.error('[Visualizer] Error broadcasting event:', err);
      }
    });

    console.log('[Visualizer] Event received:', {
      tool: event.tool,
      path: event.path,
      status: event.status,
      timestamp: event.timestamp,
    });

    return NextResponse.json({ success: true }, { status: 200 });
  } catch (error) {
    if (error instanceof z.ZodError) {
      console.error('[Visualizer] Validation error:', error.issues);
      return NextResponse.json(
        { error: 'Invalid event format', details: error.issues },
        { status: 400 }
      );
    }

    console.error('[Visualizer] Error processing event:', error);
    return NextResponse.json(
      { error: 'Failed to process event' },
      { status: 500 }
    );
  }
}

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const limit = parseInt(searchParams.get('limit') || '50', 10);

  return NextResponse.json({
    events: getRecentEvents(limit),
    count: events.length,
  });
}
