/**
 * SSE Connection Hook with Automatic Reconnection
 *
 * Manages Server-Sent Events (SSE) connection lifecycle with:
 * - Automatic reconnection with exponential backoff
 * - Event stream management and parsing
 * - Connection state tracking
 * - Memory leak prevention through proper cleanup
 *
 * @example
 * ```tsx
 * const { events, isConnected, reconnect } = useSSE(sessionId, {
 *   enabled: true,
 *   maxReconnectAttempts: 5,
 *   onEvent: (event) => console.log('Event received:', event)
 * });
 * ```
 */

import { useEffect, useRef, useState, useCallback } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { apiClient } from '@/lib/api';
import { safeValidateEvent } from '@/lib/schemas';

export interface UseSSEOptions {
  enabled?: boolean;
  /** Callback fired when an event is received */
  onEvent?: (event: AnyAgentEvent) => void;
  /** Maximum number of reconnection attempts before giving up */
  maxReconnectAttempts?: number;
}

export interface UseSSEReturn {
  events: AnyAgentEvent[];
  /** Whether the SSE connection is currently active */
  isConnected: boolean;
  /** Whether a reconnection attempt is in progress */
  isReconnecting: boolean;
  /** Current error message, if any */
  error: string | null;
  /** Number of reconnection attempts made */
  reconnectAttempts: number;
  /** Clear all accumulated events */
  clearEvents: () => void;
  /** Manually trigger a reconnection */
  reconnect: () => void;
  /** Manually add an event to the stream */
  addEvent: (event: AnyAgentEvent) => void;
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
  const pendingReconnectRef = useRef(false);
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

  /**
   * Clear all accumulated events
   */
  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  /**
   * Manually add an event to the event stream
   */
  const addEvent = useCallback((event: AnyAgentEvent) => {
    setEvents((prev) => [...prev, event]);
  }, []);

  /**
   * Cleanup function - closes connection and clears timeouts
   * Note: This is NOT used in useEffect dependencies to avoid circular deps
   */
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

  /**
   * Internal connection function - uses refs for stable dependencies
   * This function is wrapped in useCallback with minimal deps to prevent unnecessary re-creation
   */
  const connectInternal = useCallback(() => {
    const currentSessionId = sessionIdRef.current;
    const currentEnabled = enabled;
    const currentMaxAttempts = maxReconnectAttempts;

    if (!currentSessionId || !currentEnabled) {
      pendingReconnectRef.current = false;
      return;
    }

    // Prevent double connections
    if (isConnectingRef.current || eventSourceRef.current) {
      pendingReconnectRef.current = false;
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

      // All supported event types from the agent
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
            const parsed = JSON.parse(e.data);
            const validationResult = safeValidateEvent(parsed);

            if (!validationResult.success) {
              console.error(`[SSE] Event validation failed for ${type}:`, validationResult.error.format());
              // Still add the event but log the validation error
              // This allows graceful degradation if backend sends unexpected data
              const event = parsed as AnyAgentEvent;
              setEvents((prev) => [...prev, event]);
              onEventRef.current?.(event);
              return;
            }

            // Validated event
            const event = validationResult.data;
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

        if (currentAttempts < currentMaxAttempts) {
          // Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s
          const delay = Math.min(1000 * Math.pow(2, currentAttempts), 30000);
          console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${currentAttempts + 1}/${currentMaxAttempts})`);

          setIsReconnecting(true);
          reconnectAttemptsRef.current = currentAttempts + 1;
          setReconnectAttempts(currentAttempts + 1);

          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectTimeoutRef.current = null;
            isConnectingRef.current = false;
            // Recursive call for retry
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
    } finally {
      pendingReconnectRef.current = false;
    }
  }, [enabled, maxReconnectAttempts]);

  /**
   * Manually trigger a reconnection
   * Resets reconnection counter and immediately attempts to connect
   */
  const reconnect = useCallback(() => {
    if (isConnectingRef.current || pendingReconnectRef.current) {
      return;
    }

    pendingReconnectRef.current = true;
    reconnectAttemptsRef.current = 0;
    setReconnectAttempts(0);
    setError(null);
    setIsReconnecting(false);
    cleanup();
    connectInternal();
  }, [cleanup, connectInternal]);

  /**
   * Main connection effect
   *
   * IMPORTANT: This effect is intentionally simple with minimal dependencies
   * to avoid the circular dependency issue that existed before (lines 182-195).
   *
   * Previous issue: Including cleanup and connectInternal in deps caused:
   * 1. Effect runs → creates new cleanup/connect functions
   * 2. New functions trigger effect again
   * 3. Unnecessary reconnections on every render
   *
   * Solution: Use refs for dynamic values and keep deps minimal (sessionId, enabled only)
   */
  useEffect(() => {
    if (!sessionId || !enabled) {
      // Inline cleanup to avoid dependency
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      isConnectingRef.current = false;
      setIsConnected(false);
      setIsReconnecting(false);
      return;
    }

    // Initial connection
    connectInternal();

    // Cleanup on unmount or when sessionId/enabled changes
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      isConnectingRef.current = false;
    };
    // Only depend on sessionId and enabled - connectInternal is stable via refs
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, enabled]);

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
