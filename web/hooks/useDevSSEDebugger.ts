"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type Dispatch,
  type SetStateAction,
} from "react";

import { apiClient, type SSEReplayMode } from "@/lib/api";

export type DevSSEDebugEvent = {
  id: string;
  eventType: string;
  receivedAt: string;
  raw: string;
  parsed: unknown | null;
  parseError?: string;
  lastEventId?: string;
};

type CreateDevSSEConnection = (params: {
  sessionId: string;
  replayMode: SSEReplayMode;
}) => EventSource;

type UseDevSSEDebuggerOptions = {
  eventTypes: readonly string[];
  createConnection?: CreateDevSSEConnection;
  defaultReplayMode?: SSEReplayMode;
  defaultMaxEvents?: number;
};

type UseDevSSEDebuggerResult = {
  sessionIdInput: string;
  setSessionIdInput: Dispatch<SetStateAction<string>>;
  sessionId: string;
  replayMode: SSEReplayMode;
  setReplayMode: Dispatch<SetStateAction<SSEReplayMode>>;
  events: DevSSEDebugEvent[];
  selectedId: string | null;
  setSelectedId: Dispatch<SetStateAction<string | null>>;
  maxEvents: number;
  maxEventsInput: string;
  handleMaxEventsChange: (event: ChangeEvent<HTMLInputElement>) => void;
  autoScroll: boolean;
  setAutoScroll: Dispatch<SetStateAction<boolean>>;
  isConnected: boolean;
  isConnecting: boolean;
  error: string | null;
  connect: () => void;
  disconnect: () => void;
  clearEvents: () => void;
};

function parsePayload(raw: string) {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { parsed: null as unknown, error: undefined as string | undefined };
  }

  try {
    return { parsed: JSON.parse(trimmed) as unknown, error: undefined as string | undefined };
  } catch (error) {
    return {
      parsed: null as unknown,
      error: error instanceof Error ? error.message : "Invalid JSON",
    };
  }
}

const DEFAULT_REPLAY_MODE: SSEReplayMode = "session";
const DEFAULT_MAX_EVENTS = 2000;

export function useDevSSEDebugger(options: UseDevSSEDebuggerOptions): UseDevSSEDebuggerResult {
  const {
    eventTypes,
    createConnection,
    defaultReplayMode = DEFAULT_REPLAY_MODE,
    defaultMaxEvents = DEFAULT_MAX_EVENTS,
  } = options;

  const [sessionIdInput, setSessionIdInput] = useState("");
  const [replayMode, setReplayMode] = useState<SSEReplayMode>(defaultReplayMode);
  const [events, setEvents] = useState<DevSSEDebugEvent[]>([]);
  const [maxEvents, setMaxEvents] = useState(defaultMaxEvents);
  const [maxEventsInput, setMaxEventsInput] = useState(String(defaultMaxEvents));
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const eventSourceRef = useRef<EventSource | null>(null);
  const idRef = useRef(0);
  const lastConnectionRef = useRef<{ sessionId: string; replayMode: SSEReplayMode } | null>(
    null,
  );

  const sessionId = sessionIdInput.trim();

  const handleMaxEventsChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    setMaxEventsInput(value);
    const parsed = Number(value);
    if (Number.isFinite(parsed) && parsed >= 0) {
      const nextMaxEvents = Math.floor(parsed);
      setMaxEvents(nextMaxEvents);
      if (nextMaxEvents > 0) {
        setEvents((previous) => {
          if (previous.length <= nextMaxEvents) {
            return previous;
          }
          const trimmed = previous.slice(-nextMaxEvents);
          setSelectedId((current) => {
            if (!current) {
              return null;
            }
            if (trimmed.some((entry) => entry.id === current)) {
              return current;
            }
            return trimmed.length > 0 ? trimmed[trimmed.length - 1].id : null;
          });
          return trimmed;
        });
      }
    }
  }, []);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setIsConnecting(false);
    setIsConnected(false);
  }, []);

  const clearEvents = useCallback(() => {
    setEvents([]);
    setSelectedId(null);
  }, []);

  const handleIncomingEvent = useCallback(
    (eventType: string, rawEvent: MessageEvent) => {
      const rawData =
        typeof rawEvent.data === "string" ? rawEvent.data : JSON.stringify(rawEvent.data);
      const { parsed, error: parseError } = parsePayload(rawData);
      const nextId = `sse-${Date.now()}-${(idRef.current += 1)}`;
      const entry: DevSSEDebugEvent = {
        id: nextId,
        eventType,
        receivedAt: new Date().toISOString(),
        raw: rawData,
        parsed: parseError ? null : parsed,
        parseError,
        lastEventId: rawEvent.lastEventId || undefined,
      };

      setEvents((previous) => {
        const next = [...previous, entry];
        if (maxEvents > 0 && next.length > maxEvents) {
          next.splice(0, next.length - maxEvents);
        }

        setSelectedId((current) => {
          if (!current) {
            return nextId;
          }
          if (next.some((candidate) => candidate.id === current)) {
            return current;
          }
          return next.length > 0 ? next[next.length - 1].id : null;
        });

        return next;
      });
    },
    [maxEvents],
  );

  const makeConnection = useCallback<CreateDevSSEConnection>(
    ({ sessionId: nextSessionId, replayMode: nextReplayMode }) => {
      if (createConnection) {
        return createConnection({ sessionId: nextSessionId, replayMode: nextReplayMode });
      }
      return apiClient.createSSEConnection(nextSessionId, undefined, {
        replay: nextReplayMode,
        debug: true,
      });
    },
    [createConnection],
  );

  const connect = useCallback(() => {
    if (!sessionId) {
      setError("Session ID is required.");
      return;
    }

    const previous = lastConnectionRef.current;
    if (!previous || previous.sessionId !== sessionId || previous.replayMode !== replayMode) {
      clearEvents();
    }
    lastConnectionRef.current = { sessionId, replayMode };

    disconnect();
    setError(null);
    setIsConnected(false);
    setIsConnecting(true);

    const source = makeConnection({ sessionId, replayMode });

    source.onopen = () => {
      setIsConnecting(false);
      setIsConnected(true);
    };

    source.onerror = () => {
      setIsConnecting(false);
      setIsConnected(false);
      setError("SSE connection error.");
    };

    eventTypes.forEach((type) => {
      source.addEventListener(type, (event) => handleIncomingEvent(type, event as MessageEvent));
    });

    source.onmessage = (event) => {
      handleIncomingEvent("message", event);
    };
    eventSourceRef.current = source;
  }, [clearEvents, disconnect, eventTypes, handleIncomingEvent, makeConnection, replayMode, sessionId]);

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    sessionIdInput,
    setSessionIdInput,
    sessionId,
    replayMode,
    setReplayMode,
    events,
    selectedId,
    setSelectedId,
    maxEvents,
    maxEventsInput,
    handleMaxEventsChange,
    autoScroll,
    setAutoScroll,
    isConnected,
    isConnecting,
    error,
    connect,
    disconnect,
    clearEvents,
  };
}
