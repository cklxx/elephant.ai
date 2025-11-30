// EventLine component - renders a single agent event
// Optimized with React.memo for virtual scrolling performance

import React from "react";
import {
  AnyAgentEvent,
  WorkflowToolCompletedEvent,
  WorkflowNodeOutputSummaryEvent,
  WorkflowResultFinalEvent,
  eventMatches,
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
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface EventLineProps {
  event: AnyAgentEvent;
  showSubagentContext?: boolean;
}

/**
 * EventLine - Single event display component
 * Memoized for optimal virtual scrolling performance
 */
export const EventLine = React.memo(function EventLine({
  event,
  showSubagentContext = true,
}: EventLineProps) {
  const isSubtaskEvent = isSubagentLike(event);

  if (isSubtaskEvent) {
    return (
      <SubagentEventLine event={event} showContext={showSubagentContext} />
    );
  }

  if (event.event_type === "workflow.input.received") {
    const segments = parseContentSegments(event.task, event.attachments ?? undefined);
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
      <Card data-testid="event-workflow.input.received">
        <CardContent className="space-y-3">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>{formatTimestamp(event.timestamp)}</span>
            <Badge variant="outline" className="text-[10px] font-medium">
              User Task
            </Badge>
          </div>
          <div className="space-y-2">
            {textSegments.map((segment, index) => (
              <p
                key={`text-segment-${index}`}
                className="m-0 whitespace-pre-wrap text-sm leading-relaxed text-foreground"
              >
                {segment.text}
              </p>
            ))}
          </div>
          {mediaSegments.length > 0 && (
            <div className="space-y-3">
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
                      className="mt-1"
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
                    className="mt-1"
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
            </div>
          )}
          {artifactSegments.length > 0 && (
            <div className="mt-2 space-y-3">
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
        </CardContent>
      </Card>
    );
  }

  // Tool call complete - use ToolOutputCard
  if (event.event_type === "workflow.tool.completed") {
    const completeEvent = event as WorkflowToolCompletedEvent & {
      arguments?: Record<string, unknown>;
    };
    return (
      <div data-testid="event-workflow.tool.completed">
        <ToolOutputCard
          toolName={completeEvent.tool_name}
          parameters={completeEvent.arguments}
          result={completeEvent.result}
          error={completeEvent.error}
          duration={completeEvent.duration}
          timestamp={completeEvent.timestamp}
          callId={completeEvent.call_id}
          metadata={completeEvent.metadata}
          attachments={completeEvent.attachments ?? undefined}
        />
      </div>
    );
  }

  // Task complete - use TaskCompleteCard
  if (event.event_type === "workflow.result.final") {
    return <TaskCompleteCard event={event as WorkflowResultFinalEvent} />;
  }

  // Think complete - convert to TaskCompleteCard format
  if (event.event_type === "workflow.node.output.summary") {
    const thinkEvent = event as WorkflowNodeOutputSummaryEvent;
    if (thinkEvent.content) {
      const mockWorkflowResultFinalEvent: WorkflowResultFinalEvent = {
        event_type: "workflow.result.final",
        timestamp: thinkEvent.timestamp,
        agent_level: thinkEvent.agent_level,
        session_id: thinkEvent.session_id,
        task_id: thinkEvent.task_id,
        parent_task_id: thinkEvent.parent_task_id,
        final_answer: thinkEvent.content,
        attachments: thinkEvent.attachments,
        total_iterations: thinkEvent.iteration ?? 0,
        total_tokens: 0,
        stop_reason: "workflow.node.output.summary",
        duration: 0,
      };
      return <TaskCompleteCard event={mockWorkflowResultFinalEvent} />;
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
    <Card data-testid={`event-${event.event_type}`}>
      <CardContent className={cn("text-sm leading-relaxed", style.content)}>
        <div className="text-xs text-muted-foreground">{timestamp}</div>
        <div className="mt-1">{content}</div>
      </CardContent>
    </Card>
  );
});

interface SubagentEventLineProps {
  event: AnyAgentEvent;
}

function SubagentEventLine({
  event,
  showContext = true,
}: SubagentEventLineProps & { showContext?: boolean }) {
  const context = getSubagentContext(event);

  if (event.event_type === "workflow.tool.completed") {
    const completeEvent = event as WorkflowToolCompletedEvent & {
      arguments?: Record<string, unknown>;
    };
    return (
      <div
        className="space-y-3"
        data-testid={`event-subagent-${event.event_type}`}
      >
        {showContext && <SubagentHeader context={context} />}
        <ToolOutputCard
          toolName={completeEvent.tool_name}
          parameters={completeEvent.arguments}
          result={completeEvent.result}
          error={completeEvent.error}
          duration={completeEvent.duration}
          callId={completeEvent.call_id}
          metadata={completeEvent.metadata}
          attachments={completeEvent.attachments ?? undefined}
          status={completeEvent.error ? "failed" : "completed"}
        />
      </div>
    );
  }

  if (event.event_type === "workflow.result.final") {
    return (
      <div
        className="space-y-3"
        data-testid="event-subagent-workflow.result.final"
      >
        {showContext && <SubagentHeader context={context} />}
        <TaskCompleteCard event={event as WorkflowResultFinalEvent} />
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
      {showContext && <SubagentHeader context={context} />}
      <Card>
        <CardContent className={cn("text-sm leading-relaxed", style.content)}>
          <div className="text-xs text-muted-foreground">{formatTimestamp(event.timestamp)}</div>
          <div className="mt-1">{content}</div>
        </CardContent>
      </Card>
    </div>
  );
}

export interface SubagentContext {
  title: string;
  preview?: string;
  concurrency?: string;
  progress?: string;
  stats?: string;
  status?: string;
  statusTone?: 'info' | 'success' | 'warning' | 'danger';
}

export function getSubagentContext(event: AnyAgentEvent): SubagentContext {
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

  const progressParts: string[] = [];
  if ("completed" in event && typeof event.completed === "number") {
    const totalLabel =
      "total" in event && typeof event.total === "number"
        ? `${event.total}`
        : "?";
    progressParts.push(`Progress ${event.completed}/${totalLabel}`);
  }

  const statsParts: string[] = [];
  if ("tool_calls" in event && typeof event.tool_calls === "number") {
    statsParts.push(`${event.tool_calls} tool call${event.tool_calls === 1 ? "" : "s"}`);
  }
  const tokenCount =
    ("tokens" in event && typeof event.tokens === "number" && event.tokens) ||
    ("total_tokens" in event && typeof event.total_tokens === "number" && event.total_tokens) ||
    undefined;
  if (typeof tokenCount === "number") {
    statsParts.push(`${tokenCount} tokens`);
  }

  let status: SubagentContext["status"];
  let statusTone: SubagentContext["statusTone"];
  if (
    event.event_type === "workflow.subflow.completed" &&
    "success" in event &&
    typeof event.success === "number" &&
    "failed" in event &&
    typeof event.failed === "number"
  ) {
    status = `${event.success}/${event.total ?? event.success + event.failed} succeeded`;
    statusTone = event.failed > 0 ? "warning" : "success";
  } else if (eventMatches(event, "workflow.node.failed")) {
    status = "Subagent reported an error";
    statusTone = "danger";
  }

  return {
    title,
    preview,
    concurrency,
    progress: progressParts.join(" · ") || undefined,
    stats: statsParts.join(" · ") || undefined,
    status,
    statusTone,
  };
}

export function isSubagentLike(event: AnyAgentEvent): boolean {
  if (!event) return false;

  if (event.agent_level === "subagent") return true;
  if ("is_subtask" in event && Boolean((event as any).is_subtask)) return true;

  const parentTask =
    "parent_task_id" in event && typeof (event as any).parent_task_id === "string"
      ? (event as any).parent_task_id.trim()
      : "";
  if (parentTask) return true;

  const nodeId =
    "node_id" in event && typeof (event as any).node_id === "string"
      ? (event as any).node_id.toLowerCase()
      : "";
  if (nodeId.startsWith("subagent") || nodeId.startsWith("subflow-")) {
    return true;
  }

  const callId =
    "call_id" in event && typeof (event as any).call_id === "string"
      ? (event as any).call_id.toLowerCase()
      : "";
  if (callId.startsWith("subagent")) {
    return true;
  }

  const taskId = typeof event.task_id === "string" ? event.task_id.toLowerCase() : "";
  if (taskId.startsWith("subagent")) {
    return true;
  }

  return eventMatches(
    event,
    "workflow.subflow.progress",
    "workflow.subflow.completed",
  );
}

interface SubagentHeaderProps {
  context: SubagentContext;
}

export function SubagentHeader({ context }: SubagentHeaderProps) {
  return (
    <div className="space-y-2">
      <p className="text-[10px] font-semibold text-primary">
        {context.title}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        {context.preview && (
          <span className="text-xs text-foreground/80">
            {context.preview}
          </span>
        )}
        {context.concurrency && (
          <Badge variant="outline" className="text-[10px] font-medium">
            {context.concurrency}
          </Badge>
        )}
        {context.progress && (
          <Badge variant="outline" className="text-[10px] font-medium">
            {context.progress}
          </Badge>
        )}
        {context.stats && (
          <Badge variant="outline" className="text-[10px] font-medium">
            {context.stats}
          </Badge>
        )}
        {context.status && (
          <Badge
            variant={
              context.statusTone === "success"
                ? "success"
                : context.statusTone === "warning"
                ? "warning"
                : context.statusTone === "danger"
                ? "destructive"
                : "info"
            }
            className="text-[10px] font-medium"
          >
            {context.status}
          </Badge>
        )}
      </div>
    </div>
  );
}
