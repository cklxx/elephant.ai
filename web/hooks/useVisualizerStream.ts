'use client';

import { useEffect, useState, useRef } from 'react';

export interface VisualizerEvent {
  timestamp: string;
  event: string;
  tool: string;
  path?: string;
  status: 'started' | 'completed' | 'error' | 'info';
  details?: Record<string, any>;
}

export interface UseVisualizerStreamResult {
  events: VisualizerEvent[];
  isConnected: boolean;
  currentEvent: VisualizerEvent | null;
}

export function useVisualizerStream(): UseVisualizerStreamResult {
  const [events, setEvents] = useState<VisualizerEvent[]>([]);
  const [currentEvent, setCurrentEvent] = useState<VisualizerEvent | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const eventSource = new EventSource('/api/visualizer/stream');
    eventSourceRef.current = eventSource;

    eventSource.onopen = () => {
      console.log('[VisualizerStream] Connected');
      setIsConnected(true);
    };

    eventSource.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);

        // Filter out heartbeat and connection messages
        if (data.type === 'heartbeat' || data.type === 'connected') {
          return;
        }

        // Update events list (keep last 100)
        setEvents((prev) => [...prev.slice(-99), data]);

        // Update current event
        setCurrentEvent(data);

        // Clear current event after 3 seconds
        setTimeout(() => {
          setCurrentEvent((current) => (current === data ? null : current));
        }, 3000);
      } catch (err) {
        console.error('[VisualizerStream] Parse error:', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('[VisualizerStream] Error:', err);
      setIsConnected(false);
    };

    return () => {
      eventSource.close();
      setIsConnected(false);
    };
  }, []);

  return { events, isConnected, currentEvent };
}
