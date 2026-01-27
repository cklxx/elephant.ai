"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import { Play, RefreshCw, Square, Trash2 } from "lucide-react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { useSessionStore } from "@/hooks/useSessionStore";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient, type SSEReplayMode, type SessionSnapshotsResponse } from "@/lib/api";
import { type LogTraceBundle, type WorkflowEventType } from "@/lib/types";
import { cn } from "@/lib/utils";

type SSEDebugEvent = {
  id: string;
  eventType: string;
  receivedAt: string;
  raw: string;
  parsed: unknown | null;
  parseError?: string;
  lastEventId?: string;
};

type SessionDebugSnapshot = {
  session: Record<string, unknown>;
  tasks: Record<string, unknown>;
};

const EVENT_TYPES: Array<WorkflowEventType | "connected"> = [
  "connected",
  "workflow.lifecycle.updated",
  "workflow.node.started",
  "workflow.node.completed",
  "workflow.node.failed",
  "workflow.node.output.delta",
  "workflow.node.output.summary",
  "workflow.tool.started",
  "workflow.tool.progress",
  "workflow.tool.completed",
  "workflow.input.received",
  "workflow.subflow.progress",
  "workflow.subflow.completed",
  "workflow.result.final",
  "workflow.result.cancelled",
  "workflow.diagnostic.error",
  "workflow.diagnostic.context_compression",
  "workflow.diagnostic.tool_filtering",
  "workflow.diagnostic.environment_snapshot",
  "workflow.diagnostic.context_snapshot",
];

const DEFAULT_MAX_EVENTS = 2000;
const SESSION_REFRESH_MS = 3000;

function formatTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString();
}

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

function extractLogId(parsed: unknown): string | null {
  if (!parsed || typeof parsed !== "object") {
    return null;
  }
  const value = (parsed as Record<string, unknown>).log_id;
  if (typeof value === "string" && value.trim().length > 0) {
    return value.trim();
  }
  return null;
}

function buildPreview(entry: SSEDebugEvent) {
  if (entry.parsed && typeof entry.parsed === "object") {
    const payload = entry.parsed as Record<string, unknown>;
    if (typeof payload.event_type === "string") {
      return payload.event_type;
    }
    if (typeof payload.message === "string") {
      return payload.message;
    }
    if (typeof payload.error === "string") {
      return payload.error;
    }
  }
  return entry.raw.slice(0, 160);
}

function renderScalar(value: unknown) {
  if (value === null) {
    return <span className="text-rose-600">null</span>;
  }
  if (typeof value === "string") {
    return <span className="text-emerald-700 break-all">&quot;{value}&quot;</span>;
  }
  if (typeof value === "number") {
    return <span className="text-sky-700">{value}</span>;
  }
  if (typeof value === "boolean") {
    return <span className="text-amber-700">{value ? "true" : "false"}</span>;
  }
  if (typeof value === "undefined") {
    return <span className="text-muted-foreground">undefined</span>;
  }
  return <span className="text-muted-foreground break-all">{String(value)}</span>;
}

function JsonNode({
  label,
  value,
  depth = 0,
}: {
  label?: string;
  value: unknown;
  depth?: number;
}) {
  const isArray = Array.isArray(value);
  const isObject = !isArray && value !== null && typeof value === "object";

  if (!isArray && !isObject) {
    return (
      <div className="flex items-start gap-2 text-xs font-mono">
        {label && <span className="text-muted-foreground">{label}:</span>}
        {renderScalar(value)}
      </div>
    );
  }

  const entries = isArray
    ? (value as unknown[]).map((item, index) => [String(index), item] as const)
    : Object.entries(value as Record<string, unknown>);
  const summary = isArray ? `[${entries.length}]` : `{${entries.length}}`;

  return (
    <details open={depth < 1} className="rounded-md border border-dashed border-border/70 bg-muted/30 px-2 py-1">
      <summary className="flex cursor-pointer items-center gap-2 text-xs font-semibold text-foreground/80">
        {label && <span>{label}</span>}
        <span className="text-muted-foreground">{summary}</span>
      </summary>
      <div className="mt-2 space-y-1 border-l border-dashed border-border/60 pl-3">
        {entries.map(([key, entry]) => (
          <JsonNode key={`${label ?? "node"}-${key}`} label={key} value={entry} depth={depth + 1} />
        ))}
      </div>
    </details>
  );
}

function JsonViewer({ data }: { data: unknown }) {
  if (data === null || data === undefined) {
    return <p className="text-xs text-muted-foreground">No JSON payload available.</p>;
  }
  return (
    <div className="space-y-2">
      <JsonNode label="payload" value={data} />
    </div>
  );
}

function LogSnippetView({ snippet }: { snippet?: LogTraceBundle["service"] | null }) {
  if (!snippet) {
    return <p className="text-xs text-muted-foreground">No log data loaded.</p>;
  }
  if (snippet.error) {
    return (
      <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700">
        {snippet.error === "not_found" ? "Log file not found." : snippet.error}
      </div>
    );
  }
  const entries = snippet.entries ?? [];
  if (entries.length === 0) {
    return <p className="text-xs text-muted-foreground">No matching log entries.</p>;
  }
  return (
    <div className="space-y-2">
      {entries.map((entry, idx) => (
        <div
          key={`${snippet.path ?? "log"}-${idx}`}
          className="rounded-lg border border-border/60 bg-muted/20 p-2"
        >
          <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
            {entry}
          </pre>
        </div>
      ))}
      {snippet.truncated && (
        <Badge variant="warning" className="text-[10px]">
          Truncated
        </Badge>
      )}
    </div>
  );
}

type TimingEntry = {
  id: string;
  label: string;
  durationMs: number;
  hint?: string;
};

function formatDuration(ms: number) {
  if (!Number.isFinite(ms)) {
    return "—";
  }
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`;
  }
  return `${Math.round(ms)}ms`;
}

function TimingList({ entries, emptyLabel }: { entries: TimingEntry[]; emptyLabel: string }) {
  if (entries.length === 0) {
    return <p className="text-xs text-muted-foreground">{emptyLabel}</p>;
  }
  return (
    <div className="space-y-2">
      {entries.map((entry) => (
        <div
          key={entry.id}
          className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-xs"
        >
          <div className="min-w-0">
            <p className="truncate font-medium text-foreground">{entry.label}</p>
            {entry.hint && (
              <p className="truncate text-[11px] text-muted-foreground">{entry.hint}</p>
            )}
          </div>
          <span className="whitespace-nowrap font-semibold text-foreground/80">
            {formatDuration(entry.durationMs)}
          </span>
        </div>
      ))}
    </div>
  );
}

export default function ConversationDebugPage() {
  const [sessionIdInput, setSessionIdInput] = useState("");
  const [replayMode, setReplayMode] = useState<SSEReplayMode>("full");
  const [events, setEvents] = useState<SSEDebugEvent[]>([]);
  const [maxEvents, setMaxEvents] = useState(DEFAULT_MAX_EVENTS);
  const [maxEventsInput, setMaxEventsInput] = useState(String(DEFAULT_MAX_EVENTS));
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionSnapshot, setSessionSnapshot] = useState<SessionDebugSnapshot | null>(null);
  const [sessionSnapshotError, setSessionSnapshotError] = useState<string | null>(null);
  const [sessionSnapshotLoading, setSessionSnapshotLoading] = useState(false);
  const [sessionSnapshotUpdatedAt, setSessionSnapshotUpdatedAt] = useState<string | null>(null);
  const [sessionAutoRefresh, setSessionAutoRefresh] = useState(true);
  const [turnSnapshotMeta, setTurnSnapshotMeta] = useState<
    SessionSnapshotsResponse["items"][number] | null
  >(null);
  const [turnSnapshot, setTurnSnapshot] = useState<Record<string, unknown> | null>(null);
  const [turnSnapshotError, setTurnSnapshotError] = useState<string | null>(null);
  const [turnSnapshotLoading, setTurnSnapshotLoading] = useState(false);
  const [turnSnapshotUpdatedAt, setTurnSnapshotUpdatedAt] = useState<string | null>(null);
  const [logIdInput, setLogIdInput] = useState("");
  const [logTrace, setLogTrace] = useState<LogTraceBundle | null>(null);
  const [logTraceError, setLogTraceError] = useState<string | null>(null);
  const [logTraceLoading, setLogTraceLoading] = useState(false);

  const { currentSessionId, sessionHistory } = useSessionStore();

  const eventSourceRef = useRef<EventSource | null>(null);
  const listEndRef = useRef<HTMLDivElement | null>(null);
  const idRef = useRef(0);
  const autoSeededRef = useRef(false);

  const sessionId = sessionIdInput.trim();
  const logId = logIdInput.trim();

  const handleMaxEventsChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.target.value;
      setMaxEventsInput(value);
      const parsed = Number(value);
      if (Number.isFinite(parsed) && parsed >= 0) {
        setMaxEvents(Math.floor(parsed));
      }
    },
    [],
  );

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
      const nextId = `sse-${Date.now()}-${idRef.current += 1}`;
      const entry: SSEDebugEvent = {
        id: nextId,
        eventType,
        receivedAt: new Date().toISOString(),
        raw: rawData,
        parsed: parseError ? null : parsed,
        parseError,
        lastEventId: rawEvent.lastEventId || undefined,
      };

      setEvents((prev) => {
        const next = [...prev, entry];
        if (maxEvents > 0 && next.length > maxEvents) {
          next.splice(0, next.length - maxEvents);
        }
        return next;
      });

      setSelectedId((current) => current ?? nextId);
    },
    [maxEvents],
  );

  const connect = useCallback(() => {
    if (!sessionId) {
      setError("Session ID is required.");
      return;
    }

    disconnect();
    setError(null);
    setIsConnected(false);
    setIsConnecting(true);

    const source = apiClient.createSSEConnection(sessionId, undefined, {
      replay: replayMode,
      debug: true,
    });

    source.onopen = () => {
      setIsConnecting(false);
      setIsConnected(true);
    };

    source.onerror = () => {
      setIsConnecting(false);
      setIsConnected(false);
      setError("SSE connection error.");
    };

    EVENT_TYPES.forEach((type) => {
      source.addEventListener(type, (event) => handleIncomingEvent(type, event as MessageEvent));
    });

    source.onmessage = (event) => handleIncomingEvent("message", event);
    eventSourceRef.current = source;
  }, [disconnect, handleIncomingEvent, replayMode, sessionId]);

  const loadSessionSnapshot = useCallback(
    async (options?: { silent?: boolean }) => {
      if (!sessionId) {
        setSessionSnapshot(null);
        setSessionSnapshotError(null);
        setSessionSnapshotUpdatedAt(null);
        setSessionSnapshotLoading(false);
        return;
      }

      if (!options?.silent) {
        setSessionSnapshotLoading(true);
      }
      setSessionSnapshotError(null);

      try {
        const snapshot = await apiClient.getSessionRaw(sessionId);
        setSessionSnapshot(snapshot);
        setSessionSnapshotUpdatedAt(new Date().toISOString());
      } catch (err) {
        setSessionSnapshotError(
          err instanceof Error ? err.message : "Failed to load session data.",
        );
      } finally {
        if (!options?.silent) {
          setSessionSnapshotLoading(false);
        }
      }
    },
    [sessionId],
  );

  const loadTurnSnapshot = useCallback(
    async (options?: { silent?: boolean }) => {
      if (!sessionId) {
        setTurnSnapshotMeta(null);
        setTurnSnapshot(null);
        setTurnSnapshotError(null);
        setTurnSnapshotUpdatedAt(null);
        setTurnSnapshotLoading(false);
        return;
      }

      if (!options?.silent) {
        setTurnSnapshotLoading(true);
      }
      setTurnSnapshotError(null);

      try {
        const snapshots = await apiClient.listSessionSnapshots(sessionId, 1);
        if (!snapshots.items || snapshots.items.length === 0) {
          setTurnSnapshotMeta(null);
          setTurnSnapshot(null);
          setTurnSnapshotUpdatedAt(new Date().toISOString());
          return;
        }

        const latest = snapshots.items[0];
        setTurnSnapshotMeta(latest);
        const snapshot = await apiClient.getSessionTurnSnapshot(sessionId, latest.turn_id);
        setTurnSnapshot(snapshot);
        setTurnSnapshotUpdatedAt(new Date().toISOString());
      } catch (err) {
        setTurnSnapshotError(
          err instanceof Error ? err.message : "Failed to load turn snapshot.",
        );
      } finally {
        if (!options?.silent) {
          setTurnSnapshotLoading(false);
        }
      }
    },
    [sessionId],
  );

  const loadLogTrace = useCallback(
    async (options?: { silent?: boolean }) => {
      if (!logId) {
        setLogTrace(null);
        setLogTraceError(null);
        setLogTraceLoading(false);
        return;
      }

      if (!options?.silent) {
        setLogTraceLoading(true);
      }
      setLogTraceError(null);

      try {
        const trace = await apiClient.getLogTrace(logId);
        setLogTrace(trace);
      } catch (err) {
        setLogTraceError(
          err instanceof Error ? err.message : "Failed to load log trace.",
        );
      } finally {
        if (!options?.silent) {
          setLogTraceLoading(false);
        }
      }
    },
    [logId],
  );

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  useEffect(() => {
    if (autoSeededRef.current) {
      return;
    }
    if (sessionIdInput.trim()) {
      autoSeededRef.current = true;
      return;
    }
    const candidate = currentSessionId ?? sessionHistory[0];
    if (!candidate) {
      return;
    }
    setSessionIdInput(candidate);
    autoSeededRef.current = true;
  }, [currentSessionId, sessionHistory, sessionIdInput]);

  useEffect(() => {
    if (!autoScroll || events.length === 0) {
      return;
    }
    listEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [autoScroll, events]);

  useEffect(() => {
    if (maxEvents <= 0) {
      return;
    }
    setEvents((prev) => (prev.length > maxEvents ? prev.slice(-maxEvents) : prev));
  }, [maxEvents]);

  useEffect(() => {
    if (selectedId && !events.some((entry) => entry.id === selectedId)) {
      setSelectedId(events.length > 0 ? events[events.length - 1].id : null);
    }
  }, [events, selectedId]);

  useEffect(() => {
    if (!sessionId) {
      setSessionSnapshot(null);
      setSessionSnapshotError(null);
      setSessionSnapshotUpdatedAt(null);
      setSessionSnapshotLoading(false);
      setTurnSnapshotMeta(null);
      setTurnSnapshot(null);
      setTurnSnapshotError(null);
      setTurnSnapshotUpdatedAt(null);
      setTurnSnapshotLoading(false);
      return;
    }
    void loadSessionSnapshot();
    void loadTurnSnapshot();
  }, [loadSessionSnapshot, loadTurnSnapshot, sessionId]);

  useEffect(() => {
    if (!logId) {
      setLogTrace(null);
      setLogTraceError(null);
      setLogTraceLoading(false);
    }
  }, [logId]);

  useEffect(() => {
    if (!sessionId || !sessionAutoRefresh) {
      return;
    }
    const handle = setInterval(() => {
      void loadSessionSnapshot({ silent: true });
      void loadTurnSnapshot({ silent: true });
    }, SESSION_REFRESH_MS);
    return () => clearInterval(handle);
  }, [loadSessionSnapshot, loadTurnSnapshot, sessionAutoRefresh, sessionId]);

  const filteredEvents = useMemo(() => {
    if (!filter.trim()) {
      return events;
    }
    const needle = filter.trim().toLowerCase();
    return events.filter(
      (entry) =>
        entry.eventType.toLowerCase().includes(needle) ||
        entry.raw.toLowerCase().includes(needle),
    );
  }, [events, filter]);

  const selectedEvent = useMemo(
    () => events.find((entry) => entry.id === selectedId) ?? null,
    [events, selectedId],
  );
  const selectedLogId = useMemo(() => {
    if (!selectedEvent) {
      return null;
    }
    return extractLogId(selectedEvent.parsed);
  }, [selectedEvent]);
  const turnSnapshotMessagesCount = useMemo(() => {
    if (!turnSnapshot || typeof turnSnapshot !== "object") {
      return null;
    }
    const messages = (turnSnapshot as Record<string, unknown>).messages;
    if (!Array.isArray(messages)) {
      return null;
    }
    return messages.length;
  }, [turnSnapshot]);
  const timingSummary = useMemo(() => {
    const stages: TimingEntry[] = [];
    const llmCalls: TimingEntry[] = [];
    const tools: TimingEntry[] = [];
    let totalDurationMs: number | null = null;

    for (const entry of events) {
      if (!entry.parsed || typeof entry.parsed !== "object") {
        continue;
      }
      const payload = (entry.parsed as Record<string, unknown>).payload;
      if (!payload || typeof payload !== "object") {
        continue;
      }
      const data = payload as Record<string, unknown>;

      if (
        entry.eventType === "workflow.node.completed" ||
        entry.eventType === "workflow.node.failed"
      ) {
        const duration = Number(data.duration_ms);
        if (Number.isFinite(duration) && duration > 0) {
          const label =
            (data.step_description as string | undefined) ||
            (data.node_id as string | undefined) ||
            "stage";
          stages.push({
            id: entry.id,
            label,
            durationMs: duration,
          });
        }
      }

      if (entry.eventType === "workflow.node.output.summary") {
        const duration = Number(data.llm_duration_ms);
        if (Number.isFinite(duration) && duration > 0) {
          const iteration = data.iteration ? `iter ${data.iteration}` : "llm";
          const model = typeof data.llm_model === "string" ? data.llm_model : "";
          const requestId =
            typeof data.llm_request_id === "string" ? data.llm_request_id : "";
          const hintParts = [model && `model=${model}`, requestId && `req=${requestId}`]
            .filter(Boolean)
            .join(" · ");
          llmCalls.push({
            id: entry.id,
            label: iteration,
            durationMs: duration,
            hint: hintParts || undefined,
          });
        }
      }

      if (entry.eventType === "workflow.tool.completed") {
        const duration = Number(data.duration);
        if (Number.isFinite(duration) && duration > 0) {
          const toolName =
            (data.tool_name as string | undefined) ||
            (data.tool as string | undefined) ||
            "tool";
          const meta = data.metadata as Record<string, unknown> | undefined;
          const llmDuration = meta ? Number(meta.llm_duration_ms) : NaN;
          const llmRequest =
            meta && typeof meta.llm_request_id === "string"
              ? meta.llm_request_id
              : "";
          const hintParts = [
            Number.isFinite(llmDuration) && llmDuration > 0
              ? `llm=${formatDuration(llmDuration)}`
              : "",
            llmRequest ? `req=${llmRequest}` : "",
          ]
            .filter(Boolean)
            .join(" · ");
          tools.push({
            id: entry.id,
            label: toolName,
            durationMs: duration,
            hint: hintParts || undefined,
          });
        }
      }

      if (entry.eventType === "workflow.result.final" && totalDurationMs === null) {
        const duration = Number(data.duration);
        if (Number.isFinite(duration) && duration > 0) {
          totalDurationMs = duration;
        }
      }
    }

    const byDuration = (a: TimingEntry, b: TimingEntry) => b.durationMs - a.durationMs;
    return {
      stages: stages.sort(byDuration),
      llmCalls: llmCalls.sort(byDuration),
      tools: tools.sort(byDuration),
      totalDurationMs,
    };
  }, [events]);

  const statusBadge = useMemo(() => {
    if (error) {
      return <Badge variant="destructive">Error</Badge>;
    }
    if (isConnecting) {
      return <Badge variant="warning">Connecting</Badge>;
    }
    if (isConnected) {
      return <Badge variant="success">Connected</Badge>;
    }
    return <Badge variant="outline">Disconnected</Badge>;
  }, [error, isConnected, isConnecting]);

  const sessionStatusBadge = useMemo(() => {
    if (sessionSnapshotError) {
      return <Badge variant="destructive">Session error</Badge>;
    }
    if (!sessionId) {
      return <Badge variant="outline">No session</Badge>;
    }
    if (sessionSnapshotLoading) {
      return <Badge variant="warning">Loading</Badge>;
    }
    if (sessionSnapshotUpdatedAt) {
      return <Badge variant="success">Fresh</Badge>;
    }
    return <Badge variant="outline">Idle</Badge>;
  }, [sessionId, sessionSnapshotError, sessionSnapshotLoading, sessionSnapshotUpdatedAt]);

  const turnStatusBadge = useMemo(() => {
    if (turnSnapshotError) {
      return <Badge variant="destructive">Turn error</Badge>;
    }
    if (!sessionId) {
      return <Badge variant="outline">No session</Badge>;
    }
    if (turnSnapshotLoading) {
      return <Badge variant="warning">Loading</Badge>;
    }
    if (turnSnapshotUpdatedAt) {
      return <Badge variant="success">Fresh</Badge>;
    }
    return <Badge variant="outline">Idle</Badge>;
  }, [sessionId, turnSnapshotError, turnSnapshotLoading, turnSnapshotUpdatedAt]);

  return (
    <RequireAuth>
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-6xl flex-col gap-6">
          <header className="rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div className="space-y-2">
                <p className="text-[11px] font-semibold text-slate-400">
                  Dev Tools · Conversation SSE Debugger
                </p>
                <h1 className="text-xl font-semibold text-slate-900 lg:text-2xl">
                  Inspect raw SSE payloads
                </h1>
                <p className="text-sm text-slate-600">
                  Connect to a session to inspect exactly what the frontend receives from the SSE stream.
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
                {statusBadge}
                {sessionId && (
                  <Badge variant="outline">Session: {sessionId}</Badge>
                )}
                <Badge variant="outline">Replay: {replayMode}</Badge>
                <Badge variant="outline">Events: {events.length}</Badge>
                <Badge variant="outline">
                  Cap: {maxEvents > 0 ? maxEvents : "∞"}
                </Badge>
              </div>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_140px_auto] md:items-end">
              <div className="space-y-1">
                <p className="text-xs font-semibold text-muted-foreground">Session ID</p>
                <Input
                  value={sessionIdInput}
                  onChange={(event) => setSessionIdInput(event.target.value)}
                  placeholder="session-xxxxxxxx"
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      connect();
                    }
                  }}
                />
              </div>
              <div className="space-y-1">
                <p className="text-xs font-semibold text-muted-foreground">Replay</p>
                <select
                  value={replayMode}
                  onChange={(event) => setReplayMode(event.target.value as SSEReplayMode)}
                  className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm shadow-sm"
                >
                  <option value="full">full</option>
                  <option value="session">session</option>
                  <option value="none">none</option>
                </select>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  onClick={connect}
                  disabled={!sessionId || isConnecting}
                >
                  <Play className="mr-2 h-4 w-4" />
                  Connect
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={disconnect}
                  disabled={!isConnected && !isConnecting}
                >
                  <Square className="mr-2 h-4 w-4" />
                  Disconnect
                </Button>
                <Button size="sm" variant="outline" onClick={clearEvents}>
                  <Trash2 className="mr-2 h-4 w-4" />
                  Clear
                </Button>
                <Button
                  size="sm"
                  variant={autoScroll ? "default" : "outline"}
                  onClick={() => setAutoScroll((value) => !value)}
                >
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Auto-scroll {autoScroll ? "on" : "off"}
                </Button>
              </div>
            </div>

            {error && (
              <div className="mt-3 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                {error}
              </div>
            )}
          </header>

          <div className="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="text-base">Event stream</CardTitle>
                <CardDescription>
                  Filter and select a payload to inspect the parsed JSON and raw data.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Input
                    value={filter}
                    onChange={(event) => setFilter(event.target.value)}
                    placeholder="Filter by type or text"
                  />
                  <div className="min-w-[140px]">
                    <Input
                      type="number"
                      min="0"
                      value={maxEventsInput}
                      onChange={handleMaxEventsChange}
                      placeholder="Max events"
                    />
                  </div>
                  <Badge variant="outline">
                    {filteredEvents.length}/{events.length}
                  </Badge>
                </div>
                <p className="text-[11px] text-muted-foreground">
                  Event buffer size. Use 0 for unlimited.
                </p>
                <ScrollArea className="h-[520px] pr-3">
                  <div className="space-y-2">
                    {filteredEvents.map((entry) => {
                      const preview = buildPreview(entry);
                      const isSelected = entry.id === selectedId;
                      return (
                        <button
                          key={entry.id}
                          type="button"
                          onClick={() => setSelectedId(entry.id)}
                          className={cn(
                            "w-full rounded-xl border px-3 py-2 text-left transition",
                            isSelected
                              ? "border-primary/60 bg-primary/5"
                              : "border-border/60 hover:bg-muted/40",
                          )}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-xs font-semibold text-foreground">
                              {entry.eventType}
                            </span>
                            <span className="text-[11px] text-muted-foreground">
                              {formatTimestamp(entry.receivedAt)}
                            </span>
                          </div>
                          <div className="mt-1 flex items-center gap-2">
                            <p className="line-clamp-2 text-[11px] text-muted-foreground">
                              {preview || "—"}
                            </p>
                            {entry.parseError && (
                              <Badge variant="warning" className="text-[10px]">
                                Invalid JSON
                              </Badge>
                            )}
                          </div>
                        </button>
                      );
                    })}
                    {filteredEvents.length === 0 && (
                      <div className="rounded-xl border border-dashed border-border/60 px-3 py-6 text-center text-xs text-muted-foreground">
                        No events yet. Connect to a session to start capturing SSE payloads.
                      </div>
                    )}
                    <div ref={listEndRef} />
                  </div>
                </ScrollArea>
              </CardContent>
            </Card>

            <div className="space-y-4">
              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Payload inspector</CardTitle>
                  <CardDescription>
                    Parsed JSON and raw payload for the selected SSE event.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {selectedEvent ? (
                    <div className="space-y-4">
                      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <Badge variant="secondary">{selectedEvent.eventType}</Badge>
                        {selectedEvent.lastEventId && (
                          <Badge variant="outline">ID: {selectedEvent.lastEventId}</Badge>
                        )}
                        <Badge variant="outline">
                          Received: {formatTimestamp(selectedEvent.receivedAt)}
                        </Badge>
                      </div>
                      {selectedEvent.parseError && (
                        <div className="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                          JSON parse error: {selectedEvent.parseError}
                        </div>
                      )}
                      <Tabs defaultValue="json">
                        <TabsList>
                          <TabsTrigger value="json">Parsed JSON</TabsTrigger>
                          <TabsTrigger value="raw">Raw payload</TabsTrigger>
                        </TabsList>
                        <TabsContent value="json">
                          <div className="rounded-xl border border-border/60 bg-muted/20 p-3">
                            <JsonViewer data={selectedEvent.parsed} />
                          </div>
                        </TabsContent>
                        <TabsContent value="raw">
                          <div className="rounded-xl border border-border/60 bg-muted/20 p-3">
                            <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
                              {selectedEvent.raw || "—"}
                            </pre>
                          </div>
                        </TabsContent>
                      </Tabs>
                    </div>
                  ) : (
                    <div className="rounded-xl border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">
                      Select an event to inspect its payload.
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Session snapshot</CardTitle>
                  <CardDescription>
                    Raw session and task list payloads pulled from the API for the same session ID.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    {sessionStatusBadge}
                    {sessionSnapshotUpdatedAt && (
                      <Badge variant="outline">
                        Updated: {formatTimestamp(sessionSnapshotUpdatedAt)}
                      </Badge>
                    )}
                    <Badge variant="outline">
                      Auto-refresh {sessionAutoRefresh ? "on" : "off"}
                    </Badge>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => loadSessionSnapshot()}
                      disabled={!sessionId || sessionSnapshotLoading}
                    >
                      <RefreshCw className="mr-2 h-4 w-4" />
                      Refresh
                    </Button>
                    <Button
                      size="sm"
                      variant={sessionAutoRefresh ? "default" : "outline"}
                      onClick={() => setSessionAutoRefresh((value) => !value)}
                      disabled={!sessionId}
                    >
                      Auto-refresh {sessionAutoRefresh ? "on" : "off"}
                    </Button>
                    {sessionSnapshotLoading && (
                      <Badge variant="warning">Loading</Badge>
                    )}
                  </div>
                  {sessionSnapshotError && (
                    <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                      {sessionSnapshotError}
                    </div>
                  )}
                  {sessionSnapshot ? (
                    <Tabs defaultValue="parsed">
                      <TabsList>
                        <TabsTrigger value="parsed">Parsed JSON</TabsTrigger>
                        <TabsTrigger value="raw">Raw JSON</TabsTrigger>
                      </TabsList>
                      <TabsContent value="parsed">
                        <div className="rounded-xl border border-border/60 bg-muted/20 p-3">
                          <JsonViewer data={sessionSnapshot} />
                        </div>
                      </TabsContent>
                      <TabsContent value="raw">
                        <div className="rounded-xl border border-border/60 bg-muted/20 p-3">
                          <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
                            {JSON.stringify(sessionSnapshot, null, 2)}
                          </pre>
                        </div>
                      </TabsContent>
                    </Tabs>
                  ) : (
                    <div className="rounded-xl border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">
                      Enter a session ID to load the server snapshot.
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Latest turn snapshot</CardTitle>
                  <CardDescription>
                    Snapshot data captured per LLM turn, including stored messages.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    {turnStatusBadge}
                    {turnSnapshotUpdatedAt && (
                      <Badge variant="outline">
                        Updated: {formatTimestamp(turnSnapshotUpdatedAt)}
                      </Badge>
                    )}
                    {turnSnapshotMeta && (
                      <Badge variant="outline">Turn: {turnSnapshotMeta.turn_id}</Badge>
                    )}
                    {turnSnapshotMeta && (
                      <Badge variant="outline">Seq: {turnSnapshotMeta.llm_turn_seq}</Badge>
                    )}
                    {turnSnapshotMeta?.created_at && (
                      <Badge variant="outline">
                        Created: {formatTimestamp(turnSnapshotMeta.created_at)}
                      </Badge>
                    )}
                    <Badge variant="outline">
                      Auto-refresh {sessionAutoRefresh ? "on" : "off"}
                    </Badge>
                    {turnSnapshotMessagesCount !== null && (
                      <Badge variant="outline">
                        Messages: {turnSnapshotMessagesCount}
                      </Badge>
                    )}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => loadTurnSnapshot()}
                      disabled={!sessionId || turnSnapshotLoading}
                    >
                      <RefreshCw className="mr-2 h-4 w-4" />
                      Refresh
                    </Button>
                    {turnSnapshotLoading && <Badge variant="warning">Loading</Badge>}
                  </div>
                  {turnSnapshotError && (
                    <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                      {turnSnapshotError}
                    </div>
                  )}
                  {turnSnapshot ? (
                    <Tabs defaultValue="parsed">
                      <TabsList>
                        <TabsTrigger value="parsed">Parsed JSON</TabsTrigger>
                        <TabsTrigger value="raw">Raw JSON</TabsTrigger>
                      </TabsList>
                      <TabsContent value="parsed">
                        <div className="rounded-xl border border-border/60 bg-muted/20 p-3">
                          <JsonViewer data={turnSnapshot} />
                        </div>
                      </TabsContent>
                      <TabsContent value="raw">
                        <div className="rounded-xl border border-border/60 bg-muted/20 p-3">
                          <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
                            {JSON.stringify(turnSnapshot, null, 2)}
                          </pre>
                        </div>
                      </TabsContent>
                    </Tabs>
                  ) : (
                    <div className="rounded-xl border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">
                      {sessionId
                        ? "No turn snapshots found yet."
                        : "Enter a session ID to load the latest turn snapshot."}
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Timing breakdown</CardTitle>
                  <CardDescription>
                    Breakdown of workflow stages, tools, and LLM calls for the active session stream.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <Badge variant="outline">
                      Total:{" "}
                      {timingSummary.totalDurationMs !== null
                        ? formatDuration(timingSummary.totalDurationMs)
                        : "—"}
                    </Badge>
                    <Badge variant="outline">Stages: {timingSummary.stages.length}</Badge>
                    <Badge variant="outline">Tools: {timingSummary.tools.length}</Badge>
                    <Badge variant="outline">LLM calls: {timingSummary.llmCalls.length}</Badge>
                  </div>
                  <div className="grid gap-4 lg:grid-cols-2">
                    <div className="space-y-2">
                      <p className="text-xs font-semibold text-muted-foreground">Stages</p>
                      <TimingList
                        entries={timingSummary.stages}
                        emptyLabel="No stage timings yet."
                      />
                    </div>
                    <div className="space-y-2">
                      <p className="text-xs font-semibold text-muted-foreground">Tools</p>
                      <TimingList
                        entries={timingSummary.tools}
                        emptyLabel="No tool timings yet."
                      />
                    </div>
                  </div>
                  <div className="space-y-2">
                    <p className="text-xs font-semibold text-muted-foreground">LLM calls</p>
                    <TimingList
                      entries={timingSummary.llmCalls}
                      emptyLabel="No LLM timing data yet."
                    />
                  </div>
                </CardContent>
              </Card>

              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Log trace by Log ID</CardTitle>
                  <CardDescription>
                    Fetch server, LLM, latency, and raw request logs for a specific log_id.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-1">
                    <p className="text-xs font-semibold text-muted-foreground">Log ID</p>
                    <Input
                      value={logIdInput}
                      onChange={(event) => setLogIdInput(event.target.value)}
                      placeholder="log-xxxxxxxx"
                      onKeyDown={(event) => {
                        if (event.key === "Enter") {
                          void loadLogTrace();
                        }
                      }}
                    />
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      onClick={() => void loadLogTrace()}
                      disabled={!logId || logTraceLoading}
                    >
                      <Play className="mr-2 h-4 w-4" />
                      Fetch logs
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setLogIdInput("");
                        setLogTrace(null);
                        setLogTraceError(null);
                      }}
                    >
                      Clear
                    </Button>
                    {selectedLogId && selectedLogId !== logId && (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setLogIdInput(selectedLogId)}
                      >
                        Use selected log_id
                      </Button>
                    )}
                    {logTraceLoading && <Badge variant="warning">Loading</Badge>}
                  </div>
                  {logTraceError && (
                    <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                      {logTraceError}
                    </div>
                  )}
                  {logTrace ? (
                    <Tabs defaultValue="service">
                      <TabsList>
                        <TabsTrigger value="service">Service</TabsTrigger>
                        <TabsTrigger value="llm">LLM</TabsTrigger>
                        <TabsTrigger value="latency">Latency</TabsTrigger>
                        <TabsTrigger value="requests">Requests</TabsTrigger>
                      </TabsList>
                      <TabsContent value="service">
                        <LogSnippetView snippet={logTrace.service} />
                      </TabsContent>
                      <TabsContent value="llm">
                        <LogSnippetView snippet={logTrace.llm} />
                      </TabsContent>
                      <TabsContent value="latency">
                        <LogSnippetView snippet={logTrace.latency} />
                      </TabsContent>
                      <TabsContent value="requests">
                        <LogSnippetView snippet={logTrace.requests} />
                      </TabsContent>
                    </Tabs>
                  ) : (
                    <div className="rounded-xl border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">
                      Enter a log_id to fetch trace logs.
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </div>
        </div>
      </div>
    </RequireAuth>
  );
}
