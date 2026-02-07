"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiClient } from "@/lib/api";
import { LogFileSnippet, LogIndexEntry, LogTraceBundle } from "@/lib/types";

function formatLastSeen(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function countEntries(snippet?: LogFileSnippet): number {
  return snippet?.entries?.length ?? 0;
}

function formatLoadError(err: unknown, fallback: string): string {
  const message = err instanceof Error ? err.message : fallback;
  const lowered = message.toLowerCase();
  if (lowered.includes("404")) {
    return "API /api/dev/logs/index is unavailable (backend may be stale or not in development mode). Run ./dev.sh logs-ui to auto-restart services.";
  }
  if (lowered.includes("401") || lowered.includes("unauthorized")) {
    return "Authentication required. Please sign in again and refresh.";
  }
  return message || fallback;
}

function SnippetView({ title, snippet }: { title: string; snippet?: LogFileSnippet }) {
  const entries = snippet?.entries ?? [];
  return (
    <div className="space-y-2 rounded-lg border border-slate-200 p-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-semibold text-slate-800">{title}</p>
        <p className="text-xs text-slate-500">
          {entries.length} lines{snippet?.truncated ? " · truncated" : ""}
        </p>
      </div>
      {snippet?.error ? (
        <p className="text-xs text-rose-600">{snippet.error === "not_found" ? "log file not found" : snippet.error}</p>
      ) : entries.length === 0 ? (
        <p className="text-xs text-slate-500">No entries.</p>
      ) : (
        <pre className="max-h-64 overflow-auto rounded-md bg-slate-950 p-3 text-xs text-slate-100">
          {entries.join("\n")}
        </pre>
      )}
    </div>
  );
}

export default function LogAnalyzerPage() {
  const [entries, setEntries] = useState<LogIndexEntry[]>([]);
  const [indexLoading, setIndexLoading] = useState(false);
  const [indexError, setIndexError] = useState<string | null>(null);
  const [keyword, setKeyword] = useState("");

  const [selectedLogID, setSelectedLogID] = useState<string>("");
  const [trace, setTrace] = useState<LogTraceBundle | null>(null);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState<string | null>(null);

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

  const loadTrace = useCallback(async (logID: string) => {
    const trimmed = logID.trim();
    if (!trimmed) {
      return;
    }
    setSelectedLogID(trimmed);
    setTraceLoading(true);
    setTraceError(null);
    try {
      const bundle = await apiClient.getLogTrace(trimmed);
      setTrace(bundle);
    } catch (err) {
      setTrace(null);
      setTraceError(formatLoadError(err, "Failed to load trace."));
    } finally {
      setTraceLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadIndex();
  }, [loadIndex]);

  const filteredEntries = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter((entry) => {
      if (entry.log_id.toLowerCase().includes(q)) return true;
      return (entry.sources ?? []).some((source) => source.toLowerCase().includes(q));
    });
  }, [entries, keyword]);

  return (
    <RequireAuth>
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-7xl flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle>Log Analyzer</CardTitle>
              <CardDescription>
                Visualize recent log chains by <code>log_id</code> and inspect Service/LLM/Latency/Request logs in one page.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center gap-2">
              <Input
                className="max-w-sm"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="Filter by log_id or source"
              />
              <Button onClick={() => void loadIndex()} disabled={indexLoading}>
                {indexLoading ? "Refreshing..." : "Refresh Index"}
              </Button>
            </CardContent>
          </Card>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Recent Log IDs</CardTitle>
                <CardDescription>
                  {filteredEntries.length} / {entries.length} entries
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-2">
                {indexError && (
                  <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{indexError}</p>
                )}
                {!indexError && filteredEntries.length === 0 && (
                  <p className="text-sm text-slate-500">No entries.</p>
                )}
                <div className="max-h-[60vh] space-y-2 overflow-auto pr-1">
                  {filteredEntries.map((entry) => {
                    const active = selectedLogID === entry.log_id;
                    return (
                      <button
                        key={entry.log_id}
                        type="button"
                        onClick={() => void loadTrace(entry.log_id)}
                        className={`w-full rounded-lg border p-3 text-left transition ${
                          active
                            ? "border-slate-900 bg-slate-900 text-white"
                            : "border-slate-200 bg-white hover:border-slate-400"
                        }`}
                      >
                        <p className="font-mono text-xs">{entry.log_id}</p>
                        <p className={`mt-1 text-xs ${active ? "text-slate-200" : "text-slate-500"}`}>
                          last_seen: {formatLastSeen(entry.last_seen)}
                        </p>
                        <p className={`mt-1 text-xs ${active ? "text-slate-200" : "text-slate-500"}`}>
                          total={entry.total_count} · service={entry.service_count} · llm={entry.llm_count} · latency=
                          {entry.latency_count} · request={entry.request_count}
                        </p>
                      </button>
                    );
                  })}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Log Trace Detail</CardTitle>
                <CardDescription>{selectedLogID ? `log_id=${selectedLogID}` : "Select a log_id from left panel."}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {traceLoading && <p className="text-sm text-slate-500">Loading trace...</p>}
                {traceError && <p className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{traceError}</p>}
                {!traceLoading && !traceError && trace && (
                  <>
                    <div className="rounded-lg border border-slate-200 bg-slate-100 px-3 py-2 text-xs text-slate-700">
                      matched lines: service={countEntries(trace.service)}, llm={countEntries(trace.llm)}, latency=
                      {countEntries(trace.latency)}, requests={countEntries(trace.requests)}
                    </div>
                    <SnippetView title="Service" snippet={trace.service} />
                    <SnippetView title="LLM" snippet={trace.llm} />
                    <SnippetView title="Latency" snippet={trace.latency} />
                    <SnippetView title="Requests" snippet={trace.requests} />
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </RequireAuth>
  );
}
