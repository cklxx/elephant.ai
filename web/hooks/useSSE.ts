// SSE connection hook with automatic reconnection

import { useEffect, useRef, useState, useCallback } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { apiClient } from '@/lib/api';

interface UseSSEOptions {
  enabled?: boolean;
  onEvent?: (event: AnyAgentEvent) => void;
  maxReconnectAttempts?: number;
}

interface UseSSEReturn {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  clearEvents: () => void;
  reconnect: () => void;
}

export function useSSE(
  sessionId: string | null,
  options: UseSSEOptions = {}
): UseSSEReturn {
  const {
    enabled = true,
    onEvent,
    maxReconnectAttempts = 5,
  } = options;

  const [events, setEvents] = useState<AnyAgentEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    setIsConnected(false);
    setIsReconnecting(false);
  }, []);

  const connect = useCallback(() => {
    if (!sessionId || !enabled) return;

    disconnect();

    try {
      const eventSource = apiClient.createSSEConnection(sessionId);
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log('[SSE] Connected to session:', sessionId);
        setIsConnected(true);
        setIsReconnecting(false);
        setError(null);
        setReconnectAttempts(0);
      };

      // Listen to all event types from the backend
      const eventTypes = [
        'task_analysis',
        'iteration_start',
        'thinking',
        'think_complete',
        'tool_call_start',
        'tool_call_stream',
        'tool_call_complete',
        'iteration_complete',
        'task_complete',
        'error',
      ];

      eventTypes.forEach((type) => {
        eventSource.addEventListener(type, (e: MessageEvent) => {
          try {
            const event = JSON.parse(e.data) as AnyAgentEvent;
            setEvents((prev) => [...prev, event]);
            onEvent?.(event);
          } catch (err) {
            console.error(`[SSE] Failed to parse event ${type}:`, err);
          }
        });
      });

      eventSource.onerror = (err) => {
        console.error('[SSE] Connection error:', err);
        setIsConnected(false);
        eventSource.close();

        // Attempt reconnection with exponential backoff
        if (reconnectAttempts < maxReconnectAttempts) {
          const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
          console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${reconnectAttempts + 1}/${maxReconnectAttempts})`);

          setIsReconnecting(true);
          setReconnectAttempts((prev) => prev + 1);

          reconnectTimeoutRef.current = setTimeout(() => {
            connect();
          }, delay);
        } else {
          setError('Max reconnection attempts reached');
          setIsReconnecting(false);
        }
      };
    } catch (err) {
      console.error('[SSE] Failed to create connection:', err);
      setError(err instanceof Error ? err.message : 'Failed to connect');
    }
  }, [sessionId, enabled, reconnectAttempts, maxReconnectAttempts, onEvent, disconnect]);

  const reconnect = useCallback(() => {
    setReconnectAttempts(0);
    setError(null);
    connect();
  }, [connect]);

  useEffect(() => {
    if (sessionId && enabled) {
      connect();
    }

    return () => {
      disconnect();
    };
  }, [sessionId, enabled, connect, disconnect]);

  return {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents,
    reconnect,
  };
}
