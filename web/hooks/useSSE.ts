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

  // Refs for connection management
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const isConnectingRef = useRef(false);
  const sessionIdRef = useRef(sessionId);

  // Update session ID ref
  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  // Store onEvent callback in ref to avoid dependency issues
  const onEventRef = useRef(onEvent);
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  // Cleanup function
  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    isConnectingRef.current = false;
  }, []);

  //  Connect function - stable, no dependencies
  const connectInternal = useCallback(() => {
    const currentSessionId = sessionIdRef.current;

    if (!currentSessionId || !enabled) {
      return;
    }

    // Prevent double connections
    if (isConnectingRef.current || eventSourceRef.current) {
      return;
    }

    isConnectingRef.current = true;

    try {
      const eventSource = apiClient.createSSEConnection(currentSessionId);
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log('[SSE] Connected to session:', currentSessionId);
        isConnectingRef.current = false;
        setIsConnected(true);
        setIsReconnecting(false);
        setError(null);
        reconnectAttemptsRef.current = 0;
        setReconnectAttempts(0);
      };

      // Listen to all event types
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
        'research_plan',
        'step_started',
        'step_completed',
        'browser_snapshot',
      ];

      eventTypes.forEach((type) => {
        eventSource.addEventListener(type, (e: MessageEvent) => {
          try {
            const event = JSON.parse(e.data) as AnyAgentEvent;
            setEvents((prev) => [...prev, event]);
            onEventRef.current?.(event);
          } catch (err) {
            console.error(`[SSE] Failed to parse event ${type}:`, err);
          }
        });
      });

      eventSource.onerror = () => {
        console.error('[SSE] Connection error');
        isConnectingRef.current = false;
        setIsConnected(false);
        eventSource.close();
        eventSourceRef.current = null;

        const currentAttempts = reconnectAttemptsRef.current;

        if (currentAttempts < maxReconnectAttempts) {
          const delay = Math.min(1000 * Math.pow(2, currentAttempts), 30000);
          console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${currentAttempts + 1}/${maxReconnectAttempts})`);

          setIsReconnecting(true);
          reconnectAttemptsRef.current = currentAttempts + 1;
          setReconnectAttempts(currentAttempts + 1);

          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectTimeoutRef.current = null;
            isConnectingRef.current = false;
            connectInternal();
          }, delay);
        } else {
          setError('Max reconnection attempts reached');
          setIsReconnecting(false);
        }
      };
    } catch (err) {
      console.error('[SSE] Failed to create connection:', err);
      isConnectingRef.current = false;
      setError(err instanceof Error ? err.message : 'Failed to connect');
    }
  }, [enabled, maxReconnectAttempts]);

  const reconnect = useCallback(() => {
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    setError(null);
    setIsReconnecting(false);
    cleanup();
    connectInternal();
  }, [cleanup, connectInternal]);

  // Main connection effect - only triggers on sessionId or enabled changes
  useEffect(() => {
    if (!sessionId || !enabled) {
      cleanup();
      setIsConnected(false);
      setIsReconnecting(false);
      return;
    }

    connectInternal();

    return () => {
      cleanup();
    };
  }, [sessionId, enabled, cleanup, connectInternal]);

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
