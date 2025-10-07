import { useCallback, useEffect, useRef, useState } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { UseSSEOptions, UseSSEReturn } from './useSSE';
import {
  createMockEventSequence,
  TimedMockEvent,
} from '@/lib/mocks/mockAgentEvents';

type MockWindowControls = {
  pushEvent: (event: Partial<AnyAgentEvent> & { event_type: AnyAgentEvent['event_type'] }) => void;
  dropConnection: () => void;
  restart: () => void;
  clear: () => void;
};

export function useMockAgentStream(
  sessionId: string | null,
  options: UseSSEOptions = {}
): UseSSEReturn {
  const enabled = options.enabled ?? true;

  const [events, setEvents] = useState<AnyAgentEvent[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [isReconnecting, setIsReconnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const [reconnectToken, setReconnectToken] = useState(0);

  const timeoutsRef = useRef<NodeJS.Timeout[]>([]);
  const onEventRef = useRef(options.onEvent);
  const lastUserTaskRef = useRef<string>('Analyze the project repository');

  useEffect(() => {
    onEventRef.current = options.onEvent;
  }, [options.onEvent]);

  const clearTimers = useCallback(() => {
    const timers = timeoutsRef.current;
    timers.forEach((timeoutId) => clearTimeout(timeoutId));
    timers.length = 0;
  }, []);

  const scheduleSequence = useCallback(
    (sequence: TimedMockEvent[]) => {
      clearTimers();

      const start = Date.now();
      sequence.forEach(({ delay, event }) => {
        const timeoutId = setTimeout(() => {
          const timestampedEvent = {
            ...event,
            timestamp: new Date(start + delay).toISOString(),
          } as AnyAgentEvent;

          setEvents((prev) => [...prev, timestampedEvent]);
          onEventRef.current?.(timestampedEvent);
        }, delay);

        timeoutsRef.current.push(timeoutId);
      });
    },
    [clearTimers]
  );

  const clearEvents = useCallback(() => {
    setEvents([]);
  }, []);

  const addEvent = useCallback((event: AnyAgentEvent) => {
    if (event.event_type === 'user_task' && 'task' in event) {
      lastUserTaskRef.current = event.task;
    }
    setEvents((prev) => [...prev, event]);
  }, []);

  const reconnect = useCallback(() => {
    if (!enabled) return;

    setIsReconnecting(true);
    setIsConnected(false);
    setError(null);
    setReconnectAttempts((prev) => prev + 1);
    setReconnectToken((token) => token + 1);
  }, [enabled]);

  const dropConnection = useCallback(() => {
    setIsConnected(false);
    setIsReconnecting(true);
    setError('Mock stream disconnected');
    clearTimers();
    setReconnectAttempts((prev) => prev + 1);
    setReconnectToken((token) => token + 1);
  }, [clearTimers]);

  useEffect(() => {
    if (!enabled) {
      clearTimers();
      setIsConnected(false);
      setIsReconnecting(false);
      return;
    }

    if (!sessionId) {
      clearTimers();
      setEvents([]);
      setIsConnected(false);
      setIsReconnecting(false);
      setReconnectAttempts(0);
      return;
    }

    const sequence = createMockEventSequence(lastUserTaskRef.current);
    setEvents([]);
    setError(null);
    setIsConnected(true);
    setIsReconnecting(false);

    if (reconnectToken === 0) {
      setReconnectAttempts(0);
    }

    scheduleSequence(sequence);

    return () => {
      clearTimers();
    };
  }, [sessionId, enabled, reconnectToken, scheduleSequence, clearTimers]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const controls: MockWindowControls = {
      pushEvent: (event) => {
        const timestamped: AnyAgentEvent = {
          ...(event as AnyAgentEvent),
          timestamp: event.timestamp ?? new Date().toISOString(),
        };

        timestamped.agent_level ??= 'core';
        addEvent(timestamped);
      },
      dropConnection: () => {
        dropConnection();
      },
      restart: () => {
        setReconnectAttempts(0);
        setReconnectToken((token) => token + 1);
      },
      clear: () => {
        clearEvents();
      },
    };

    (window as unknown as { __ALEX_MOCK_STREAM__?: MockWindowControls }).__ALEX_MOCK_STREAM__ = controls;

    return () => {
      const win = window as unknown as { __ALEX_MOCK_STREAM__?: MockWindowControls };
      if (win.__ALEX_MOCK_STREAM__ === controls) {
        delete win.__ALEX_MOCK_STREAM__;
      }
    };
  }, [addEvent, dropConnection, clearEvents]);

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
