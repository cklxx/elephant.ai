"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import { Play, RefreshCw, Square, Trash2 } from "lucide-react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { useSessionStore, useSessions } from "@/hooks/useSessionStore";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient, type SSEReplayMode, type SessionSnapshotsResponse, type MemoryEntry } from "@/lib/api";
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
  "proactive.context.refresh",
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

/** Derive log_id from llm_request_id. Format: `<log_id>:llm-<ksuid>` or `<parent>:sub:<child>:llm-<ksuid>` */
function deriveLogIdFromRequestId(requestId: string): string | null {
  if (!requestId) return null;
  // Find the last `:llm-` segment and take everything before it as the log_id
  const llmIdx = requestId.lastIndexOf(":llm-");
  if (llmIdx > 0) {
    return requestId.slice(0, llmIdx);
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

type ToolTimingEntry = TimingEntry & {
  seq?: number;
  callId?: string;
};

type LLMTimingEntry = TimingEntry & {
  seq?: number;
  logId?: string;
  requestId?: string;
  model?: string;
};

type RunNode = {
  runId: string;
  parentRunId: string | null;
  correlationId: string | null;
  causationId: string | null;
  agentLevel: string;
  children: RunNode[];
  stages: TimingEntry[];
  tools: ToolTimingEntry[];
  llmCalls: LLMTimingEntry[];
  totalDurationMs: number | null;
  firstSeq: number | null;
};

type RunTree = {
  roots: RunNode[];
  totalDurationMs: number | null;
  stageCount: number;
  toolCount: number;
  llmCount: number;
};

type LLMDetailCache = Map<string, LogTraceBundle | "loading" | "error">;

function formatDuration(ms: number) {
  if (!Number.isFinite(ms)) {
    return "—";
  }
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`;
  }
  return `${Math.round(ms)}ms`;
}


function truncateId(id: string, maxLen = 12): string {
  if (id.length <= maxLen) return id;
  return `${id.slice(0, maxLen)}…`;
}

function LLMCallDetailView({
  entry,
  cache,
  onFetch,
}: {
  entry: LLMTimingEntry;
  cache: LLMDetailCache;
  onFetch: (logId: string) => void;
}) {
  const logId = entry.logId;
  const cached = logId ? cache.get(logId) : undefined;

  const handleExpand = useCallback(() => {
    if (logId && !cached) {
      onFetch(logId);
    }
  }, [logId, cached, onFetch]);

  const requestsSnippet = useMemo(() => {
    if (!cached || cached === "loading" || cached === "error") return null;
    return cached.requests ?? null;
  }, [cached]);

  const requestEntries = useMemo(() => {
    if (!requestsSnippet) return [];
    const raw = requestsSnippet.entries ?? [];
    return raw.map((line) => {
      try {
        return JSON.parse(line) as Record<string, unknown>;
      } catch {
        return null;
      }
    }).filter((item): item is Record<string, unknown> => item !== null);
  }, [requestsSnippet]);

  const matchedEntries = useMemo(() => {
    if (!entry.requestId || requestEntries.length === 0) return requestEntries;
    // Try exact match on request_id
    const exact = requestEntries.filter(
      (item) => item.request_id === entry.requestId,
    );
    if (exact.length > 0) return exact;
    // Fall back: match by llm-<ksuid> suffix (the entry may store a shorter form)
    const llmSuffix = entry.requestId.includes(":llm-")
      ? entry.requestId.slice(entry.requestId.lastIndexOf(":llm-") + 1)
      : null;
    if (llmSuffix) {
      const bySuffix = requestEntries.filter((item) => {
        const rid = typeof item.request_id === "string" ? item.request_id : "";
        return rid === llmSuffix || rid.endsWith(llmSuffix);
      });
      if (bySuffix.length > 0) return bySuffix;
    }
    // Final fallback: return all entries (already scoped by log_id on the backend)
    return requestEntries;
  }, [requestEntries, entry.requestId]);

  const requestPayloads = useMemo(
    () => matchedEntries.filter((item) => item.entry_type === "request"),
    [matchedEntries],
  );
  const responsePayloads = useMemo(
    () => matchedEntries.filter((item) => item.entry_type === "response"),
    [matchedEntries],
  );

  return (
    <details
      className="rounded-md border border-border/60 bg-muted/10"
      onToggle={(e) => {
        if ((e.target as HTMLDetailsElement).open) handleExpand();
      }}
    >
      <summary className="flex cursor-pointer items-center justify-between gap-2 px-3 py-2 text-xs">
        <div className="flex min-w-0 items-center gap-2">
          <span className="font-medium text-foreground">{entry.label}</span>
          {entry.model && (
            <span className="text-[11px] text-muted-foreground">model={entry.model}</span>
          )}
          {entry.seq !== undefined && (
            <span className="text-[11px] text-muted-foreground">seq:{entry.seq}</span>
          )}
        </div>
        <span className="whitespace-nowrap font-semibold text-foreground/80">
          {formatDuration(entry.durationMs)}
        </span>
      </summary>
      <div className="space-y-2 border-t border-border/40 px-3 py-2">
        {!logId && (
          <p className="text-xs text-muted-foreground">No log_id available for this LLM call.</p>
        )}
        {logId && cached === "loading" && (
          <p className="text-xs text-muted-foreground">Loading log trace…</p>
        )}
        {logId && cached === "error" && (
          <p className="text-xs text-rose-600">Failed to load log trace.</p>
        )}
        {logId && cached && cached !== "loading" && cached !== "error" && (
          <>
            {requestsSnippet?.error && (
              <p className="text-xs text-rose-600">
                Request log error: {requestsSnippet.error === "not_found" ? "llm.jsonl not found" : requestsSnippet.error}
                {requestsSnippet.path && <span className="ml-1 text-muted-foreground">(path: {requestsSnippet.path})</span>}
              </p>
            )}
            {!requestsSnippet?.error && matchedEntries.length === 0 && (
              <p className="text-xs text-muted-foreground">
                No request log entries found (log_id={logId}, raw={requestsSnippet?.entries?.length ?? 0}, path={requestsSnippet?.path ?? "—"}).
              </p>
            )}
            {matchedEntries.length > 0 && (
              <>
                {requestPayloads.length > 0 && (
                  <details className="rounded-md border border-dashed border-border/70 bg-muted/30 px-2 py-1">
                    <summary className="cursor-pointer text-xs font-semibold text-foreground/80">
                      Request ({requestPayloads.length})
                    </summary>
                    <div className="mt-2 space-y-2">
                      {requestPayloads.map((item, idx) => (
                        <JsonNode key={`req-${idx}`} label={`request[${idx}]`} value={item} depth={0} />
                      ))}
                    </div>
                  </details>
                )}
                {responsePayloads.length > 0 && (
                  <details className="rounded-md border border-dashed border-border/70 bg-muted/30 px-2 py-1">
                    <summary className="cursor-pointer text-xs font-semibold text-foreground/80">
                      Response ({responsePayloads.length})
                    </summary>
                    <div className="mt-2 space-y-2">
                      {responsePayloads.map((item, idx) => (
                        <JsonNode key={`res-${idx}`} label={`response[${idx}]`} value={item} depth={0} />
                      ))}
                    </div>
                  </details>
                )}
              </>
            )}
          </>
        )}
      </div>
    </details>
  );
}

function RunNodeView({
  node,
  cache,
  onFetchLLM,
  depth = 0,
}: {
  node: RunNode;
  cache: LLMDetailCache;
  onFetchLLM: (logId: string) => void;
  depth?: number;
}) {
  const hasContent =
    node.stages.length > 0 ||
    node.tools.length > 0 ||
    node.llmCalls.length > 0 ||
    node.children.length > 0;

  const idChainParts: string[] = [];
  if (node.correlationId) idChainParts.push(`corr: ${truncateId(node.correlationId)}`);
  if (node.causationId) idChainParts.push(`cause: ${truncateId(node.causationId)}`);
  if (node.parentRunId) idChainParts.push(`parent: ${truncateId(node.parentRunId)}`);

  return (
    <details open={depth < 2} className="rounded-lg border border-border/60 bg-muted/10">
      <summary className="flex cursor-pointer items-center gap-2 px-3 py-2 text-xs">
        <Badge variant={node.agentLevel === "core" ? "default" : "secondary"} className="text-[10px]">
          {node.agentLevel}
        </Badge>
        <span className="font-medium text-foreground" title={node.runId}>
          {node.runId === "__root__" ? "(no run_id)" : truncateId(node.runId, 16)}
        </span>
        {node.totalDurationMs !== null && (
          <span className="font-semibold text-foreground/80">
            {formatDuration(node.totalDurationMs)}
          </span>
        )}
        {node.firstSeq !== null && (
          <span className="text-[11px] text-muted-foreground">seq:{node.firstSeq}</span>
        )}
        {idChainParts.length > 0 && (
          <span className="text-[10px] text-muted-foreground/70">
            {idChainParts.join(" · ")}
          </span>
        )}
      </summary>
      {hasContent && (
        <div className="space-y-3 border-t border-border/40 px-3 py-2">
          {node.stages.length > 0 && (
            <div className="space-y-1">
              <p className="text-[11px] font-semibold text-muted-foreground">Stages</p>
              <div className="space-y-1">
                {node.stages.map((stage) => (
                  <div
                    key={stage.id}
                    className="flex items-center justify-between gap-2 rounded border border-border/40 bg-muted/20 px-2 py-1 text-xs"
                  >
                    <span className="truncate font-medium text-foreground">{stage.label}</span>
                    <span className="whitespace-nowrap font-semibold text-foreground/80">
                      {formatDuration(stage.durationMs)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {node.tools.length > 0 && (
            <div className="space-y-1">
              <p className="text-[11px] font-semibold text-muted-foreground">Tools</p>
              <div className="space-y-1">
                {node.tools.map((tool) => (
                  <div
                    key={tool.id}
                    className="flex items-center justify-between gap-2 rounded border border-border/40 bg-muted/20 px-2 py-1 text-xs"
                  >
                    <div className="min-w-0">
                      <span className="truncate font-medium text-foreground">{tool.label}</span>
                      <span className="ml-2 text-[11px] text-muted-foreground">
                        {[
                          tool.seq !== undefined && `seq:${tool.seq}`,
                          tool.callId && `call:${truncateId(tool.callId, 10)}`,
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </span>
                      {tool.hint && (
                        <p className="truncate text-[10px] text-muted-foreground">{tool.hint}</p>
                      )}
                    </div>
                    <span className="whitespace-nowrap font-semibold text-foreground/80">
                      {formatDuration(tool.durationMs)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {node.llmCalls.length > 0 && (
            <div className="space-y-1">
              <p className="text-[11px] font-semibold text-muted-foreground">LLM Calls</p>
              <div className="space-y-1">
                {node.llmCalls.map((llm) => (
                  <LLMCallDetailView
                    key={llm.id}
                    entry={llm}
                    cache={cache}
                    onFetch={onFetchLLM}
                  />
                ))}
              </div>
            </div>
          )}

          {node.children.length > 0 && (
            <div className="space-y-1">
              <p className="text-[11px] font-semibold text-muted-foreground">
                Children ({node.children.length})
              </p>
              <div className="space-y-2 pl-2">
                {node.children.map((child) => (
                  <RunNodeView
                    key={child.runId}
                    node={child}
                    cache={cache}
                    onFetchLLM={onFetchLLM}
                    depth={depth + 1}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </details>
  );
}

function RunTreeView({
  tree,
  cache,
  onFetchLLM,
}: {
  tree: RunTree;
  cache: LLMDetailCache;
  onFetchLLM: (logId: string) => void;
}) {
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <Badge variant="outline">
          Total:{" "}
          {tree.totalDurationMs !== null ? formatDuration(tree.totalDurationMs) : "—"}
        </Badge>
        <Badge variant="outline">Stages: {tree.stageCount}</Badge>
        <Badge variant="outline">Tools: {tree.toolCount}</Badge>
        <Badge variant="outline">LLM calls: {tree.llmCount}</Badge>
      </div>

      {tree.roots.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">
          No timing data yet. Connect to a session to capture events.
        </div>
      ) : (
        <div className="space-y-2">
          {tree.roots.map((root) => (
            <RunNodeView
              key={root.runId}
              node={root}
              cache={cache}
              onFetchLLM={onFetchLLM}
            />
          ))}
        </div>
      )}
    </div>
  );
}

const SOURCE_BADGE_VARIANTS: Record<string, { variant: "default" | "secondary" | "outline" | "destructive"; label: string }> = {
  proactive_context: { variant: "default", label: "Memory" },
  user_input: { variant: "secondary", label: "User" },
  system_prompt: { variant: "outline", label: "System" },
  user_history: { variant: "outline", label: "History" },
};

function sourceBadgeFor(source: string | undefined, role: string | undefined) {
  if (source && SOURCE_BADGE_VARIANTS[source]) {
    return SOURCE_BADGE_VARIANTS[source];
  }
  if (role === "assistant") return { variant: "secondary" as const, label: "Assistant" };
  if (role === "user") return { variant: "secondary" as const, label: "User" };
  if (role === "system") return { variant: "outline" as const, label: "System" };
  return { variant: "outline" as const, label: source || role || "unknown" };
}

/** Rough token estimate: ~4 chars/token for ASCII, ~1.5 chars/token for CJK. */
function estimateTokens(text: string): number {
  if (!text) return 0;
  let asciiChars = 0;
  let cjkChars = 0;
  for (const ch of text) {
    const code = ch.codePointAt(0) ?? 0;
    if (code > 0x2e80) {
      cjkChars++;
    } else {
      asciiChars++;
    }
  }
  return Math.ceil(asciiChars / 4 + cjkChars / 1.5);
}

function formatTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

function TurnMessagesViewer({
  turnSnapshot,
  sessionChannel,
}: {
  turnSnapshot: Record<string, unknown>;
  sessionChannel: string | null;
}) {
  const messages = useMemo(() => {
    const raw = turnSnapshot.messages;
    return Array.isArray(raw) ? (raw as Array<Record<string, unknown>>) : [];
  }, [turnSnapshot.messages]);

  const tokenStats = useMemo(() => {
    return messages.reduce<Array<{ tokens: number; cumulative: number }>>((acc, msg) => {
      const content = typeof msg.content === "string" ? msg.content : "";
      const toolCalls = Array.isArray(msg.tool_calls)
        ? JSON.stringify(msg.tool_calls).length
        : 0;
      const toolResults = Array.isArray(msg.tool_results)
        ? JSON.stringify(msg.tool_results).length
        : 0;
      const tokens = estimateTokens(content) + Math.ceil(toolCalls / 4) + Math.ceil(toolResults / 4);
      const prev = acc.length > 0 ? acc[acc.length - 1].cumulative : 0;
      const cumulative = prev + tokens;
      return [...acc, { tokens, cumulative }];
    }, []);
  }, [messages]);

  const totalTokens = tokenStats.length > 0 ? tokenStats[tokenStats.length - 1].cumulative : 0;

  if (messages.length === 0) {
    return <p className="text-xs text-muted-foreground">No messages in turn snapshot.</p>;
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <Badge variant="outline">Messages: {messages.length}</Badge>
        <Badge variant="outline">~{formatTokens(totalTokens)} tokens (est.)</Badge>
      </div>
      <ScrollArea className="h-[400px]">
        <div className="space-y-1 pr-3">
          {messages.map((msg, idx) => {
            const role = typeof msg.role === "string" ? msg.role : undefined;
            const source = typeof msg.source === "string" ? msg.source : undefined;
            const content = typeof msg.content === "string" ? msg.content : "";
            const badge = sourceBadgeFor(source, role);
            const isLarkUser = sessionChannel === "lark" && role === "user" && source !== "proactive_context";
            const stats = tokenStats[idx];

            return (
              <details
                key={`turn-msg-${idx}`}
                className="rounded-md border border-border/60 bg-muted/10"
              >
                <summary className="flex cursor-pointer items-center gap-2 px-3 py-2 text-xs">
                  <Badge
                    variant={badge.variant}
                    className={cn(
                      "text-[10px]",
                      source === "proactive_context" && "bg-purple-100 text-purple-800 border-purple-200",
                    )}
                  >
                    {badge.label}
                  </Badge>
                  {isLarkUser && (
                    <Badge variant="outline" className="text-[10px] border-blue-200 text-blue-700">
                      Lark
                    </Badge>
                  )}
                  {role && <span className="text-muted-foreground">[{role}]</span>}
                  <span className="text-[10px] text-muted-foreground/70 whitespace-nowrap">
                    ~{formatTokens(stats.tokens)}t / {formatTokens(stats.cumulative)} cum
                  </span>
                  <span className="truncate text-foreground/80">
                    {content.slice(0, 100)}{content.length > 100 ? "..." : ""}
                  </span>
                </summary>
                <div className="border-t border-border/40 px-3 py-2">
                  <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
                    {content || "(empty)"}
                  </pre>
                  {Array.isArray(msg.tool_calls) && msg.tool_calls.length > 0 && (
                    <div className="mt-2">
                      <JsonNode label="tool_calls" value={msg.tool_calls as unknown[]} depth={0} />
                    </div>
                  )}
                  {Array.isArray(msg.tool_results) && msg.tool_results.length > 0 && (
                    <div className="mt-2">
                      <JsonNode label="tool_results" value={msg.tool_results as unknown[]} depth={0} />
                    </div>
                  )}
                </div>
              </details>
            );
          })}
        </div>
      </ScrollArea>
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
  const [llmDetailCache, setLlmDetailCache] = useState<LLMDetailCache>(new Map());
  const [memoryEntries, setMemoryEntries] = useState<MemoryEntry[]>([]);
  const [memoryLoading, setMemoryLoading] = useState(false);
  const [memoryError, setMemoryError] = useState<string | null>(null);

  const { currentSessionId, sessionHistory } = useSessionStore();
  const { data: sessionsData } = useSessions();

  const larkSessions = useMemo(() => {
    if (!sessionsData?.sessions) return [];
    return sessionsData.sessions
      .filter((s) => s.id.startsWith("lark-"))
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
  }, [sessionsData]);

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

  const fetchLLMDetail = useCallback(
    (targetLogId: string) => {
      setLlmDetailCache((prev) => {
        if (prev.has(targetLogId)) return prev;
        const next = new Map(prev);
        next.set(targetLogId, "loading");
        return next;
      });
      apiClient
        .getLogTrace(targetLogId)
        .then((trace) => {
          setLlmDetailCache((prev) => {
            const next = new Map(prev);
            next.set(targetLogId, trace);
            return next;
          });
        })
        .catch(() => {
          setLlmDetailCache((prev) => {
            const next = new Map(prev);
            next.set(targetLogId, "error");
            return next;
          });
        });
    },
    [],
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

  // Derive channel from session metadata or session ID prefix heuristic.
  const sessionChannel = useMemo(() => {
    const meta = sessionSnapshot?.session as Record<string, unknown> | undefined;
    const metadata = meta?.metadata as Record<string, string> | undefined;
    if (metadata?.channel) return metadata.channel;
    if (sessionId.startsWith("lark-")) return "lark";
    if (sessionId.startsWith("wechat-")) return "wechat";
    return null;
  }, [sessionSnapshot, sessionId]);

  // Extract proactive context messages from the turn snapshot.
  const proactiveMessages = useMemo(() => {
    if (!turnSnapshot || typeof turnSnapshot !== "object") return [];
    const messages = (turnSnapshot as Record<string, unknown>).messages;
    if (!Array.isArray(messages)) return [];
    return messages.filter(
      (msg: Record<string, unknown>) => msg.source === "proactive_context",
    ) as Array<Record<string, unknown>>;
  }, [turnSnapshot]);

  // Extract proactive.context.refresh events from the SSE stream.
  const memoryRefreshEvents = useMemo(() => {
    return events
      .filter((e) => e.eventType === "proactive.context.refresh")
      .map((e) => ({
        id: e.id,
        receivedAt: e.receivedAt,
        payload: e.parsed as Record<string, unknown> | null,
      }));
  }, [events]);

  const loadMemoryEntries = useCallback(async () => {
    if (!sessionId) return;
    setMemoryLoading(true);
    setMemoryError(null);
    try {
      const entries = await apiClient.getMemoryEntries(sessionId);
      setMemoryEntries(entries ?? []);
    } catch (err) {
      setMemoryError(err instanceof Error ? err.message : "Failed to load memories.");
    } finally {
      setMemoryLoading(false);
    }
  }, [sessionId]);

  const runTree = useMemo((): RunTree => {
    const nodeMap = new Map<string, RunNode>();
    let totalDurationMs: number | null = null;
    let stageCount = 0;
    let toolCount = 0;
    let llmCount = 0;

    const FALLBACK_RUN = "__root__";

    function getOrCreateNode(envelope: Record<string, unknown>): RunNode {
      const runId = typeof envelope.run_id === "string" && envelope.run_id.trim()
        ? envelope.run_id.trim()
        : FALLBACK_RUN;
      let node = nodeMap.get(runId);
      if (!node) {
        node = {
          runId,
          parentRunId: typeof envelope.parent_run_id === "string" && envelope.parent_run_id.trim()
            ? envelope.parent_run_id.trim()
            : null,
          correlationId: typeof envelope.correlation_id === "string" && envelope.correlation_id.trim()
            ? envelope.correlation_id.trim()
            : null,
          causationId: typeof envelope.causation_id === "string" && envelope.causation_id.trim()
            ? envelope.causation_id.trim()
            : null,
          agentLevel: typeof envelope.agent_level === "string" ? envelope.agent_level : "core",
          children: [],
          stages: [],
          tools: [],
          llmCalls: [],
          totalDurationMs: null,
          firstSeq: null,
        };
        nodeMap.set(runId, node);
      }
      // Update metadata if not yet set
      if (!node.parentRunId && typeof envelope.parent_run_id === "string" && envelope.parent_run_id.trim()) {
        node.parentRunId = envelope.parent_run_id.trim();
      }
      if (!node.correlationId && typeof envelope.correlation_id === "string" && envelope.correlation_id.trim()) {
        node.correlationId = envelope.correlation_id.trim();
      }
      if (!node.causationId && typeof envelope.causation_id === "string" && envelope.causation_id.trim()) {
        node.causationId = envelope.causation_id.trim();
      }
      const seq = typeof envelope.seq === "number" ? envelope.seq : null;
      if (seq !== null && (node.firstSeq === null || seq < node.firstSeq)) {
        node.firstSeq = seq;
      }
      return node;
    }

    for (const entry of events) {
      if (!entry.parsed || typeof entry.parsed !== "object") {
        continue;
      }
      const envelope = entry.parsed as Record<string, unknown>;
      const payload = envelope.payload;
      if (!payload || typeof payload !== "object") {
        continue;
      }
      const data = payload as Record<string, unknown>;
      const node = getOrCreateNode(envelope);
      const seq = typeof envelope.seq === "number" ? envelope.seq : undefined;

      // Stages
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
          node.stages.push({ id: entry.id, label, durationMs: duration });
          stageCount++;
        }
      }

      // LLM calls
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
          const logId = extractLogId(envelope) ?? deriveLogIdFromRequestId(requestId);
          node.llmCalls.push({
            id: entry.id,
            label: iteration,
            durationMs: duration,
            hint: hintParts || undefined,
            seq,
            logId: logId ?? undefined,
            requestId: requestId || undefined,
            model: model || undefined,
          });
          llmCount++;
        }
      }

      // Tools
      if (entry.eventType === "workflow.tool.completed") {
        const duration = Number(data.duration);
        if (Number.isFinite(duration) && duration > 0) {
          const toolName =
            (data.tool_name as string | undefined) ||
            (data.tool as string | undefined) ||
            "tool";
          const callId = typeof data.call_id === "string" ? data.call_id : undefined;
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
          node.tools.push({
            id: entry.id,
            label: toolName,
            durationMs: duration,
            hint: hintParts || undefined,
            seq,
            callId,
          });
          toolCount++;
        }
      }

      // Total duration from final result
      if (entry.eventType === "workflow.result.final") {
        const duration = Number(data.duration);
        if (Number.isFinite(duration) && duration > 0) {
          node.totalDurationMs = duration;
          if (totalDurationMs === null) {
            totalDurationMs = duration;
          }
        }
      }
    }

    // Link children to parents
    const bySeq = (a: { firstSeq: number | null }, b: { firstSeq: number | null }) => {
      if (a.firstSeq === null && b.firstSeq === null) return 0;
      if (a.firstSeq === null) return 1;
      if (b.firstSeq === null) return -1;
      return a.firstSeq - b.firstSeq;
    };
    const roots: RunNode[] = [];
    for (const node of nodeMap.values()) {
      // Sort tools and LLM calls by seq (stages keep insertion order from event loop)
      const byEntrySeq = (a: ToolTimingEntry | LLMTimingEntry, b: ToolTimingEntry | LLMTimingEntry) => {
        if (a.seq === undefined && b.seq === undefined) return 0;
        if (a.seq === undefined) return 1;
        if (b.seq === undefined) return -1;
        return a.seq - b.seq;
      };
      node.tools.sort(byEntrySeq);
      node.llmCalls.sort(byEntrySeq);

      if (node.parentRunId && nodeMap.has(node.parentRunId)) {
        nodeMap.get(node.parentRunId)!.children.push(node);
      } else {
        roots.push(node);
      }
    }
    // Sort children and roots by seq
    for (const node of nodeMap.values()) {
      node.children.sort(bySeq);
    }
    roots.sort(bySeq);

    // Prune empty leaf nodes (no stages, tools, llmCalls, or children with content)
    function hasContent(node: RunNode): boolean {
      return (
        node.stages.length > 0 ||
        node.tools.length > 0 ||
        node.llmCalls.length > 0 ||
        node.totalDurationMs !== null ||
        node.children.some(hasContent)
      );
    }
    function pruneEmpty(node: RunNode): RunNode {
      return { ...node, children: node.children.filter(hasContent).map(pruneEmpty) };
    }
    const prunedRoots = roots.filter(hasContent).map(pruneEmpty);

    return { roots: prunedRoots, totalDurationMs, stageCount, toolCount, llmCount };
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
                {sessionChannel && (
                  <Badge variant={sessionChannel === "lark" ? "default" : "secondary"}>
                    Channel: {sessionChannel}
                  </Badge>
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

            {larkSessions.length > 0 && (
              <div className="mt-3 flex items-center gap-2">
                <p className="text-xs font-semibold text-muted-foreground whitespace-nowrap">Lark Sessions</p>
                <select
                  value={larkSessions.some((s) => s.id === sessionIdInput) ? sessionIdInput : ""}
                  onChange={(event) => {
                    if (event.target.value) {
                      setSessionIdInput(event.target.value);
                    }
                  }}
                  className="h-8 min-w-[260px] max-w-md rounded-md border border-input bg-background px-2 text-xs shadow-sm"
                >
                  <option value="">-- Pick a Lark session --</option>
                  {larkSessions.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.id}{s.title ? ` — ${s.title}` : ""} ({new Date(s.updated_at).toLocaleString()})
                    </option>
                  ))}
                </select>
                <Badge variant="outline" className="text-[10px]">
                  {larkSessions.length} Lark
                </Badge>
              </div>
            )}

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
                  <CardTitle className="text-base">Memory</CardTitle>
                  <CardDescription>
                    Proactive memory: recalled context injected into the LLM, refresh events, and stored entries.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  {/* Section A: Recalled memories from turn snapshot */}
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold text-muted-foreground">
                      Recalled memories (turn snapshot)
                    </p>
                    {proactiveMessages.length > 0 ? (
                      <div className="space-y-1">
                        <Badge variant="outline">{proactiveMessages.length} memories recalled</Badge>
                        {proactiveMessages.map((msg, idx) => {
                          const content = typeof msg.content === "string" ? msg.content : JSON.stringify(msg.content);
                          return (
                            <details
                              key={`proactive-msg-${idx}`}
                              className="rounded-md border border-border/60 bg-muted/10"
                            >
                              <summary className="cursor-pointer px-3 py-2 text-xs font-medium text-foreground">
                                Memory #{idx + 1}
                                <span className="ml-2 text-muted-foreground">
                                  {content.slice(0, 80)}{content.length > 80 ? "..." : ""}
                                </span>
                              </summary>
                              <div className="border-t border-border/40 px-3 py-2">
                                <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
                                  {content}
                                </pre>
                              </div>
                            </details>
                          );
                        })}
                      </div>
                    ) : (
                      <p className="text-xs text-muted-foreground">No proactive context in current turn.</p>
                    )}
                  </div>

                  {/* Section B: Memory refresh events from SSE */}
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold text-muted-foreground">
                      Memory refresh events (SSE stream)
                    </p>
                    {memoryRefreshEvents.length > 0 ? (
                      <div className="space-y-1">
                        {memoryRefreshEvents.map((evt) => {
                          const payload = evt.payload?.payload as Record<string, unknown> | undefined;
                          return (
                            <div
                              key={evt.id}
                              className="flex items-center gap-2 rounded border border-border/40 bg-muted/20 px-2 py-1 text-xs"
                            >
                              <Badge variant="secondary" className="text-[10px]">refresh</Badge>
                              <span>iteration: {String(payload?.iteration ?? "—")}</span>
                              <span>memories: {String(payload?.memories_injected ?? "—")}</span>
                              <span className="text-muted-foreground">
                                {formatTimestamp(evt.receivedAt)}
                              </span>
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <p className="text-xs text-muted-foreground">No memory refresh events captured.</p>
                    )}
                  </div>

                  {/* Section C: Memory store entries from API */}
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold text-muted-foreground">
                      Memory store
                    </p>
                    <div className="flex items-center gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void loadMemoryEntries()}
                        disabled={!sessionId || memoryLoading}
                      >
                        Load memories
                      </Button>
                      {memoryLoading && <Badge variant="warning">Loading</Badge>}
                      {memoryEntries.length > 0 && (
                        <Badge variant="outline">{memoryEntries.length} entries</Badge>
                      )}
                    </div>
                    {memoryError && (
                      <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                        {memoryError}
                      </div>
                    )}
                    {memoryEntries.length > 0 && (
                      <ScrollArea className="h-[300px]">
                        <div className="space-y-2 pr-3">
                          {memoryEntries.map((entry) => (
                            <details
                              key={entry.key}
                              className="rounded-md border border-border/60 bg-muted/10"
                            >
                              <summary className="flex cursor-pointer items-center justify-between gap-2 px-3 py-2 text-xs">
                                <div className="flex min-w-0 items-center gap-2">
                                  <span className="font-medium text-foreground">
                                    {entry.content.slice(0, 60)}{entry.content.length > 60 ? "..." : ""}
                                  </span>
                                  {entry.slots?.type && (
                                    <Badge variant="secondary" className="text-[10px]">
                                      {entry.slots.type}
                                    </Badge>
                                  )}
                                </div>
                                <span className="whitespace-nowrap text-muted-foreground">
                                  {entry.created_at ? formatTimestamp(entry.created_at) : "—"}
                                </span>
                              </summary>
                              <div className="space-y-2 border-t border-border/40 px-3 py-2">
                                <pre className="whitespace-pre-wrap break-words text-xs text-foreground/80">
                                  {entry.content}
                                </pre>
                                {entry.keywords && entry.keywords.length > 0 && (
                                  <div className="flex flex-wrap gap-1">
                                    {entry.keywords.map((kw) => (
                                      <Badge key={kw} variant="outline" className="text-[10px]">
                                        {kw}
                                      </Badge>
                                    ))}
                                  </div>
                                )}
                                {entry.slots && Object.keys(entry.slots).length > 0 && (
                                  <div className="text-[11px] text-muted-foreground">
                                    Slots: {Object.entries(entry.slots).map(([k, v]) => `${k}=${v}`).join(", ")}
                                  </div>
                                )}
                              </div>
                            </details>
                          ))}
                        </div>
                      </ScrollArea>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Turn messages</CardTitle>
                  <CardDescription>
                    Messages from the latest turn snapshot with source badges.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {turnSnapshot ? (
                    <TurnMessagesViewer
                      turnSnapshot={turnSnapshot}
                      sessionChannel={sessionChannel}
                    />
                  ) : (
                    <div className="rounded-xl border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">
                      No turn snapshot available.
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-base">Timing breakdown</CardTitle>
                  <CardDescription>
                    Hierarchical breakdown of workflow stages, tools, and LLM calls grouped by run.
                    Click LLM calls to expand request/response details.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <RunTreeView
                    tree={runTree}
                    cache={llmDetailCache}
                    onFetchLLM={fetchLLMDetail}
                  />
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
