"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Play, RefreshCw, Square, Trash2 } from "lucide-react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient, type SSEReplayMode } from "@/lib/api";
import { type WorkflowEventType } from "@/lib/types";
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

const MAX_EVENTS = 500;
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

export default function ConversationDebugPage() {
  const [sessionIdInput, setSessionIdInput] = useState("");
  const [replayMode, setReplayMode] = useState<SSEReplayMode>("full");
  const [events, setEvents] = useState<SSEDebugEvent[]>([]);
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

  const eventSourceRef = useRef<EventSource | null>(null);
  const listEndRef = useRef<HTMLDivElement | null>(null);
  const idRef = useRef(0);

  const sessionId = sessionIdInput.trim();

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
        if (next.length > MAX_EVENTS) {
          next.shift();
        }
        return next;
      });

      setSelectedId((current) => current ?? nextId);
    },
    [],
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

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  useEffect(() => {
    if (!autoScroll || events.length === 0) {
      return;
    }
    listEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [autoScroll, events]);

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
      return;
    }
    void loadSessionSnapshot();
  }, [loadSessionSnapshot, sessionId]);

  useEffect(() => {
    if (!sessionId || !sessionAutoRefresh) {
      return;
    }
    const handle = setInterval(() => {
      void loadSessionSnapshot({ silent: true });
    }, SESSION_REFRESH_MS);
    return () => clearInterval(handle);
  }, [loadSessionSnapshot, sessionAutoRefresh, sessionId]);

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
                <div className="flex items-center gap-2">
                  <Input
                    value={filter}
                    onChange={(event) => setFilter(event.target.value)}
                    placeholder="Filter by type or text"
                  />
                  <Badge variant="outline">
                    {filteredEvents.length}/{events.length}
                  </Badge>
                </div>
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
            </div>
          </div>
        </div>
      </div>
    </RequireAuth>
  );
}
