import { NextRequest } from 'next/server';
import { registerListener, getRecentEvents, VisualizerEvent } from '../events/route';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function GET(request: NextRequest) {
  // Create SSE stream
  const stream = new ReadableStream({
    start(controller) {
      const encoder = new TextEncoder();

      // Send initial connection message
      const sendMessage = (data: any) => {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(data)}\n\n`));
      };

      // Send recent events as initial state
      const recentEvents = getRecentEvents(50);
      recentEvents.forEach((event) => sendMessage(event));

      sendMessage({ type: 'connected', timestamp: new Date().toISOString() });

      // Register listener for new events
      const unregister = registerListener((event: VisualizerEvent) => {
        try {
          sendMessage(event);
        } catch (err) {
          console.error('[Visualizer SSE] Error sending event:', err);
        }
      });

      // Heartbeat to keep connection alive
      const heartbeatInterval = setInterval(() => {
        try {
          sendMessage({ type: 'heartbeat', timestamp: new Date().toISOString() });
        } catch (err) {
          console.error('[Visualizer SSE] Heartbeat error:', err);
          clearInterval(heartbeatInterval);
        }
      }, 30000); // 30 seconds

      // Cleanup on connection close
      request.signal.addEventListener('abort', () => {
        clearInterval(heartbeatInterval);
        unregister();
        try {
          controller.close();
        } catch (err) {
          // Controller may already be closed
        }
      });
    },
  });

  return new Response(stream, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache, no-transform',
      'Connection': 'keep-alive',
      'X-Accel-Buffering': 'no',
    },
  });
}
