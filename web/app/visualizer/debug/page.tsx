'use client';

import { useEffect, useState } from 'react';

export default function VisualizerDebugPage() {
  const [events, setEvents] = useState<any[]>([]);
  const [sseStatus, setSSEStatus] = useState<string>('未连接');
  const [apiEvents, setAPIEvents] = useState<any[]>([]);

  // Test direct API call
  useEffect(() => {
    fetch('/api/visualizer/events?limit=20')
      .then((res) => res.json())
      .then((data) => {
        console.log('[Debug] API returned:', data);
        setAPIEvents(data.events || []);
      })
      .catch((err) => console.error('[Debug] API error:', err));
  }, []);

  // Test SSE connection
  useEffect(() => {
    console.log('[Debug] Starting SSE connection');
    const eventSource = new EventSource('/api/visualizer/stream');

    eventSource.onopen = () => {
      console.log('[Debug] SSE opened');
      setSSEStatus('已连接');
    };

    eventSource.onmessage = (e) => {
      console.log('[Debug] SSE message:', e.data);
      try {
        const data = JSON.parse(e.data);
        setEvents((prev) => [...prev, { time: new Date().toLocaleTimeString(), data }]);
      } catch (err) {
        console.error('[Debug] Parse error:', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('[Debug] SSE error:', err);
      setSSEStatus('连接失败');
    };

    return () => {
      console.log('[Debug] Closing SSE');
      eventSource.close();
    };
  }, []);

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-6xl mx-auto space-y-6">
        <h1 className="text-3xl font-bold">Visualizer Debug Panel</h1>

        {/* SSE Status */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">SSE Connection</h2>
          <div className="flex items-center gap-3">
            <div
              className={`w-4 h-4 rounded-full ${
                sseStatus === '已连接' ? 'bg-green-500' : 'bg-red-500'
              }`}
            />
            <span className="font-medium">{sseStatus}</span>
          </div>
        </div>

        {/* API Events */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">API Events (GET /api/visualizer/events)</h2>
          <div className="text-sm text-gray-600 mb-2">
            Total: {apiEvents.length} events
          </div>
          <div className="space-y-2 max-h-96 overflow-auto">
            {apiEvents.length === 0 && (
              <div className="text-gray-500 italic">No events in API</div>
            )}
            {apiEvents.map((event, i) => (
              <div key={i} className="bg-gray-50 p-3 rounded text-xs font-mono">
                {JSON.stringify(event, null, 2)}
              </div>
            ))}
          </div>
        </div>

        {/* SSE Events */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">SSE Stream Events (Real-time)</h2>
          <div className="text-sm text-gray-600 mb-2">
            Received: {events.length} messages
          </div>
          <div className="space-y-2 max-h-96 overflow-auto">
            {events.length === 0 && (
              <div className="text-gray-500 italic">Waiting for SSE messages...</div>
            )}
            {events
              .slice()
              .reverse()
              .map((event, i) => (
                <div key={i} className="bg-blue-50 p-3 rounded">
                  <div className="text-xs text-gray-500 mb-1">{event.time}</div>
                  <div className="text-xs font-mono">
                    {JSON.stringify(event.data, null, 2)}
                  </div>
                </div>
              ))}
          </div>
        </div>

        {/* Test Actions */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Test Actions</h2>
          <div className="space-y-3">
            <button
              onClick={async () => {
                const res = await fetch('/api/visualizer/events', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({
                    timestamp: new Date().toISOString(),
                    event: 'tool-use',
                    tool: 'Read',
                    path: '/test/file.ts',
                    status: 'started',
                    details: {},
                  }),
                });
                const data = await res.json();
                alert(`Event sent: ${JSON.stringify(data)}`);
              }}
              className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
            >
              Send Test Event
            </button>

            <button
              onClick={() => {
                fetch('/api/visualizer/events?limit=20')
                  .then((res) => res.json())
                  .then((data) => setAPIEvents(data.events || []));
              }}
              className="ml-3 px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600"
            >
              Refresh API Events
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
