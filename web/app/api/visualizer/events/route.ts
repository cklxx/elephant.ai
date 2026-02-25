import { NextRequest, NextResponse } from 'next/server';
import { z } from 'zod';

import { VisualizerEventSchema, addEvent, getRecentEvents, getEventCount } from './_store';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function POST(request: NextRequest) {
  try {
    const rawEvent = await request.json();
    const event = VisualizerEventSchema.parse(rawEvent);

    const isNew = addEvent(event);

    if (!isNew) {
      return NextResponse.json({ success: true, deduplicated: true }, { status: 200 });
    }

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
    count: getEventCount(),
  });
}
