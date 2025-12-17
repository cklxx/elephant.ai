"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { createPortal } from "react-dom";
import {
  AnyAgentEvent,
  WorkflowNodeOutputDeltaEvent,
  AttachmentPayload,
  WorkflowNodeOutputSummaryEvent,
  WorkflowToolCompletedEvent,
  WorkflowToolStartedEvent,
  eventMatches,
} from "@/lib/types";
import { X } from "lucide-react";
import { ToolOutputCard } from "./ToolOutputCard";
import { LazyMarkdownRenderer } from "./LazyMarkdownRenderer";
import { humanizeToolName, formatDuration } from "@/lib/utils";
import { MagicBlackHole } from "../effects/MagicBlackHole";
import { isDebugModeEnabled } from "@/lib/debugMode";
import { userFacingToolSummary } from "@/lib/toolPresentation";
import { useElapsedDurationMs } from "@/hooks/useElapsedDurationMs";

interface IntermediatePanelProps {
  events: AnyAgentEvent[];
}

interface ThinkPreviewItem {
  id: string;
  iteration: number;
  timestamp: string;
  content: string;
  isFinal: boolean;
}

const getThinkStreamKey = (
  event: WorkflowNodeOutputDeltaEvent | WorkflowNodeOutputSummaryEvent,
  iteration: number,
) => {
  const taskIdentifier =
    event.task_id ??
    (event.parent_task_id
      ? `${event.parent_task_id}:${event.subtask_index ?? "0"}`
      : event.session_id);
  return `${taskIdentifier}:${iteration}`;
};

const isWorkflowToolStartedEvent = (
  event: AnyAgentEvent,
): event is WorkflowToolStartedEvent =>
  eventMatches(event, "workflow.tool.started", "workflow.tool.started");

const isWorkflowToolCompletedEvent = (
  event: AnyAgentEvent,
): event is WorkflowToolCompletedEvent =>
  eventMatches(event, "workflow.tool.completed", "workflow.tool.completed");

const isWorkflowNodeOutputDeltaEvent = (
  event: AnyAgentEvent,
): event is WorkflowNodeOutputDeltaEvent =>
  eventMatches(
    event,
    "workflow.node.output.delta",
    "workflow.node.output.delta",
    "workflow.node.output.delta",
  );

const isWorkflowNodeOutputSummaryEvent = (
  event: AnyAgentEvent,
): event is WorkflowNodeOutputSummaryEvent =>
  eventMatches(event, "workflow.node.output.summary", "workflow.node.output.summary");

export function IntermediatePanel({ events }: IntermediatePanelProps) {
  const [isPanelOpen, setIsPanelOpen] = useState(false);
  const debugMode = useMemo(() => isDebugModeEnabled(), []);

  interface AggregatedToolCall {
    callId: string;
    toolName: string;
    timestamp: string;
    completedTimestamp?: string;
    result?: string;
    error?: string;
    duration?: number;
    parameters?: Record<string, unknown>;
    metadata?: Record<string, unknown>;
    attachments?: Record<string, AttachmentPayload>;
    isComplete: boolean;
    status: "running" | "completed" | "failed";
  }

  const summarizeToolHint = useCallback((call: AggregatedToolCall) => {
    const parameters = call.parameters ?? {};
    const metadata = call.metadata ?? {};
    const preferredKeys = [
      "url",
      "path",
      "file_path",
      "filePath",
      "command",
      "input",
      "query",
      "prompt",
      "message",
    ];

    const candidate = preferredKeys
      .map((key) => parameters?.[key])
      .find((value) => typeof value === "string" && value.trim().length > 0);

    if (typeof candidate === "string") {
      return candidate;
    }

    const browserMetadata =
      metadata && typeof metadata === "object" && metadata.browser
        ? (metadata.browser as Record<string, unknown>)
        : undefined;
    const metadataUrl =
      typeof metadata.url === "string"
        ? metadata.url
        : typeof browserMetadata?.url === "string"
          ? browserMetadata.url
          : undefined;
    if (metadataUrl && typeof metadataUrl === "string") {
      return metadataUrl;
    }

    const result = call.error || call.result;
    if (typeof result === "string" && result.trim().length > 0) {
      const summary = userFacingToolSummary({
        toolName: call.toolName,
        result: call.result ?? null,
        error: call.error ?? null,
        metadata: (call.metadata as Record<string, any>) ?? null,
        attachments: (call.attachments as any) ?? null,
      });
      if (summary) {
        return summary.length > 120 ? `${summary.slice(0, 120)}…` : summary;
      }
      const trimmed = result.trim();
      return trimmed.length > 120 ? `${trimmed.slice(0, 120)}…` : trimmed;
    }

    return undefined;
  }, []);

  // Aggregate tool calls and model outputs
  const { toolCalls, thinkStreamItems } = useMemo(() => {
    const toolCallsMap = new Map<string, AggregatedToolCall>();
    const thinkStreams = new Map<string, ThinkPreviewItem>();

    events.forEach((event) => {
      if (isWorkflowToolStartedEvent(event)) {
        // Initialize with start event data
        toolCallsMap.set(event.call_id, {
          callId: event.call_id,
          toolName: event.tool_name,
          timestamp: event.timestamp,
          parameters: event.arguments as Record<string, unknown>,
          isComplete: false,
          status: "running",
        });
      } else if (isWorkflowToolCompletedEvent(event)) {
        // Update with complete event data (including metadata)
        const toolCall = toolCallsMap.get(event.call_id);
        if (toolCall) {
          toolCall.result = event.result;
          toolCall.error = event.error;
          toolCall.duration = event.duration;
          toolCall.completedTimestamp = event.timestamp;
          toolCall.metadata = event.metadata as Record<string, unknown>;
          toolCall.attachments = event.attachments as Record<
            string,
            AttachmentPayload
          >;
          toolCall.isComplete = true;
          toolCall.status =
            event.error && event.error.trim().length > 0
              ? "failed"
              : "completed";
        } else {
          // If no start event, create from complete event directly
          toolCallsMap.set(event.call_id, {
            callId: event.call_id,
            toolName: event.tool_name,
            timestamp: event.timestamp,
            completedTimestamp: event.timestamp,
            result: event.result,
            error: event.error,
            duration: event.duration,
            metadata: event.metadata as Record<string, unknown>,
            attachments: event.attachments as Record<string, AttachmentPayload>,
            isComplete: true,
            status:
              event.error && event.error.trim().length > 0
                ? "failed"
                : "completed",
          });
        }
      } else if (isWorkflowNodeOutputDeltaEvent(event)) {
        const assistantEvent = event;
        const iteration = assistantEvent.iteration;
        if (typeof iteration !== "number") {
          return;
        }
        const streamKey = getThinkStreamKey(assistantEvent, iteration);
        const existing = thinkStreams.get(streamKey) ?? {
          id: streamKey,
          iteration,
          timestamp: assistantEvent.created_at ?? assistantEvent.timestamp,
          content: "",
          isFinal: false,
        };
        const delta = assistantEvent.delta ?? "";
        if (delta.length > 0) {
          existing.content = `${existing.content}${delta}`;
        }
        existing.timestamp =
          assistantEvent.created_at ?? assistantEvent.timestamp;
        existing.isFinal = Boolean(assistantEvent.final);
        thinkStreams.set(streamKey, existing);
      } else if (isWorkflowNodeOutputSummaryEvent(event)) {
        const thinkEvent = event;
        const iteration = thinkEvent.iteration;
        if (typeof iteration !== "number") {
          return;
        }
        const streamKey = getThinkStreamKey(thinkEvent, iteration);
        const existing = thinkStreams.get(streamKey) ?? {
          id: streamKey,
          iteration,
          timestamp: thinkEvent.timestamp,
          content: "",
          isFinal: true,
        };
        if (!existing.content.trim().length && thinkEvent.content) {
          existing.content = thinkEvent.content;
        }
        existing.timestamp = thinkEvent.timestamp;
        existing.isFinal = true;
        thinkStreams.set(streamKey, existing);
      }
    });

    return {
      toolCalls: Array.from(toolCallsMap.values()).filter((call) => {
        if (debugMode) return true;
        return call.toolName.toLowerCase().trim() !== "think";
      }),
      thinkStreamItems: Array.from(thinkStreams.values())
        .filter((item) => item.content.trim().length > 0)
        .sort(
          (a, b) =>
            new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
        ),
    };
  }, [events, debugMode]);

  const runningTools = useMemo(
    () => toolCalls.filter((call) => call.status === "running"),
    [toolCalls],
  );
  const runningSummary = useMemo(() => {
    if (runningTools.length === 0) {
      return "";
    }
    const names = runningTools.map((call) => call.toolName);
    if (names.length === 1) {
      return names[0];
    }
    if (names.length === 2) {
      return names.join(" · ");
    }
    return `${names.slice(0, 2).join(" · ")} +${names.length - 2}`;
  }, [runningTools]);
  const runningSummaryFull = runningTools
    .map((call) => call.toolName)
    .join(", ");

  const toolSummary = useMemo(() => {
    if (toolCalls.length === 0) {
      return "";
    }
    const names = toolCalls
      .map((call) => call.toolName)
      .filter((name) => Boolean(name));
    if (names.length === 0) {
      return "";
    }
    if (names.length === 1) {
      return names[0];
    }
    if (names.length === 2) {
      return names.join(" -> ");
    }
    return `${names.slice(0, 2).join(" -> ")} +${names.length - 2}`;
  }, [toolCalls]);

  const latestRunning = runningTools[0];
  const latestCompleted = [...toolCalls]
    .filter((call) => call.status !== "running")
    .sort(
      (a, b) =>
        new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
    )[0];

  const headlineCall = latestRunning ?? latestCompleted;
  const headlineHint = useMemo(() => {
    if (!headlineCall) {
      return null;
    }
    return summarizeToolHint(headlineCall) ?? null;
  }, [headlineCall, summarizeToolHint]);

  const headlineText = useMemo(() => {
    if (!headlineCall) {
      return runningSummary || toolSummary || "Tool activity";
    }

    const humanizedName = humanizeToolName(headlineCall.toolName);
    if (!headlineHint) {
      return humanizedName;
    }
    return `${humanizedName} · ${headlineHint}`;
  }, [headlineCall, headlineHint, runningSummary, toolSummary]);

  const headlinePreview = useMemo(() => {
    if (!headlineCall) {
      return "";
    }
    const summary = userFacingToolSummary({
      toolName: headlineCall.toolName,
      result: headlineCall.result ?? null,
      error: headlineCall.error ?? null,
      metadata: (headlineCall.metadata as Record<string, any>) ?? null,
      attachments: (headlineCall.attachments as any) ?? null,
    });
    if (!summary) {
      return "";
    }
    if (headlineHint && summary.trim() === headlineHint.trim()) {
      return "";
    }
    return summary.length > 160 ? `${summary.slice(0, 160)}…` : summary;
  }, [headlineCall, headlineHint]);

  const headlineElapsedMs = useElapsedDurationMs({
    startTimestamp: headlineCall?.timestamp,
    running: headlineCall?.status === "running",
    tickMs: 250,
  });

  const headlineDurationLabel = useMemo(() => {
    if (!headlineCall) return null;
    if (headlineCall.status === "running") {
      return typeof headlineElapsedMs === "number"
        ? formatDuration(headlineElapsedMs)
        : null;
    }
    if (typeof headlineCall.duration === "number" && headlineCall.duration > 0) {
      return formatDuration(headlineCall.duration);
    }
    if (headlineCall.completedTimestamp) {
      const startMs = new Date(headlineCall.timestamp).getTime();
      const endMs = new Date(headlineCall.completedTimestamp).getTime();
      if (!Number.isNaN(startMs) && !Number.isNaN(endMs) && endMs >= startMs) {
        return formatDuration(endMs - startMs);
      }
    }
    return null;
  }, [headlineCall, headlineElapsedMs]);

  const completedCount = toolCalls.filter(
    (call) => call.status === "completed",
  ).length;
  const failedCount = toolCalls.filter((call) => call.status === "failed").length;

  const thinkPreviewItems = debugMode ? thinkStreamItems : [];
  const timelineItems = useMemo(() => {
    const thinkEntries = thinkPreviewItems.map((item) => ({
      kind: "think" as const,
      timestamp: item.timestamp,
      item,
    }));
    const toolEntries = toolCalls.map((item) => ({
      kind: "tool" as const,
      timestamp: item.timestamp,
      item,
    }));
    return [...thinkEntries, ...toolEntries].sort(
      (a, b) =>
        new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
    );
  }, [thinkPreviewItems, toolCalls]);

  // Don't show panel if there are no tool calls
  if (toolCalls.length === 0) {
    return null;
  }

  const openDetails = () => setIsPanelOpen(true);

  return (
    <div className="space-y-2 pb-1">
      {/*{latestThinkPreviewItem && (
        <ThinkStreamCard item={latestThinkPreviewItem} />
      )}*/}
      <button
        type="button"
        onClick={openDetails}
        className="group flex w-full max-w-[fit-content] items-center gap-3 rounded-md border border-border/40 bg-secondary/40 px-3 py-2 text-left text-sm font-medium text-foreground transition-colors hover:bg-secondary/60"
        title={
          runningSummaryFull.length > 0
            ? `Running: ${runningSummaryFull}`
            : headlineText
        }
      >
        {runningTools.length > 0 && (
          <div className="flex items-center justify-center text-muted-foreground/70">
            <MagicBlackHole size="sm" className="mr-1" />
          </div>
        )}

        <div className="min-w-0 flex flex-col gap-0.5">
          <span className="text-sm font-medium opacity-90 truncate max-w-[300px]">
            {headlineText}
          </span>
          {headlinePreview && headlinePreview !== headlineText && (
            <span className="text-xs text-muted-foreground/70 truncate max-w-[300px]">
              {headlinePreview}
            </span>
          )}
        </div>

        {(runningTools.length > 0 || completedCount > 0 || failedCount > 0) && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground/60 ml-2 border-l border-border/40 pl-3">
            {runningTools.length > 0 && <span>{runningTools.length} running</span>}
            {completedCount > 0 && <span>{completedCount} done</span>}
            {failedCount > 0 && <span className="text-red-500">{failedCount} failed</span>}
            {headlineDurationLabel ? (
              <span data-testid="intermediate-headline-duration">
                {headlineDurationLabel}
              </span>
            ) : null}
          </div>
        )}
      </button>

      <ToolCallDetailsPanel
        open={isPanelOpen}
        onClose={() => setIsPanelOpen(false)}
      >
        <div className="space-y-4">
          {timelineItems.map((entry) =>
            entry.kind === "think" ? (
              <ThinkStreamCard
                key={`think-${entry.item.id}`}
                item={entry.item}
              />
            ) : (
              <ToolOutputCard
                key={`tool-${entry.item.callId}`}
                toolName={entry.item.toolName}
                parameters={entry.item.parameters}
                result={entry.item.result}
                error={entry.item.error}
                duration={entry.item.duration}
                timestamp={entry.item.timestamp}
                callId={entry.item.callId}
                metadata={entry.item.metadata}
                attachments={entry.item.attachments}
                status={entry.item.status}
              />
            ),
          )}
        </div>
      </ToolCallDetailsPanel>
    </div>
  );
}

interface ToolCallDetailsPanelProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}

function ToolCallDetailsPanel({
  open,
  onClose,
  children,
}: ToolCallDetailsPanelProps) {
  useEffect(() => {
    if (!open) {
      return;
    }
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [open]);

  if (!open || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex" role="dialog" aria-modal="true">
      <div
        className="flex-1 bg-black/30 transition-opacity"
        onClick={onClose}
        aria-hidden="true"
      />
      <aside
        className="relative flex h-full w-full max-w-3xl flex-col border-l border-border/60 bg-background transition-transform duration-300 ease-out"
        aria-label="Tool call activity"
      >
        <header className="flex items-center justify-end border-b border-border/60 px-4 py-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background"
            aria-label="Close tool call details"
          >
            <X className="h-4 w-4" />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto space-y-2 px-5 py-4">
          {children}
        </div>
      </aside>
    </div>,
    document.body,
  );
}

function ThinkStreamCard({ item }: { item: ThinkPreviewItem }) {
  return (
    <section className="rounded-xl border border-border/40 bg-muted/20 px-4 py-3">
      <div className="space-y-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] font-semibold text-muted-foreground/80">
          <span>LLM think</span>
          <span className="font-mono tracking-normal text-[10px] text-muted-foreground/70">
            iter {item.iteration}
          </span>
          {!item.isFinal && (
            <span className="flex items-center gap-1 text-primary">
              <span
                className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary"
                aria-hidden="true"
              />
              streaming
            </span>
          )}
        </div>
        <LazyMarkdownRenderer
          content={item.content}
          containerClassName="markdown-body text-sm"
          className="prose prose-sm max-w-none text-muted-foreground"
        />
      </div>
    </section>
  );
}
