"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient } from "@/lib/api";
import type {
  LogIndexEntry,
  ParsedTextLogEntry,
  ParsedRequestLogEntry,
  StructuredLogBundle,
  StructuredLogSnippet,
  StructuredRequestSnippet,
} from "@/lib/types";

// ─── Helpers ────────────────────────────────────────────────────────────────

function formatLastSeen(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatLoadError(err: unknown, fallback: string): string {
  const message = err instanceof Error ? err.message : fallback;
  const lowered = message.toLowerCase();
  if (lowered.includes("404")) {
    return "API unavailable (backend may be stale or not in development mode).";
  }
  if (lowered.includes("401") || lowered.includes("unauthorized")) {
    return "Authentication required. Please sign in and refresh.";
  }
  return message || fallback;
}

function snippetEntryCount(snippet?: StructuredLogSnippet | StructuredRequestSnippet): number {
  return snippet?.entries?.length ?? 0;
}

function formatTimestamp(ts: string): string {
  if (!ts) return "";
  const spaceIdx = ts.indexOf(" ");
  return spaceIdx > 0 ? ts.slice(spaceIdx + 1) : ts;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ─── Search Highlight ───────────────────────────────────────────────────────

function HighlightText({ text, search }: { text: string; search: string }) {
  if (!search || !text) return <>{text}</>;
  const lowerText = text.toLowerCase();
  const lowerSearch = search.toLowerCase();
  const parts: { text: string; highlight: boolean }[] = [];
  let cursor = 0;

  while (cursor < text.length) {
    const idx = lowerText.indexOf(lowerSearch, cursor);
    if (idx === -1) {
      parts.push({ text: text.slice(cursor), highlight: false });
      break;
    }
    if (idx > cursor) {
      parts.push({ text: text.slice(cursor, idx), highlight: false });
    }
    parts.push({ text: text.slice(idx, idx + search.length), highlight: true });
    cursor = idx + search.length;
  }

  return (
    <>
      {parts.map((part, i) =>
        part.highlight ? (
          <mark key={i} className="rounded-sm bg-yellow-200 px-0.5">
            {part.text}
          </mark>
        ) : (
          <span key={i}>{part.text}</span>
        ),
      )}
    </>
  );
}

// ─── Log Level Badge ────────────────────────────────────────────────────────

function LogLevelBadge({ level }: { level: string }) {
  const variant = {
    DEBUG: "secondary" as const,
    INFO: "info" as const,
    WARN: "warning" as const,
    ERROR: "destructive" as const,
  }[level.toUpperCase()] ?? ("outline" as const);

  return (
    <Badge variant={variant} className="font-mono text-[10px] px-1.5 py-0">
      {level || "???"}
    </Badge>
  );
}

// ─── Text Log Table ─────────────────────────────────────────────────────────

function TextLogTable({
  entries,
  search,
}: {
  entries: ParsedTextLogEntry[];
  search: string;
}) {
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());

  const toggleRow = (idx: number) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  };

  if (entries.length === 0) {
    return <p className="py-8 text-center text-sm text-slate-500">No log entries found.</p>;
  }

  return (
    <div className="overflow-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-slate-200 text-left text-slate-500">
            <th className="px-2 py-1.5 font-medium w-[72px]">Time</th>
            <th className="px-2 py-1.5 font-medium w-[60px]">Level</th>
            <th className="px-2 py-1.5 font-medium w-[80px]">Component</th>
            <th className="px-2 py-1.5 font-medium w-[90px]">Source</th>
            <th className="px-2 py-1.5 font-medium">Message</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, idx) => {
            const expanded = expandedRows.has(idx);
            const isLong = entry.message.length > 200;
            return (
              <tr
                key={idx}
                className="border-b border-slate-100 hover:bg-slate-50 transition-colors"
              >
                <td className="px-2 py-1 font-mono text-slate-500 whitespace-nowrap align-top">
                  {formatTimestamp(entry.timestamp)}
                </td>
                <td className="px-2 py-1 align-top">
                  <LogLevelBadge level={entry.level} />
                </td>
                <td className="px-2 py-1 text-slate-600 align-top">{entry.component}</td>
                <td className="px-2 py-1 font-mono text-slate-400 align-top whitespace-nowrap">
                  {entry.source_file ? `${entry.source_file}:${entry.source_line}` : ""}
                </td>
                <td className="px-2 py-1 align-top">
                  <div
                    className={`${!expanded && isLong ? "line-clamp-2" : ""} break-all`}
                  >
                    <HighlightText text={entry.message} search={search} />
                  </div>
                  {isLong && (
                    <button
                      type="button"
                      onClick={() => toggleRow(idx)}
                      className="mt-0.5 text-sky-600 hover:text-sky-800 text-[10px]"
                    >
                      {expanded ? "Collapse" : "Expand"}
                    </button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ─── JSON Payload Viewer ────────────────────────────────────────────────────

function JsonPayloadViewer({
  payload,
  search,
}: {
  payload: unknown;
  search: string;
}) {
  const [expanded, setExpanded] = useState(false);

  if (payload === undefined || payload === null) {
    return <span className="text-slate-400 text-xs italic">no payload</span>;
  }

  const formatted = JSON.stringify(payload, null, 2);
  const isLarge = formatted.length > 50 * 1024;
  const lines = formatted.split("\n");
  const previewLines = 3;

  const displayText = expanded ? formatted : lines.slice(0, previewLines).join("\n");

  return (
    <div className="mt-1">
      {isLarge && !expanded && (
        <p className="text-[10px] text-amber-600 mb-1">
          Large payload ({formatBytes(formatted.length)}) — click to expand
        </p>
      )}
      <pre className="rounded-md bg-slate-950 p-2 text-[11px] text-slate-100 overflow-auto max-h-96">
        <HighlightText text={displayText} search={search} />
        {!expanded && lines.length > previewLines && (
          <span className="text-slate-500">{"\n... "}{lines.length - previewLines} more lines</span>
        )}
      </pre>
      {lines.length > previewLines && (
        <button
          type="button"
          onClick={() => setExpanded(!expanded)}
          className="mt-1 text-sky-600 hover:text-sky-800 text-[10px]"
        >
          {expanded ? "Collapse" : `Expand (${lines.length} lines)`}
        </button>
      )}
    </div>
  );
}

// ─── Request Log List ───────────────────────────────────────────────────────

function RequestLogList({
  entries,
  search,
}: {
  entries: ParsedRequestLogEntry[];
  search: string;
}) {
  if (entries.length === 0) {
    return <p className="py-8 text-center text-sm text-slate-500">No LLM request/response entries found.</p>;
  }

  return (
    <div className="space-y-3">
      {entries.map((entry, idx) => (
        <div
          key={idx}
          className="rounded-lg border border-slate-200 bg-white p-3 space-y-1"
        >
          <div className="flex items-center gap-2 text-xs">
            <Badge
              variant={entry.entry_type === "request" ? "info" : "success"}
              className="font-mono text-[10px] px-1.5 py-0"
            >
              {entry.entry_type}
            </Badge>
            <span className="font-mono text-slate-500">{entry.request_id}</span>
            <span className="text-slate-400">{formatTimestamp(entry.timestamp)}</span>
            <span className="text-slate-400">{formatBytes(entry.body_bytes)}</span>
          </div>
          <JsonPayloadViewer payload={entry.payload} search={search} />
        </div>
      ))}
    </div>
  );
}

// ─── Log ID Sidebar ─────────────────────────────────────────────────────────

function LogIDSidebar({
  entries,
  selectedLogID,
  onSelect,
  loading,
}: {
  entries: LogIndexEntry[];
  selectedLogID: string;
  onSelect: (logID: string) => void;
  loading: boolean;
}) {
  const [filter, setFilter] = useState("");

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter(
      (e) =>
        e.log_id.toLowerCase().includes(q) ||
        (e.sources ?? []).some((s) => s.toLowerCase().includes(q)),
    );
  }, [entries, filter]);

  return (
    <div className="flex h-full flex-col">
      <div className="px-3 pb-2">
        <Input
          className="h-8 text-xs"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter log IDs..."
        />
      </div>
      <div className="px-3 pb-1 text-[10px] text-slate-400">
        {filtered.length} / {entries.length} IDs
      </div>
      <ScrollArea className="flex-1 px-2">
        <div className="space-y-1 pb-2">
          {loading && entries.length === 0 && (
            <p className="px-2 py-4 text-xs text-slate-400">Loading...</p>
          )}
          {!loading && filtered.length === 0 && (
            <p className="px-2 py-4 text-xs text-slate-400">No entries.</p>
          )}
          {filtered.map((entry) => {
            const active = selectedLogID === entry.log_id;
            return (
              <button
                key={entry.log_id}
                type="button"
                onClick={() => onSelect(entry.log_id)}
                className={`w-full rounded-md px-2 py-1.5 text-left transition text-xs ${
                  active
                    ? "bg-slate-900 text-white"
                    : "hover:bg-slate-100 text-slate-700"
                }`}
              >
                <p className="font-mono text-[11px] truncate">{entry.log_id}</p>
                <div className={`flex items-center gap-1.5 mt-0.5 text-[10px] ${active ? "text-slate-300" : "text-slate-400"}`}>
                  <span>{formatLastSeen(entry.last_seen)}</span>
                  <span>·</span>
                  <span>{entry.total_count} lines</span>
                </div>
              </button>
            );
          })}
        </div>
      </ScrollArea>
    </div>
  );
}

// ─── Main Page ──────────────────────────────────────────────────────────────

export default function LogAnalyzerPage() {
  const [entries, setEntries] = useState<LogIndexEntry[]>([]);
  const [indexLoading, setIndexLoading] = useState(false);
  const [indexError, setIndexError] = useState<string | null>(null);

  const [selectedLogID, setSelectedLogID] = useState("");
  const [bundle, setBundle] = useState<StructuredLogBundle | null>(null);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadIndex = useCallback(async () => {
    setIndexLoading(true);
    setIndexError(null);
    try {
      const payload = await apiClient.getLogIndex(120);
      setEntries(payload.entries ?? []);
    } catch (err) {
      setIndexError(formatLoadError(err, "Failed to load log index."));
    } finally {
      setIndexLoading(false);
    }
  }, []);

  const loadTrace = useCallback(async (logID: string, searchTerm?: string) => {
    const trimmed = logID.trim();
    if (!trimmed) return;
    setSelectedLogID(trimmed);
    setTraceLoading(true);
    setTraceError(null);
    try {
      const data = await apiClient.getStructuredLogTrace(trimmed, searchTerm || undefined);
      setBundle(data);
    } catch (err) {
      setBundle(null);
      setTraceError(formatLoadError(err, "Failed to load trace."));
    } finally {
      setTraceLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadIndex();
  }, [loadIndex]);

  // Debounced search — re-fetch when search changes
  useEffect(() => {
    if (!selectedLogID) return;
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    searchDebounceRef.current = setTimeout(() => {
      void loadTrace(selectedLogID, search);
    }, 300);
    return () => {
      if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    };
  }, [search, selectedLogID, loadTrace]);

  const handleSelectLogID = useCallback(
    (logID: string) => {
      void loadTrace(logID, search);
    },
    [loadTrace, search],
  );

  const serviceEntries = bundle?.service?.entries ?? [];
  const llmEntries = bundle?.llm?.entries ?? [];
  const latencyEntries = bundle?.latency?.entries ?? [];
  const requestEntries = bundle?.requests?.entries ?? [];

  return (
    <div className="flex h-screen flex-col bg-slate-50">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-200 bg-white px-4 py-2">
          <div>
            <h1 className="text-lg font-semibold text-slate-900">Log Analyzer</h1>
            <p className="text-xs text-slate-500">
              Structured log viewer — select a log_id, browse by category, search within entries.
            </p>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => void loadIndex()}
            disabled={indexLoading}
          >
            {indexLoading ? "Refreshing..." : "Refresh"}
          </Button>
        </div>

        {/* Main layout */}
        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar */}
          <div className="w-72 shrink-0 border-r border-slate-200 bg-white flex flex-col pt-3">
            {indexError && (
              <p className="mx-3 mb-2 rounded-md bg-rose-50 px-2 py-1.5 text-xs text-rose-700">
                {indexError}
              </p>
            )}
            <LogIDSidebar
              entries={entries}
              selectedLogID={selectedLogID}
              onSelect={handleSelectLogID}
              loading={indexLoading}
            />
          </div>

          {/* Content area */}
          <div className="flex-1 flex flex-col overflow-hidden">
            {!selectedLogID ? (
              <div className="flex flex-1 items-center justify-center">
                <p className="text-sm text-slate-400">Select a log_id from the sidebar to begin.</p>
              </div>
            ) : (
              <>
                {/* Search bar */}
                <div className="border-b border-slate-200 bg-white px-4 py-2 flex items-center gap-3">
                  <Input
                    className="max-w-sm h-8 text-xs"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search within logs..."
                  />
                  <span className="text-xs text-slate-400 font-mono">{selectedLogID}</span>
                  {bundle && (
                    <span className="text-[10px] text-slate-400 ml-auto">
                      S:{snippetEntryCount(bundle.service)} L:{snippetEntryCount(bundle.llm)} T:{snippetEntryCount(bundle.latency)} R:{snippetEntryCount(bundle.requests)}
                    </span>
                  )}
                </div>

                {/* Tabs */}
                <div className="flex-1 overflow-auto px-4 py-3">
                  {traceLoading && !bundle && (
                    <p className="py-8 text-center text-sm text-slate-400">Loading trace...</p>
                  )}
                  {traceError && (
                    <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{traceError}</p>
                  )}
                  {bundle && (
                    <Tabs defaultValue="service">
                      <TabsList>
                        <TabsTrigger value="service">
                          Service
                          {serviceEntries.length > 0 && (
                            <Badge variant="secondary" className="ml-1.5 text-[10px] px-1 py-0">
                              {serviceEntries.length}
                            </Badge>
                          )}
                        </TabsTrigger>
                        <TabsTrigger value="llm">
                          LLM
                          {(llmEntries.length > 0 || requestEntries.length > 0) && (
                            <Badge variant="secondary" className="ml-1.5 text-[10px] px-1 py-0">
                              {llmEntries.length + requestEntries.length}
                            </Badge>
                          )}
                        </TabsTrigger>
                        <TabsTrigger value="latency">
                          Latency
                          {latencyEntries.length > 0 && (
                            <Badge variant="secondary" className="ml-1.5 text-[10px] px-1 py-0">
                              {latencyEntries.length}
                            </Badge>
                          )}
                        </TabsTrigger>
                      </TabsList>

                      <TabsContent value="service">
                        <Card>
                          <CardHeader className="py-2 px-3">
                            <CardTitle className="text-sm">Service Logs</CardTitle>
                          </CardHeader>
                          <CardContent className="px-0 pb-3">
                            {bundle.service?.error ? (
                              <SnippetError error={bundle.service.error} />
                            ) : (
                              <TextLogTable entries={serviceEntries} search={search} />
                            )}
                            <TruncationWarning truncated={bundle.service?.truncated} />
                          </CardContent>
                        </Card>
                      </TabsContent>

                      <TabsContent value="llm">
                        <div className="space-y-4">
                          {llmEntries.length > 0 && (
                            <Card>
                              <CardHeader className="py-2 px-3">
                                <CardTitle className="text-sm">LLM Logs</CardTitle>
                              </CardHeader>
                              <CardContent className="px-0 pb-3">
                                <TextLogTable entries={llmEntries} search={search} />
                                <TruncationWarning truncated={bundle.llm?.truncated} />
                              </CardContent>
                            </Card>
                          )}
                          <Card>
                            <CardHeader className="py-2 px-3">
                              <CardTitle className="text-sm">LLM Requests / Responses</CardTitle>
                            </CardHeader>
                            <CardContent className="pb-3">
                              {bundle.requests?.error ? (
                                <SnippetError error={bundle.requests.error} />
                              ) : (
                                <RequestLogList entries={requestEntries} search={search} />
                              )}
                              <TruncationWarning truncated={bundle.requests?.truncated} />
                            </CardContent>
                          </Card>
                        </div>
                      </TabsContent>

                      <TabsContent value="latency">
                        <Card>
                          <CardHeader className="py-2 px-3">
                            <CardTitle className="text-sm">Latency Logs</CardTitle>
                          </CardHeader>
                          <CardContent className="px-0 pb-3">
                            {bundle.latency?.error ? (
                              <SnippetError error={bundle.latency.error} />
                            ) : (
                              <TextLogTable entries={latencyEntries} search={search} />
                            )}
                            <TruncationWarning truncated={bundle.latency?.truncated} />
                          </CardContent>
                        </Card>
                      </TabsContent>
                    </Tabs>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      </div>
  );
}

// ─── Small Shared Components ────────────────────────────────────────────────

function SnippetError({ error }: { error: string }) {
  return (
    <p className="px-3 py-2 text-xs text-rose-600">
      {error === "not_found" ? "Log file not found." : error}
    </p>
  );
}

function TruncationWarning({ truncated }: { truncated?: boolean }) {
  if (!truncated) return null;
  return (
    <p className="px-3 pt-1 text-[10px] text-amber-600">
      Results truncated — refine your search or log_id for more specific results.
    </p>
  );
}
