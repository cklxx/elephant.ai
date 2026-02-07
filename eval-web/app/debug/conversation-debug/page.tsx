"use client";

import { useEffect, useRef, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";

interface SSEEvent {
  id: string;
  type: string;
  data: string;
  timestamp: string;
}

const API_URL = process.env.NEXT_PUBLIC_EVAL_API_URL ?? "http://localhost:8081";

export default function ConversationDebugPage() {
  const [connected, setConnected] = useState(false);
  const [events, setEvents] = useState<SSEEvent[]>([]);
  const [filter, setFilter] = useState("");
  const sourceRef = useRef<EventSource | null>(null);
  const logRef = useRef<HTMLDivElement>(null);

  function connect() {
    if (sourceRef.current) {
      sourceRef.current.close();
    }
    const source = new EventSource(`${API_URL}/api/sse`);
    sourceRef.current = source;

    source.onopen = () => setConnected(true);
    source.onerror = () => {
      setConnected(false);
      source.close();
      sourceRef.current = null;
    };
    source.onmessage = (ev) => {
      const event: SSEEvent = {
        id: ev.lastEventId || String(Date.now()),
        type: ev.type,
        data: ev.data,
        timestamp: new Date().toISOString(),
      };
      setEvents((prev) => [...prev.slice(-500), event]);
    };
  }

  function disconnect() {
    sourceRef.current?.close();
    sourceRef.current = null;
    setConnected(false);
  }

  useEffect(() => {
    return () => sourceRef.current?.close();
  }, []);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [events]);

  const filtered = filter
    ? events.filter(
        (e) =>
          e.type.includes(filter) ||
          e.data.includes(filter) ||
          e.id.includes(filter),
      )
    : events;

  return (
    <PageShell
      title="Conversation Debug"
      description="SSE event inspector and session snapshot viewer."
    >
      <div className="flex items-center gap-3">
        <button
          onClick={connected ? disconnect : connect}
          className={`rounded-md border px-3 py-1.5 text-xs font-medium transition-colors ${
            connected
              ? "border-destructive bg-destructive/10 text-destructive"
              : "border-primary bg-primary text-primary-foreground"
          }`}
        >
          {connected ? "Disconnect" : "Connect SSE"}
        </button>
        <span
          className={`inline-block h-2 w-2 rounded-full ${
            connected ? "bg-green-500" : "bg-gray-300"
          }`}
        />
        <span className="text-xs text-muted-foreground">
          {connected ? "Connected" : "Disconnected"} — {events.length} events
        </span>
        {events.length > 0 && (
          <button
            onClick={() => setEvents([])}
            className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-accent"
          >
            Clear
          </button>
        )}
      </div>

      <div className="mt-4">
        <input
          type="text"
          placeholder="Filter events by type or content..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>

      <div
        ref={logRef}
        className="mt-4 h-[500px] overflow-y-auto rounded-lg border border-border bg-card font-mono text-xs"
      >
        {filtered.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <p className="text-muted-foreground">
              {connected
                ? "Waiting for events..."
                : "Click Connect to start receiving SSE events"}
            </p>
          </div>
        ) : (
          <div className="divide-y divide-border">
            {filtered.map((ev, i) => (
              <div key={`${ev.id}-${i}`} className="p-2 hover:bg-muted/30">
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">
                    {new Date(ev.timestamp).toLocaleTimeString()}
                  </span>
                  <span className="rounded bg-muted px-1 py-0.5 text-primary">
                    {ev.type}
                  </span>
                  <span className="text-muted-foreground">id={ev.id}</span>
                </div>
                <pre className="mt-1 whitespace-pre-wrap break-all text-foreground">
                  {ev.data.length > 500
                    ? ev.data.slice(0, 500) + "..."
                    : ev.data}
                </pre>
              </div>
            ))}
          </div>
        )}
      </div>
    </PageShell>
  );
}
