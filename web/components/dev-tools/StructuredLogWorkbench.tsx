"use client";

import React, { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient } from "@/lib/api";
import type {
  LogIndexEntry,
  ParsedRequestLogEntry,
  ParsedTextLogEntry,
  StructuredLogBundle,
  StructuredLogSnippet,
  StructuredRequestSnippet,
} from "@/lib/types";
import { HighlightText } from "@/components/dev-tools/shared/highlight-text";
import { JsonTreeViewer } from "@/components/dev-tools/shared/json-tree-viewer";
import { useDebouncedValue } from "@/components/dev-tools/shared/useDebouncedValue";
import { VirtualizedList } from "@/components/dev-tools/shared/virtualized-list";

const INDEX_PAGE_SIZE = 80;

type SidebarFilter = "all" | "llm" | "service" | "latency";

function formatLastSeen(value: string): string {
  if (!value) return "-";
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

function matchesSidebarFilter(entry: LogIndexEntry, filter: SidebarFilter): boolean {
  switch (filter) {
    case "all":
      return true;
    case "llm":
      return entry.llm_count > 0 || entry.request_count > 0;
    case "service":
      return entry.service_count > 0;
    case "latency":
      return entry.latency_count > 0;
    default:
      return true;
  }
}

function LogLevelBadge({ level }: { level: string }) {
  const variant =
    {
      DEBUG: "secondary" as const,
      INFO: "info" as const,
      WARN: "warning" as const,
      ERROR: "destructive" as const,
    }[level.toUpperCase()] ?? ("outline" as const);

  return (
    <Badge variant={variant} className="px-1.5 py-0 font-mono text-[10px]">
      {level || "???"}
    </Badge>
  );
}

const LogIDSidebarItem = React.memo(function LogIDSidebarItem({
  entry,
  active,
  onSelect,
}: {
  entry: LogIndexEntry;
  active: boolean;
  onSelect: (logID: string) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(entry.log_id)}
      className={`w-full rounded-md px-2 py-1.5 text-left text-xs transition ${
        active ? "bg-slate-900 text-white" : "text-slate-700 hover:bg-slate-100"
      }`}
    >
      <p className="truncate font-mono text-[11px]">{entry.log_id}</p>
      <div
        className={`mt-0.5 flex items-center gap-1.5 text-[10px] ${
          active ? "text-slate-300" : "text-slate-400"
        }`}
      >
        <span>{formatLastSeen(entry.last_seen)}</span>
        <span>-</span>
        <span>{entry.total_count} lines</span>
      </div>
    </button>
  );
});

const TextLogRow = React.memo(function TextLogRow({
  entry,
  search,
  expanded,
  onToggle,
}: {
  entry: ParsedTextLogEntry;
  search: string;
  expanded: boolean;
  onToggle: () => void;
}) {
  const isLong = entry.message.length > 200;
  return (
    <tr className="border-b border-slate-100 transition-colors hover:bg-slate-50">
      <td className="w-[72px] whitespace-nowrap px-2 py-1 font-mono align-top text-slate-500">
        {formatTimestamp(entry.timestamp)}
      </td>
      <td className="w-[60px] px-2 py-1 align-top">
        <LogLevelBadge level={entry.level} />
      </td>
      <td className="w-[80px] px-2 py-1 align-top text-slate-600">{entry.component}</td>
      <td className="w-[90px] whitespace-nowrap px-2 py-1 font-mono align-top text-slate-400">
        {entry.source_file ? `${entry.source_file}:${entry.source_line}` : ""}
      </td>
      <td className="px-2 py-1 align-top">
        <div className={`${!expanded && isLong ? "line-clamp-2" : ""} break-all`}>
          <HighlightText text={entry.message} search={search} />
        </div>
        {isLong ? (
          <button
            type="button"
            onClick={onToggle}
            className="mt-0.5 text-[10px] text-sky-600 hover:text-sky-800"
          >
            {expanded ? "Collapse" : "Expand"}
          </button>
        ) : null}
      </td>
    </tr>
  );
});

function TextLogTable({
  entries,
  search,
}: {
  entries: ParsedTextLogEntry[];
  search: string;
}) {
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const parentRef = useRef<HTMLDivElement>(null);

  const toggleRow = useCallback((idx: number) => {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }, []);

  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: entries.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 34,
    overscan: 10,
  });

  if (entries.length === 0) {
    return <p className="py-8 text-center text-sm text-slate-500">No log entries found.</p>;
  }

  const virtualItems = virtualizer.getVirtualItems();
  const firstStart = virtualItems[0]?.start ?? 0;
  const lastEnd = virtualItems[virtualItems.length - 1]?.end ?? 0;

  return (
    <div ref={parentRef} className="max-h-[70vh] overflow-auto">
      <table className="w-full text-xs">
        <thead className="sticky top-0 z-10 bg-white">
          <tr className="border-b border-slate-200 text-left text-slate-500">
            <th className="w-[72px] px-2 py-1.5 font-medium">Time</th>
            <th className="w-[60px] px-2 py-1.5 font-medium">Level</th>
            <th className="w-[80px] px-2 py-1.5 font-medium">Component</th>
            <th className="w-[90px] px-2 py-1.5 font-medium">Source</th>
            <th className="px-2 py-1.5 font-medium">Message</th>
          </tr>
        </thead>
        <tbody>
          {virtualItems.length > 0 ? (
            <tr style={{ height: firstStart }}>
              <td colSpan={5} />
            </tr>
          ) : null}
          {virtualItems.map((virtualRow) => {
            const entry = entries[virtualRow.index];
            return (
              <TextLogRow
                key={`txt-${virtualRow.index}`}
                entry={entry}
                search={search}
                expanded={expandedRows.has(virtualRow.index)}
                onToggle={() => toggleRow(virtualRow.index)}
              />
            );
          })}
          {virtualItems.length > 0 ? (
            <tr style={{ height: Math.max(0, virtualizer.getTotalSize() - lastEnd) }}>
              <td colSpan={5} />
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}

function RequestPayloadPreview({ payload }: { payload: unknown }) {
  const [expanded, setExpanded] = useState(false);

  if (payload === undefined || payload === null) {
    return <span className="text-xs italic text-slate-400">no payload</span>;
  }

  if (!expanded) {
    return (
      <div className="mt-1">
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="h-6 px-2 text-[10px]"
          onClick={() => setExpanded(true)}
        >
          Expand payload
        </Button>
      </div>
    );
  }

  return (
    <div className="mt-1 space-y-1">
      <Button
        type="button"
        size="sm"
        variant="outline"
        className="h-6 px-2 text-[10px]"
        onClick={() => setExpanded(false)}
      >
        Collapse payload
      </Button>
      <div className="max-h-96 overflow-auto rounded-md border border-slate-200 bg-slate-50 p-2">
        <JsonTreeViewer data={payload} rootLabel="payload" initiallyExpandedDepth={1} />
      </div>
    </div>
  );
}

const RequestLogItem = React.memo(function RequestLogItem({
  entry,
  search,
}: {
  entry: ParsedRequestLogEntry;
  search: string;
}) {
  return (
    <div className="space-y-1 rounded-lg border border-slate-200 bg-white p-3">
      <div className="flex items-center gap-2 text-xs">
        <Badge
          variant={entry.entry_type === "request" ? "info" : "success"}
          className="px-1.5 py-0 font-mono text-[10px]"
        >
          {entry.entry_type}
        </Badge>
        <span className="font-mono text-slate-500">
          <HighlightText text={entry.request_id} search={search} />
        </span>
        <span className="text-slate-400">{formatTimestamp(entry.timestamp)}</span>
        <span className="text-slate-400">{formatBytes(entry.body_bytes)}</span>
      </div>
      <RequestPayloadPreview payload={entry.payload} />
    </div>
  );
});

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
    <VirtualizedList
      items={entries}
      estimateSize={132}
      overscan={5}
      className="max-h-[70vh] overflow-auto"
      contentClassName="w-full"
      itemKey={(entry, idx) => `${entry.request_id}-${idx}`}
      renderItem={(entry) => (
        <div className="pb-3">
          <RequestLogItem entry={entry} search={search} />
        </div>
      )}
    />
  );
}

const FILTER_OPTIONS: { value: SidebarFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "llm", label: "LLM" },
  { value: "service", label: "Service" },
  { value: "latency", label: "Latency" },
];

function SidebarFilterButtons({
  active,
  onChange,
}: {
  active: SidebarFilter;
  onChange: (filter: SidebarFilter) => void;
}) {
  return (
    <div className="flex gap-1 px-3 pb-2">
      {FILTER_OPTIONS.map((option) => (
        <Button
          key={option.value}
          size="sm"
          variant={active === option.value ? "default" : "outline"}
          className="h-6 px-2 text-[10px]"
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </Button>
      ))}
    </div>
  );
}

function LogIDSidebar({
  entries,
  selectedLogID,
  onSelect,
  loading,
  hasMore,
  onLoadMore,
  sidebarFilter,
  onFilterChange,
}: {
  entries: LogIndexEntry[];
  selectedLogID: string;
  onSelect: (logID: string) => void;
  loading: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  sidebarFilter: SidebarFilter;
  onFilterChange: (filter: SidebarFilter) => void;
}) {
  const [textFilter, setTextFilter] = useState("");
  const debouncedFilter = useDebouncedValue(textFilter, 180);
  const deferredEntries = useDeferredValue(entries);

  const filtered = useMemo(() => {
    let result = deferredEntries;
    if (sidebarFilter !== "all") {
      result = result.filter((entry) => matchesSidebarFilter(entry, sidebarFilter));
    }

    const query = debouncedFilter.trim().toLowerCase();
    if (!query) {
      return result;
    }

    return result.filter(
      (entry) =>
        entry.log_id.toLowerCase().includes(query) ||
        (entry.sources ?? []).some((source) => source.toLowerCase().includes(query)),
    );
  }, [deferredEntries, sidebarFilter, debouncedFilter]);

  return (
    <div className="flex h-full flex-col">
      <SidebarFilterButtons active={sidebarFilter} onChange={onFilterChange} />
      <div className="px-3 pb-2">
        <Input
          className="h-8 text-xs"
          value={textFilter}
          onChange={(event) => setTextFilter(event.target.value)}
          placeholder="Filter log IDs..."
        />
      </div>
      <div className="px-3 pb-1 text-[10px] text-slate-400">
        {filtered.length} / {entries.length} IDs
      </div>
      <VirtualizedList
        items={filtered}
        estimateSize={52}
        overscan={12}
        className="flex-1 overflow-auto px-2"
        contentClassName="pb-2"
        itemKey={(entry) => entry.log_id}
        empty={<p className="px-2 py-4 text-xs text-slate-400">No entries.</p>}
        renderItem={(entry) => (
          <div className="pb-1">
            <LogIDSidebarItem
              entry={entry}
              active={selectedLogID === entry.log_id}
              onSelect={onSelect}
            />
          </div>
        )}
      />
      <div className="border-t border-slate-200 p-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="w-full text-xs"
          disabled={!hasMore || loading}
          onClick={onLoadMore}
        >
          {loading && entries.length > 0 ? "Loading..." : hasMore ? "Load more" : "No more"}
        </Button>
      </div>
    </div>
  );
}

export function StructuredLogWorkbench() {
  const [entries, setEntries] = useState<LogIndexEntry[]>([]);
  const [indexLoading, setIndexLoading] = useState(false);
  const [indexError, setIndexError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);

  const [sidebarFilter, setSidebarFilter] = useState<SidebarFilter>("llm");

  const [selectedLogID, setSelectedLogID] = useState("");
  const selectedLogIDRef = useRef(selectedLogID);
  selectedLogIDRef.current = selectedLogID;

  const [bundle, setBundle] = useState<StructuredLogBundle | null>(null);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState<string | null>(null);

  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(searchInput, 260);
  const entriesRef = useRef(entries);
  entriesRef.current = entries;

  const loadIndex = useCallback(async (offset = 0) => {
    setIndexLoading(true);
    if (offset === 0) {
      setIndexError(null);
    }
    try {
      const payload = await apiClient.getLogIndex(INDEX_PAGE_SIZE, offset);
      const newEntries = payload.entries ?? [];
      setHasMore(payload.has_more ?? false);
      if (offset === 0) {
        setEntries(newEntries);
      } else {
        setEntries((prev) => {
          const seen = new Set(prev.map((entry) => entry.log_id));
          const unique = newEntries.filter((entry) => !seen.has(entry.log_id));
          return [...prev, ...unique];
        });
      }
    } catch (error) {
      setIndexError(formatLoadError(error, "Failed to load log index."));
    } finally {
      setIndexLoading(false);
    }
  }, []);

  const loadMore = useCallback(() => {
    void loadIndex(entriesRef.current.length);
  }, [loadIndex]);

  const loadTrace = useCallback(async (logID: string, searchTerm?: string) => {
    const trimmed = logID.trim();
    if (!trimmed) return;

    setSelectedLogID(trimmed);
    setTraceLoading(true);
    setTraceError(null);

    try {
      const data = await apiClient.getStructuredLogTrace(trimmed, searchTerm || undefined);
      setBundle(data);
    } catch (error) {
      setBundle(null);
      setTraceError(formatLoadError(error, "Failed to load trace."));
    } finally {
      setTraceLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadIndex(0);
  }, [loadIndex]);

  useEffect(() => {
    if (entries.length > 0 && !selectedLogIDRef.current) {
      void loadTrace(entries[0].log_id, debouncedSearch);
    }
  }, [entries, loadTrace, debouncedSearch]);

  useEffect(() => {
    if (!selectedLogID) return;
    void loadTrace(selectedLogID, debouncedSearch);
  }, [debouncedSearch, loadTrace, selectedLogID]);

  const handleSelectLogID = useCallback(
    (logID: string) => {
      void loadTrace(logID, debouncedSearch);
    },
    [loadTrace, debouncedSearch],
  );

  const serviceEntries = useMemo(() => bundle?.service?.entries ?? [], [bundle]);
  const llmEntries = useMemo(() => bundle?.llm?.entries ?? [], [bundle]);
  const latencyEntries = useMemo(() => bundle?.latency?.entries ?? [], [bundle]);
  const requestEntries = useMemo(() => bundle?.requests?.entries ?? [], [bundle]);

  return (
    <div className="flex h-[calc(100vh-220px)] min-h-[640px] min-w-0 overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div className="flex w-72 shrink-0 flex-col border-r border-slate-200 bg-white pt-3">
        {indexError ? (
          <p className="mx-3 mb-2 rounded-md bg-rose-50 px-2 py-1.5 text-xs text-rose-700">{indexError}</p>
        ) : null}
        <LogIDSidebar
          entries={entries}
          selectedLogID={selectedLogID}
          onSelect={handleSelectLogID}
          loading={indexLoading}
          hasMore={hasMore}
          onLoadMore={loadMore}
          sidebarFilter={sidebarFilter}
          onFilterChange={setSidebarFilter}
        />
      </div>

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {!selectedLogID ? (
          <div className="flex flex-1 items-center justify-center">
            <p className="text-sm text-slate-400">Select a log_id from the sidebar to begin.</p>
          </div>
        ) : (
          <>
            <div className="flex items-center gap-3 border-b border-slate-200 bg-white px-4 py-2">
              <Input
                className="h-8 max-w-sm text-xs"
                value={searchInput}
                onChange={(event) => setSearchInput(event.target.value)}
                placeholder="Search within logs..."
              />
              <span className="font-mono text-xs text-slate-400">{selectedLogID}</span>
              {bundle ? (
                <span className="ml-auto text-[10px] text-slate-400">
                  S:{snippetEntryCount(bundle.service)} L:{snippetEntryCount(bundle.llm)} T:{snippetEntryCount(bundle.latency)} R:{snippetEntryCount(bundle.requests)}
                </span>
              ) : null}
            </div>

            <div className="flex-1 overflow-auto px-4 py-3">
              {traceLoading && !bundle ? (
                <p className="py-8 text-center text-sm text-slate-400">Loading trace...</p>
              ) : null}
              {traceError ? (
                <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{traceError}</p>
              ) : null}

              {bundle ? (
                <Tabs defaultValue="service">
                  <TabsList>
                    <TabsTrigger value="service">
                      Service
                      {serviceEntries.length > 0 ? (
                        <Badge variant="secondary" className="ml-1.5 px-1 py-0 text-[10px]">
                          {serviceEntries.length}
                        </Badge>
                      ) : null}
                    </TabsTrigger>
                    <TabsTrigger value="llm">
                      LLM
                      {llmEntries.length > 0 || requestEntries.length > 0 ? (
                        <Badge variant="secondary" className="ml-1.5 px-1 py-0 text-[10px]">
                          {llmEntries.length + requestEntries.length}
                        </Badge>
                      ) : null}
                    </TabsTrigger>
                    <TabsTrigger value="latency">
                      Latency
                      {latencyEntries.length > 0 ? (
                        <Badge variant="secondary" className="ml-1.5 px-1 py-0 text-[10px]">
                          {latencyEntries.length}
                        </Badge>
                      ) : null}
                    </TabsTrigger>
                  </TabsList>

                  <TabsContent value="service">
                    <Card>
                      <CardHeader className="px-3 py-2">
                        <CardTitle className="text-sm">Service Logs</CardTitle>
                      </CardHeader>
                      <CardContent className="px-0 pb-3">
                        {bundle.service?.error ? (
                          <SnippetError error={bundle.service.error} />
                        ) : (
                          <TextLogTable entries={serviceEntries} search={debouncedSearch} />
                        )}
                        <TruncationWarning truncated={bundle.service?.truncated} />
                      </CardContent>
                    </Card>
                  </TabsContent>

                  <TabsContent value="llm">
                    <div className="space-y-4">
                      {llmEntries.length > 0 ? (
                        <Card>
                          <CardHeader className="px-3 py-2">
                            <CardTitle className="text-sm">LLM Logs</CardTitle>
                          </CardHeader>
                          <CardContent className="px-0 pb-3">
                            <TextLogTable entries={llmEntries} search={debouncedSearch} />
                            <TruncationWarning truncated={bundle.llm?.truncated} />
                          </CardContent>
                        </Card>
                      ) : null}

                      <Card>
                        <CardHeader className="px-3 py-2">
                          <CardTitle className="text-sm">LLM Requests / Responses</CardTitle>
                        </CardHeader>
                        <CardContent className="pb-3">
                          {bundle.requests?.error ? (
                            <SnippetError error={bundle.requests.error} />
                          ) : (
                            <RequestLogList entries={requestEntries} search={debouncedSearch} />
                          )}
                          <TruncationWarning truncated={bundle.requests?.truncated} />
                        </CardContent>
                      </Card>
                    </div>
                  </TabsContent>

                  <TabsContent value="latency">
                    <Card>
                      <CardHeader className="px-3 py-2">
                        <CardTitle className="text-sm">Latency Logs</CardTitle>
                      </CardHeader>
                      <CardContent className="px-0 pb-3">
                        {bundle.latency?.error ? (
                          <SnippetError error={bundle.latency.error} />
                        ) : (
                          <TextLogTable entries={latencyEntries} search={debouncedSearch} />
                        )}
                        <TruncationWarning truncated={bundle.latency?.truncated} />
                      </CardContent>
                    </Card>
                  </TabsContent>
                </Tabs>
              ) : null}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function SnippetError({ error }: { error: string }) {
  return <p className="px-3 py-2 text-xs text-rose-600">{error === "not_found" ? "Log file not found." : error}</p>;
}

function TruncationWarning({ truncated }: { truncated?: boolean }) {
  if (!truncated) return null;
  return (
    <p className="px-3 pt-1 text-[10px] text-amber-600">
      Results truncated. Refine your search or log_id for more specific results.
    </p>
  );
}
