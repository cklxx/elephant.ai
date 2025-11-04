/**
 * SSE Connection Hook built on top of a dedicated SSE client and
 * event pipeline.
 *
 * Responsibilities are split as follows:
 * - `SSEClient` manages the browser EventSource connection.
 * - `EventPipeline` validates raw payloads, triggers side effects via
 *   the registry and emits events on the shared bus.
 * - This hook only manages connection state for React consumers and
 *   subscribes to the bus for event updates.
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { agentEventBus } from '@/lib/events/eventBus';
import { defaultEventRegistry } from '@/lib/events/eventRegistry';
import { EventPipeline } from '@/lib/events/eventPipeline';
import { SSEClient } from '@/lib/events/sseClient';

export interface UseSSEOptions {
  enabled?: boolean;
  onEvent?: (event: AnyAgentEvent) => void;
  maxReconnectAttempts?: number;
}

export interface UseSSEReturn {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  clearEvents: () => void;
  reconnect: () => void;
  addEvent: (event: AnyAgentEvent) => void;
}

export function useSSE(
  sessionId: string | null,
  options: UseSSEOptions = {},
): UseSSEReturn {
  const { enabled = true, onEvent, maxReconnectAttempts = 5 } = options;

  const [events, setEvents] = useState<AnyAgentEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const isConnectingRef = useRef(false);
  const clientRef = useRef<SSEClient | null>(null);
  const pipelineRef = useRef<EventPipeline | null>(null);
  const sessionIdRef = useRef(sessionId);

  const onEventRef = useRef(onEvent);
  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const cleanup = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (clientRef.current) {
      clientRef.current.dispose();
      clientRef.current = null;
    }
    isConnectingRef.current = false;
  }, []);

  const connectInternal = useCallback(() => {
    const currentSessionId = sessionIdRef.current;
    const currentEnabled = enabled;
    const pipeline = pipelineRef.current;

    if (!currentSessionId || !currentEnabled || !pipeline) {
      return;
    }

    if (isConnectingRef.current || clientRef.current) {
      return;
    }

    // Check if we've exceeded max attempts - stop reconnecting
    if (reconnectAttemptsRef.current >= maxReconnectAttempts) {
      console.warn('[SSE] Max reconnection attempts reached, stopping auto-reconnect');
      setIsReconnecting(false);
      return;
    }

    isConnectingRef.current = true;

    const client = new SSEClient(currentSessionId, pipeline, {
      onOpen: () => {
        isConnectingRef.current = false;
        setIsConnected(true);
        setIsReconnecting(false);
        setError(null);
        reconnectAttemptsRef.current = 0;
        setReconnectAttempts(0);
      },
      onError: (err) => {
        console.error('[SSE] Connection error:', err);

        // Clean up current client
        if (clientRef.current) {
          clientRef.current.dispose();
          clientRef.current = null;
        }
        isConnectingRef.current = false;
        setIsConnected(false);

        const nextAttempts = reconnectAttemptsRef.current + 1;
        reconnectAttemptsRef.current = nextAttempts;
        const clampedAttempts = Math.min(nextAttempts, maxReconnectAttempts);
        setReconnectAttempts(clampedAttempts);

        if (nextAttempts > maxReconnectAttempts) {
          console.warn('[SSE] Maximum reconnection attempts exceeded');
          setError('Maximum reconnection attempts exceeded. Please refresh the page or click Reconnect.');
          setIsReconnecting(false);
          return;
        }

        // Exponential backoff: 2s, 4s, 8s, 16s, 32s, capped at 60s
        const delay = Math.min(1000 * 2 ** nextAttempts, 60000);
        console.log(`[SSE] Scheduling reconnect attempt ${nextAttempts}/${maxReconnectAttempts} in ${delay}ms`);
        setIsReconnecting(true);

        reconnectTimeoutRef.current = setTimeout(() => {
          connectInternal();
        }, delay);
      },
      onClose: () => {
        setIsConnected(false);
      },
    });

    clientRef.current = client;
    try {
      client.connect();
    } catch (err) {
      console.error('[SSE] Failed to connect:', err);
      isConnectingRef.current = false;
      setIsConnected(false);
      setError(err instanceof Error ? err.message : 'Unknown connection error');
    }
  }, [enabled, maxReconnectAttempts]);

  useEffect(() => {
    pipelineRef.current = new EventPipeline({
      bus: agentEventBus,
      registry: defaultEventRegistry,
      onInvalidEvent: (raw, validationError) => {
        // Log as warning instead of error for better UX
        // Unknown/invalid events should not break the application
        console.warn('[SSE] Event validation failed (skipping):', {
          raw,
          error: validationError,
          note: 'This event will be skipped. This is usually harmless.',
        });
      },
    });

    const unsubscribe = agentEventBus.subscribe((event) => {
      setEvents((prev) => [...prev, event]);
      onEventRef.current?.(event);
    });

    return () => {
      unsubscribe();
      pipelineRef.current = null;
    };
  }, []);

  useEffect(() => {
    cleanup();
    setEvents([]);
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    setError(null);

    if (sessionId && enabled) {
      connectInternal();
    }

    return () => {
      cleanup();
    };
  }, [sessionId, enabled, connectInternal, cleanup]);

  const reconnect = useCallback(() => {
    cleanup();
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    connectInternal();
  }, [cleanup, connectInternal]);

  const addEvent = useCallback((event: AnyAgentEvent) => {
    defaultEventRegistry.run(event);
    agentEventBus.emit(event);
  }, []);

  return {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents,
    reconnect,
    addEvent,
  };
}
