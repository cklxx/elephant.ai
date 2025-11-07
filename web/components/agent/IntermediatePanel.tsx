"use client";

import { type ReactNode, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { AnyAgentEvent, AttachmentPayload } from "@/lib/types";
import { PanelRightOpen, X } from "lucide-react";
import { ToolOutputCard } from "./ToolOutputCard";
import { TaskCompleteCard } from "./TaskCompleteCard";

interface IntermediatePanelProps {
  events: AnyAgentEvent[];
}

interface ModelOutput {
  iteration: number;
  content: string;
  timestamp: string;
}

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
  }

  // Aggregate tool calls and model outputs
  const { toolCalls, modelOutputs } = useMemo(() => {
    const toolCallsMap = new Map<string, AggregatedToolCall>();
    const outputList: ModelOutput[] = [];

    events.forEach((event) => {
      if (event.event_type === "tool_call_start") {
        // Initialize with start event data
        toolCallsMap.set(event.call_id, {
          callId: event.call_id,
          toolName: event.tool_name,
          timestamp: event.timestamp,
          parameters: event.arguments as Record<string, unknown>,
          isComplete: false,
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
          });
        }
      } else if (event.event_type === "think_complete") {
        outputList.push({
          iteration: event.iteration,
          content: event.content,
          timestamp: event.timestamp,
        });
      }
    });

    return {
      toolCalls: Array.from(toolCallsMap.values()),
      modelOutputs: outputList,
    };
  }, [events]);

  const timelineItems = useMemo(
    () =>
      [...modelOutputs, ...toolCalls].sort(
        (a, b) =>
          new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
      ),
    [modelOutputs, toolCalls],
  );

  const hasRunningTool = useMemo(
    () => toolCalls.some((call) => !call.isComplete),
    [toolCalls],
  );

  // Don't show panel if there are no tool calls or model outputs
  if (timelineItems.length === 0) {
    return null;
  }

  const openDetails = () => setIsPanelOpen(true);

  return (
    <div className="pb-2 pl-2">
      <button
        type="button"
        onClick={openDetails}
        className="group flex w-full items-center gap-2 overflow-hidden bg-muted/30 px-4 py-2 text-left text-sm text-foreground transition hover:bg-muted/50 focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background"
      >
        {hasRunningTool && (
          <span className="flex items-center gap-1 text-xs font-medium text-primary transition-colors group-hover:text-primary/90">
            <span
              className="h-2 w-2 animate-pulse rounded-full bg-primary"
              aria-hidden="true"
            />
            loading
          </span>
        )}
        <span className="font-mono text-sm text-foreground">
          {toolCalls.length.toLocaleString()}
        </span>
        <span>tool{toolCalls.length !== 1 ? "s" : ""}</span>
        <PanelRightOpen className="ml-auto h-4 w-4 text-muted-foreground transition group-hover:text-primary" />
      </button>

      <ToolCallDetailsPanel
        open={isPanelOpen}
        onClose={() => setIsPanelOpen(false)}
        title={`${toolCalls.length} tool call${
          toolCalls.length !== 1 ? "s" : ""
        }`}
      >
        {timelineItems.map((item) => {
          if ("iteration" in item) {
            return (
              <ModelOutputItem
                key={`output-${item.iteration}-${item.timestamp}`}
                modelOutput={item}
              />
            );
          }
          return (
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
            />
          );
        })}
      </ToolCallDetailsPanel>
    </div>
  );
}

function ModelOutputItem({ modelOutput }: { modelOutput: ModelOutput }) {
  const [showContent, setShowContent] = useState(true);
  if (!modelOutput.content) return null;

  // Convert ModelOutput to TaskCompleteEvent format for consistent rendering
  const mockEvent = {
    event_type: "task_complete" as const,
    timestamp: modelOutput.timestamp,
    agent_level: "core" as const,
    session_id: "",
    task_id: "",
    final_answer: modelOutput.content,
    total_iterations: modelOutput.iteration,
    total_tokens: 0,
    stop_reason: "",
    duration: 0,
  };

  return (
    <div className="rounded-xl bg-muted/40 px-3 py-2">
      <button
        type="button"
        onClick={() => setShowContent(!showContent)}
        className="flex w-full items-start gap-2 text-left text-sm text-muted-foreground focus:outline-none focus-visible:text-foreground"
      >
        <span className="font-semibold text-foreground">Model Output</span>
        <span className="text-xs text-muted-foreground">
          iteration {modelOutput.iteration}
        </span>
      </button>

      {showContent && <TaskCompleteCard event={mockEvent} />}
    </div>
  );
}

interface ToolCallDetailsPanelProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}

function ToolCallDetailsPanel({
  open,
  onClose,
  title,
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
      <aside className="relative flex h-full w-full max-w-3xl flex-col bg-background shadow-2xl transition-transform duration-300 ease-out">
        <header className="flex items-center justify-between border-b border-border px-6 py-5">
          <div>
            <p className="text-[11px] uppercase tracking-[0.24em] text-muted-foreground">
              Tool Calls
            </p>
            <h2 className="text-lg font-semibold text-foreground">{title}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background"
            aria-label="Close tool call details"
          >
            <X className="h-4 w-4" />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto px-6 py-6 space-y-3">
          {children}
        </div>
      </aside>
    </div>,
    document.body,
  );
}
