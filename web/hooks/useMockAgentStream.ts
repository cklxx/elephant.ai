import { useCallback, useEffect, useRef, useState } from 'react';
import { AnyAgentEvent, WorkflowNodeOutputDeltaEvent, eventMatches } from '@/lib/types';
import { UseSSEOptions, UseSSEReturn } from './useSSE';
import {
  createMockEventSequence,
  TimedMockEvent,
} from '@/lib/mocks/mockAgentEvents';
import { defaultEventRegistry } from '@/lib/events/eventRegistry';
import { resetAttachmentRegistry } from '@/lib/events/attachmentRegistry';

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

  const timeoutsRef = useRef<Array<ReturnType<typeof setTimeout>>>([]);
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
          const eventTimestamp = new Date(start + delay).toISOString();
          const timestampedEvent = {
            ...event,
            timestamp: eventTimestamp,
          } as AnyAgentEvent;

          if (eventMatches(timestampedEvent, 'workflow.node.output.delta', 'workflow.node.output.delta')) {
            const assistantEvent = timestampedEvent as WorkflowNodeOutputDeltaEvent;
            if (!assistantEvent.created_at) {
              assistantEvent.created_at = eventTimestamp;
            }
          }

          defaultEventRegistry.run(timestampedEvent);
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
    resetAttachmentRegistry();
  }, []);

  const addEvent = useCallback((event: AnyAgentEvent) => {
    if (event.event_type === 'workflow.input.received' && 'task' in event) {
      lastUserTaskRef.current = event.task;
    }
    if (eventMatches(event, 'workflow.node.output.delta', 'workflow.node.output.delta')) {
      const assistantEvent = event as WorkflowNodeOutputDeltaEvent;
      if (!assistantEvent.created_at) {
        assistantEvent.created_at = assistantEvent.timestamp ?? new Date().toISOString();
      }
    }
    defaultEventRegistry.run(event);
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
      resetAttachmentRegistry();
      setIsConnected(false);
      setIsReconnecting(false);
      setReconnectAttempts(0);
      return;
    }

    const sequence = createMockEventSequence(lastUserTaskRef.current);
    setEvents([]);
    resetAttachmentRegistry();
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
