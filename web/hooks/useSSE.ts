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
  // UI state - shows current reconnection attempt count
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  // Refs for connection management (no re-render triggers)
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  // Use ref for reconnection logic to avoid dependency cycle
  // This tracks the ACTUAL attempt count without triggering re-renders
  const reconnectAttemptsRef = useRef(0);
  // Track if a connection is currently being established
  const isConnectingRef = useRef(false);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const disconnect = useCallback(() => {
    // Clear any pending reconnection attempts
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    // Close active EventSource connection
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    // Reset connection state
    isConnectingRef.current = false;
    setIsConnected(false);
    setIsReconnecting(false);
  }, []);

  const connect = useCallback(() => {
    if (!sessionId || !enabled) return;

    // Prevent double connections - if already connecting, skip
    if (isConnectingRef.current) {
      console.log('[SSE] Connection already in progress, skipping duplicate attempt');
      return;
    }

    // Mark as connecting BEFORE cleanup to prevent race conditions
    isConnectingRef.current = true;

    // Cleanup any existing connection/timers before establishing new one
    // This ensures only ONE connection attempt happens at a time
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    try {
      const eventSource = apiClient.createSSEConnection(sessionId);
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log('[SSE] Connected to session:', sessionId);
        isConnectingRef.current = false;
        setIsConnected(true);
        setIsReconnecting(false);
        setError(null);
        // Reset both state and ref on successful connection
        reconnectAttemptsRef.current = 0;
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
        // Manus-style events (Phase 3)
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
            onEvent?.(event);
          } catch (err) {
            console.error(`[SSE] Failed to parse event ${type}:`, err);
          }
        });
      });

      eventSource.onerror = (err) => {
        console.error('[SSE] Connection error:', err);
        isConnectingRef.current = false;
        setIsConnected(false);
        eventSource.close();

        // Use REF for reconnection logic to avoid dependency cycle
        // This prevents the connect function from being recreated on every attempt
        const currentAttempts = reconnectAttemptsRef.current;

        // Attempt reconnection with exponential backoff
        if (currentAttempts < maxReconnectAttempts) {
          // Exponential backoff: 1s, 2s, 4s, 8s, 16s, capped at 30s
          const delay = Math.min(1000 * Math.pow(2, currentAttempts), 30000);
          console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${currentAttempts + 1}/${maxReconnectAttempts})`);

          setIsReconnecting(true);
          // Increment both ref (for logic) and state (for UI)
          reconnectAttemptsRef.current = currentAttempts + 1;
          setReconnectAttempts(currentAttempts + 1);

          // Schedule reconnection - this will NOT trigger useEffect
          // because connect() is stable (no dependency on reconnectAttempts state)
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
      isConnectingRef.current = false;
      setError(err instanceof Error ? err.message : 'Failed to connect');
    }
  }, [sessionId, enabled, maxReconnectAttempts, onEvent]);

  const reconnect = useCallback(() => {
    // Manual reconnect - reset all counters and errors
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    setError(null);
    connect();
  }, [connect]);

  // Main connection effect - only triggers on sessionId or enabled changes
  // NOT on reconnectAttempts changes (which was the bug)
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
