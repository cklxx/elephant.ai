// EventLine component - renders a single agent event
// Optimized with React.memo for virtual scrolling performance

import React from "react";
import Image from "next/image";
import {
  AnyAgentEvent,
  ToolCallCompleteEvent,
  ThinkCompleteEvent,
  TaskCompleteEvent,
} from "@/lib/types";
import { formatContent, formatTimestamp } from "./formatters";
import { getEventStyle } from "./styles";
import { ToolOutputCard } from "../ToolOutputCard";
import { TaskCompleteCard } from "../TaskCompleteCard";
import { cn } from "@/lib/utils";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";

interface EventLineProps {
  event: AnyAgentEvent;
}

/**
 * EventLine - Single event display component
 * Memoized for optimal virtual scrolling performance
 */
export const EventLine = React.memo(function EventLine({
  event,
}: EventLineProps) {
  if (event.event_type === "user_task") {
    const segments = parseContentSegments(event.task, event.attachments);
    const style = getEventStyle(event);
    return (
      <div className={cn("console-event-line", style.line)}>
        <div className="console-event-timestamp">
          {formatTimestamp(event.timestamp)}
        </div>
        <div
          className={cn(
            "console-event-content flex flex-col gap-3",
            style.content,
          )}
        >
          {segments.map((segment, index) => {
            if (segment.type === "image" && segment.attachment) {
              const uri = buildAttachmentUri(segment.attachment);
              if (!uri) {
                return (
                  <span key={`segment-${index}`}>
                    {segment.placeholder ?? ""}
                  </span>
                );
              }
              return (
                <figure
                  key={`segment-${index}`}
                  className="flex w-full flex-col gap-1 rounded-lg border border-border/60 bg-white p-2 shadow-sm"
                >
                  <div className="relative h-48 w-full overflow-hidden rounded-md bg-slate-50">
                    <Image
                      src={uri}
                      alt={
                        segment.attachment.description ||
                        segment.attachment.name
                      }
                      fill
                      className="object-contain"
                      sizes="(min-width: 1280px) 32vw, (min-width: 768px) 48vw, 90vw"
                      unoptimized
                    />
                  </div>
                  <figcaption className="text-[10px] uppercase tracking-wide text-muted-foreground">
                    {segment.attachment.description || segment.attachment.name}
                  </figcaption>
                </figure>
              );
            }

            if (segment.type === "text" && segment.text) {
              return <span key={`segment-${index}`}>{segment.text}</span>;
            }

            return null;
          })}
        </div>
      </div>
    );
  }

  // Tool call complete - use ToolOutputCard
  if (event.event_type === "tool_call_complete") {
    const completeEvent = event as ToolCallCompleteEvent & {
      arguments?: Record<string, unknown>;
    };
    return (
      <ToolOutputCard
        toolName={completeEvent.tool_name}
        parameters={completeEvent.arguments}
        result={completeEvent.result}
        error={completeEvent.error}
        duration={completeEvent.duration}
        timestamp={completeEvent.timestamp}
        callId={completeEvent.call_id}
        metadata={completeEvent.metadata}
        attachments={completeEvent.attachments}
      />
    );
  }

  // Task complete - use TaskCompleteCard
  if (event.event_type === "task_complete") {
    return <TaskCompleteCard event={event as TaskCompleteEvent} />;
  }

  // Think complete - convert to TaskCompleteCard format
  if (event.event_type === "think_complete") {
    const thinkEvent = event as ThinkCompleteEvent;
    if (thinkEvent.content) {
      const mockTaskCompleteEvent: TaskCompleteEvent = {
        event_type: "task_complete",
        timestamp: thinkEvent.timestamp,
        agent_level: thinkEvent.agent_level,
        session_id: thinkEvent.session_id,
        task_id: thinkEvent.task_id,
        parent_task_id: thinkEvent.parent_task_id,
        final_answer: thinkEvent.content,
        total_iterations: thinkEvent.iteration,
        total_tokens: 0,
        stop_reason: "think_complete",
        duration: 0,
      };
      return <TaskCompleteCard event={mockTaskCompleteEvent} />;
    }
  }

  // Other events - use simple line format
  const timestamp = formatTimestamp(event.timestamp);
  const content = formatContent(event);
  const style = getEventStyle(event);
  if (!content) {
    return null;
  }
  return (
    <div className={cn("console-event-line items-center flex", style.line)}>
      <div className={cn("console-event-content", style.content)}>
        {content}
      </div>
    </div>
  );
});
