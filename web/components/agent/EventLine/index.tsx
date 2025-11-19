// EventLine component - renders a single agent event
// Optimized with React.memo for virtual scrolling performance

import React from "react";
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
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { ArtifactPreviewCard } from "../ArtifactPreviewCard";

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
  const isSubtaskEvent =
    event.agent_level === "subagent" ||
    ("is_subtask" in event && Boolean(event.is_subtask));

  if (isSubtaskEvent) {
    return <SubagentEventLine event={event} />;
  }

  if (event.event_type === "user_task") {
    const segments = parseContentSegments(event.task, event.attachments);
    const textSegments = segments.filter(
      (segment) => segment.type === "text" && segment.text && segment.text.length > 0,
    );
    const mediaSegments = segments.filter(
      (segment) => segment.type === "image" || segment.type === "video",
    );
    const artifactSegments = segments.filter(
      (segment) =>
        (segment.type === "document" || segment.type === "embed") &&
        segment.attachment,
    );
    return (
      <div className="console-user-task" data-testid="event-user_task">
        <div className="console-user-task-bubble">
          <div className="console-user-task-meta">
            {formatTimestamp(event.timestamp)}
          </div>
          <div className="console-user-task-content">
            {textSegments.map((segment, index) => (
              <p
                key={`text-segment-${index}`}
                className="m-0 whitespace-pre-wrap"
              >
                {segment.text}
              </p>
            ))}
            {mediaSegments.map((segment, index) => {
              if (!segment.attachment) {
                return null;
              }
              const uri = buildAttachmentUri(segment.attachment);
              if (!uri) {
                return null;
              }
              const key = segment.placeholder || `${segment.type}-${index}`;
              const attachmentName =
                segment.attachment.description ||
                segment.attachment.name ||
                key;
              if (segment.type === "video") {
                return (
                  <div
                    key={`task-media-${key}`}
                    className="mt-3"
                    data-testid="event-attachment-video"
                    data-attachment-name={attachmentName}
                  >
                    <VideoPreview
                      src={uri}
                      mimeType={segment.attachment.media_type || "video/mp4"}
                      description={segment.attachment.description}
                      maxHeight="20rem"
                    />
                  </div>
                );
              }
              return (
                <div
                  key={`task-media-${key}`}
                  className="mt-3"
                  data-testid="event-attachment-image"
                  data-attachment-name={attachmentName}
                >
                  <ImagePreview
                    src={uri}
                    alt={segment.attachment.description || segment.attachment.name}
                    minHeight="12rem"
                    maxHeight="20rem"
                    sizes="(min-width: 1280px) 32vw, (min-width: 768px) 48vw, 90vw"
                  />
                </div>
              );
            })}
            {artifactSegments.length > 0 && (
              <div className="mt-4 space-y-3">
                {artifactSegments.map((segment, index) => {
                  if (!segment.attachment) {
                    return null;
                  }
                  const key = segment.placeholder || `artifact-${index}`;
                  return (
                    <ArtifactPreviewCard
                      key={`task-artifact-${key}`}
                      attachment={segment.attachment}
                    />
                  );
                })}
              </div>
            )}
          </div>
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
      <div data-testid="event-tool_call_complete">
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
      </div>
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
    <div
      className={cn("console-event-line items-center flex", style.line)}
      data-testid={`event-${event.event_type}`}
    >
      <div className={cn("console-event-content", style.content)}>
        {content}
      </div>
    </div>
  );
});

interface SubagentEventLineProps {
  event: AnyAgentEvent;
}

function SubagentEventLine({ event }: SubagentEventLineProps) {
  const context = getSubagentContext(event);

  if (event.event_type === "tool_call_complete") {
    const completeEvent = event as ToolCallCompleteEvent & {
      arguments?: Record<string, unknown>;
    };
    return (
      <div
        className="space-y-3"
        data-testid={`event-subagent-${event.event_type}`}
      >
        <SubagentHeader context={context} />
        <ToolOutputCard
          toolName={completeEvent.tool_name}
          parameters={completeEvent.arguments}
          result={completeEvent.result}
          error={completeEvent.error}
          duration={completeEvent.duration}
          callId={completeEvent.call_id}
          metadata={completeEvent.metadata}
          attachments={completeEvent.attachments}
          status={completeEvent.error ? "failed" : "completed"}
        />
      </div>
    );
  }

  if (event.event_type === "task_complete") {
    return (
      <div
        className="space-y-3"
        data-testid="event-subagent-task_complete"
      >
        <SubagentHeader context={context} />
        <TaskCompleteCard event={event as TaskCompleteEvent} />
      </div>
    );
  }

  const content = formatContent(event);
  if (!content) {
    return null;
  }

  const style = getEventStyle(event);

  return (
    <div
      className="space-y-2"
      data-testid={`event-subagent-${event.event_type}`}
    >
      <SubagentHeader context={context} />
      <div
        className={cn(
          "console-event-line flex rounded-lg border border-primary/30 bg-primary/5 px-3 py-2",
          style.line,
        )}
      >
        <div className={cn("console-event-content", style.content)}>
          {content}
        </div>
      </div>
    </div>
  );
}

interface SubagentContext {
  title: string;
  preview?: string;
  concurrency?: string;
}

function getSubagentContext(event: AnyAgentEvent): SubagentContext {
  const index =
    "subtask_index" in event && typeof event.subtask_index === "number"
      ? event.subtask_index + 1
      : undefined;
  const total =
    "total_subtasks" in event &&
    typeof event.total_subtasks === "number" &&
    event.total_subtasks > 0
      ? event.total_subtasks
      : undefined;

  let title = "Subagent Task";
  if (index !== undefined && total !== undefined) {
    title = `Subagent Task ${index}/${total}`;
  } else if (index !== undefined) {
    title = `Subagent Task ${index}`;
  }

  const preview =
    "subtask_preview" in event ? event.subtask_preview?.trim() : undefined;
  const concurrency =
    "max_parallel" in event && event.max_parallel && event.max_parallel > 1
      ? `Parallel ×${event.max_parallel}`
      : undefined;

  return { title, preview, concurrency };
}

interface SubagentHeaderProps {
  context: SubagentContext;
}

function SubagentHeader({ context }: SubagentHeaderProps) {
  return (
    <div className="space-y-1">
      <p className="text-[10px] font-semibold uppercase tracking-[0.28em] text-primary">
        {context.title}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {context.preview && (
          <span className="text-xs text-foreground/80">
            {context.preview}
          </span>
        )}
        {context.concurrency && (
          <span className="text-[10px] uppercase tracking-[0.2em] text-muted-foreground">
            {context.concurrency}
          </span>
        )}
      </div>
    </div>
  );
}
