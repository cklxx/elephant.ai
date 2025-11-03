"use client";

import { useMemo, useState } from "react";
import { AnyAgentEvent } from "@/lib/types";
import { ChevronDown, ChevronUp } from "lucide-react";
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
  const [isExpanded, setIsExpanded] = useState(false);

  interface AggregatedToolCall {
    callId: string;
    toolName: string;
    timestamp: string;
    result?: string;
    error?: string;
    duration?: number;
    parameters?: Record<string, unknown>;
    metadata?: Record<string, unknown>;
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
        });
      } else if (event.event_type === "tool_call_complete") {
        // Update with complete event data (including metadata)
        const toolCall = toolCallsMap.get(event.call_id);
        if (toolCall) {
          toolCall.result = event.result;
          toolCall.error = event.error;
          toolCall.duration = event.duration;
          toolCall.metadata = event.metadata as Record<string, unknown>;
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

  // Get the last tool call for collapsed state
  const lastToolCall = toolCalls[toolCalls.length - 1];

  // Don't show panel if there are no tool calls or model outputs
  if (toolCalls.length === 0 && modelOutputs.length === 0) {
    return null;
  }

  return (
    <div className="px-6 py-4">
      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className="flex w-full items-center justify-between gap-3 text-left transition-colors hover:text-foreground"
      >
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-slate-700">
            Tool Calls
          </span>
          <span className="text-xs text-slate-500">
            {toolCalls.length} tool{toolCalls.length !== 1 ? "s" : ""}
          </span>
        </div>
        {isExpanded ? (
          <ChevronUp className="h-4 w-4 text-slate-400" />
        ) : (
          <ChevronDown className="h-4 w-4 text-slate-400" />
        )}
      </button>

      {/* Collapsed: Show only last tool call */}
      {!isExpanded && lastToolCall && (
        <div className="mt-3">
          <ToolOutputCard
            toolName={lastToolCall.toolName}
            parameters={lastToolCall.parameters}
            result={lastToolCall.result}
            error={lastToolCall.error}
            duration={lastToolCall.duration}
            timestamp={lastToolCall.timestamp}
            callId={lastToolCall.callId}
            metadata={lastToolCall.metadata}
          />
        </div>
      )}

      {/* Expanded: Show all model outputs and tool calls */}
      {isExpanded && (
        <div className="mt-4 space-y-4">
          {/* Show model outputs and tool calls in chronological order */}
          {[...modelOutputs, ...toolCalls]
            .sort(
              (a, b) =>
                new Date(a.timestamp).getTime() -
                new Date(b.timestamp).getTime(),
            )
            .map((item) => {
              if ("iteration" in item) {
                // It's a model output
                return (
                  <ModelOutputItem
                    key={`output-${item.iteration}-${item.timestamp}`}
                    modelOutput={item}
                  />
                );
              } else {
                // It's a tool call
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
                  />
                );
              }
            })}
        </div>
      )}
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
    <div className="border-l-2 border-blue-200 pl-3">
      <button
        type="button"
        onClick={() => setShowContent(!showContent)}
        className="flex w-full items-start gap-2 text-left text-sm transition-colors hover:text-slate-900"
      >
        <span className="font-semibold text-blue-600">Model Output</span>
        <span className="text-xs text-slate-400">
          iteration {modelOutput.iteration}
        </span>
      </button>

      {showContent && <TaskCompleteCard event={mockEvent} />}
    </div>
  );
}
