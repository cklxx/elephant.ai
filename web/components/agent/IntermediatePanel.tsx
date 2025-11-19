"use client";

import { type ReactNode, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import {
  AnyAgentEvent,
  AssistantMessageEvent,
  AttachmentPayload,
  ThinkCompleteEvent,
} from "@/lib/types";
import { PanelRightOpen, X } from "lucide-react";
import { ToolOutputCard } from "./ToolOutputCard";

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
  event: AssistantMessageEvent | ThinkCompleteEvent,
  iteration: number,
) => {
  const taskIdentifier =
    event.task_id ??
    (event.parent_task_id
      ? `${event.parent_task_id}:${event.subtask_index ?? "0"}`
      : event.session_id);
  return `${taskIdentifier}:${iteration}`;
};

export function IntermediatePanel({ events }: IntermediatePanelProps) {
  const [isPanelOpen, setIsPanelOpen] = useState(false);

  interface AggregatedToolCall {
    callId: string;
    toolName: string;
    timestamp: string;
    result?: string;
    error?: string;
    duration?: number;
    parameters?: Record<string, unknown>;
    metadata?: Record<string, unknown>;
    attachments?: Record<string, AttachmentPayload>;
    isComplete: boolean;
    status: "running" | "completed" | "failed";
  }

  // Aggregate tool calls and model outputs
  const { toolCalls, thinkStreamItems } = useMemo(() => {
    const toolCallsMap = new Map<string, AggregatedToolCall>();
    const thinkStreams = new Map<string, ThinkPreviewItem>();

    events.forEach((event) => {
      if (event.event_type === "tool_call_start") {
        // Initialize with start event data
        toolCallsMap.set(event.call_id, {
          callId: event.call_id,
          toolName: event.tool_name,
          timestamp: event.timestamp,
          parameters: event.arguments as Record<string, unknown>,
          isComplete: false,
          status: "running",
        });
      } else if (event.event_type === "tool_call_complete") {
        // Update with complete event data (including metadata)
        const toolCall = toolCallsMap.get(event.call_id);
        if (toolCall) {
          toolCall.result = event.result;
          toolCall.error = event.error;
          toolCall.duration = event.duration;
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
      } else if (event.event_type === "assistant_message") {
        const assistantEvent = event as AssistantMessageEvent;
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
      } else if (event.event_type === "think_complete") {
        const thinkEvent = event as ThinkCompleteEvent;
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
      toolCalls: Array.from(toolCallsMap.values()),
      thinkStreamItems: Array.from(thinkStreams.values())
        .filter((item) => item.content.trim().length > 0)
        .sort(
          (a, b) =>
            new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
        ),
    };
  }, [events]);

  const sortedToolCalls = useMemo(
    () =>
      [...toolCalls].sort(
        (a, b) =>
          new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
      ),
    [toolCalls],
  );

  const runningTools = useMemo(
    () => toolCalls.filter((call) => call.status === "running"),
    [toolCalls],
  );
  const hasRunningTool = runningTools.length > 0;
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

  const thinkPreviewItems = thinkStreamItems;
  const latestThinkPreviewItem =
    thinkPreviewItems[thinkPreviewItems.length - 1];

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
      return names.join(" · ");
    }
    return `${names.slice(0, 2).join(" · ")} +${names.length - 2}`;
  }, [toolCalls]);

  // Don't show panel if there are no tool calls
  if (toolCalls.length === 0) {
    return null;
  }

  const openDetails = () => setIsPanelOpen(true);

  return (
    <div className="space-y-2 pb-1 pl-1">
      {/*{latestThinkPreviewItem && (
        <ThinkStreamList items={[latestThinkPreviewItem]} />
      )}*/}
      <button
        type="button"
        onClick={openDetails}
        className="group inline-flex max-w-full items-start gap-3 overflow-hidden rounded-2xl bg-background/70 px-3 py-2 text-left text-xs font-medium text-foreground shadow-sm transition hover:bg-background focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background"
        title={
          runningSummaryFull.length > 0
            ? `Running: ${runningSummaryFull}`
            : undefined
        }
      >
        <span className="max-w-full truncate text-[11px] text-muted-foreground">
          {runningSummary || toolSummary}
        </span>
        {hasRunningTool && (
          <span className="flex items-center gap-1 text-[11px] font-semibold text-primary transition-colors group-hover:text-primary/90">
            <span
              className="h-2 w-2 animate-pulse rounded-full bg-primary"
              aria-hidden="true"
            />
            running
          </span>
        )}
      </button>

      <ToolCallDetailsPanel
        open={isPanelOpen}
        onClose={() => setIsPanelOpen(false)}
      >
        <div className="space-y-4">
          {thinkPreviewItems.length > 0 && (
            <ThinkStreamList items={thinkPreviewItems} />
          )}
          {sortedToolCalls.map((item) => (
            <ToolOutputCard
              key={item.callId}
              toolName={item.toolName}
              parameters={item.parameters}
              result={item.result}
              error={item.error}
              duration={item.duration}
              timestamp={item.timestamp}
              callId={item.callId}
              metadata={item.metadata}
              attachments={item.attachments}
              status={item.status}
            />
          ))}
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
  const [isMounted, setIsMounted] = useState(false);

  useEffect(() => {
    setIsMounted(true);
  }, []);

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

  if (!isMounted || !open) {
    return null;
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex">
      <div
        className="flex-1 bg-black/40 backdrop-blur-sm transition-opacity"
        onClick={onClose}
        aria-hidden="true"
      />
      <aside
        className="relative flex h-full w-full max-w-3xl flex-col bg-background shadow-2xl transition-transform duration-300 ease-out"
        aria-label="Tool call activity"
      >
        <header className="flex items-center justify-end border-b border-border px-4 py-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background"
            aria-label="Close tool call details"
          >
            <X className="h-4 w-4" />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-2">
          {children}
        </div>
      </aside>
    </div>,
    document.body,
  );
}

function ThinkStreamList({ items }: { items: ThinkPreviewItem[] }) {
  return (
    <section className="rounded-2xl bg-muted/40 px-4 py-3">
      <div className="space-y-3">
        {items.map((item) => (
          <div key={item.id} className="space-y-1">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] font-semibold uppercase tracking-[0.3em] text-muted-foreground/80">
              <span>LLM THINK</span>
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
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
              {item.content}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}
