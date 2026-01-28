// EventLine component - renders a single agent event
// Optimized with React.memo for virtual scrolling performance

import React from "react";
import {
  AnyAgentEvent,
  WorkflowArtifactManifestEvent,
  WorkflowToolCompletedEvent,
  WorkflowToolStartedEvent,
  WorkflowNodeOutputSummaryEvent,
  WorkflowResultFinalEvent,
} from "@/lib/types";
import { isEventType } from "@/lib/events/matching";
import { formatContent, formatTimestamp } from "./formatters";
import { getEventStyle } from "./styles";
import { ToolOutputCard } from "../ToolOutputCard";
import { TaskCompleteCard } from "../TaskCompleteCard";
import { cn, isOrchestratorRetryMessage } from "@/lib/utils";
import { parseContentSegments, buildAttachmentUri } from "@/lib/attachments";
import { ImagePreview } from "@/components/ui/image-preview";
import { VideoPreview } from "@/components/ui/video-preview";
import { isSubagentLike } from "@/lib/subagent";
import { ArtifactPreviewCard } from "../ArtifactPreviewCard";
import { Badge } from "@/components/ui/badge";
import { AgentMarkdown } from "../AgentMarkdown";
import { AlexWordmark } from "@/components/ui/alex-wordmark";
import Image from "next/image";
import { CompactToolCall } from "../AgentCard/CompactToolCall";

interface EventLineProps {
  event: AnyAgentEvent;
  showSubagentContext?: boolean;
  pairedToolStartEvent?: WorkflowToolStartedEvent | null;
  variant?: "default" | "nested";
}

/**
 * EventLine - Single event display component
 * Memoized for optimal virtual scrolling performance
 */
export const EventLine = React.memo(function EventLine({
  event,
  showSubagentContext = false,
  pairedToolStartEvent = null,
  variant = "default",
}: EventLineProps) {
  const isNested = variant === "nested";
  const isSubagentEvent = isSubagentLike(event);
  const subagentContext = isSubagentEvent ? getSubagentContext(event) : null;

  const wrapWithSubagentContext = (content: React.ReactNode) => {
    if (!isSubagentEvent || !subagentContext || !showSubagentContext) {
      return content;
    }

    return (
      <div
        className="space-y-1"
        data-testid={`event-subagent-${event.event_type}`}
      >
        <SubagentHeader context={subagentContext} />
        {content}
      </div>
    );
  };

  // User Task / Input
  if (event.event_type === "workflow.input.received") {
    const segments = parseContentSegments(
      event.task,
      event.attachments ?? undefined,
    );
    const textSegments = segments.filter(
      (segment) =>
        segment.type === "text" && segment.text && segment.text.length > 0,
    );
    const mediaSegments = segments.filter(
      (segment) => segment.type === "image" || segment.type === "video",
    );
    const artifactSegments = segments.filter(
      (segment) =>
        (segment.type === "document" || segment.type === "embed") &&
        segment.attachment,
    );

    const hasTextContent = textSegments.some(
      (segment) => typeof segment.text === "string" && segment.text.length > 0,
    );
    return wrapWithSubagentContext(
      <div
        className="py-2 flex justify-end"
        data-testid="event-workflow.input.received"
      >
        <div className="flex w-full max-w-[min(36rem,100%)] flex-col items-end gap-2">
          {hasTextContent && (
            <div className="w-fit max-w-full rounded-2xl border border-border/60 bg-background px-4 py-3 shadow-sm">
              <div className="text-base text-foreground">
                {textSegments.map((segment, index) => (
                  <p
                    key={`text-segment-${index}`}
                    className="whitespace-pre-wrap leading-normal"
                  >
                    {segment.text}
                  </p>
                ))}
              </div>
            </div>
          )}

          {mediaSegments.length > 0 && (
            <div
              className="w-full grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-2"
              data-testid="event-input-media"
            >
              {mediaSegments.map((segment, index) => {
                if (!segment.attachment) return null;
                const uri = buildAttachmentUri(segment.attachment);
                if (!uri) return null;
                if (segment.type === "video") {
                  return (
                    <VideoPreview
                      key={index}
                      src={uri}
                      mimeType={segment.attachment.media_type || "video/mp4"}
                      description={segment.attachment.description}
                      className="rounded-lg border border-border/20"
                      maxHeight="16rem"
                      controls
                    />
                  );
                }
                return (
                  <ImagePreview
                    key={index}
                    src={uri}
                    alt={segment.attachment.description || "User upload"}
                    className="rounded-lg border border-border/20"
                    maxHeight="16rem"
                  />
                );
              })}
            </div>
          )}

          {artifactSegments.length > 0 && (
            <div className="w-full space-y-2">
              {artifactSegments.map((segment, index) =>
                segment.attachment ? (
                  <ArtifactPreviewCard
                    key={index}
                    attachment={segment.attachment}
                  />
                ) : null,
              )}
            </div>
          )}

          <div className="text-[10px] text-muted-foreground/40">
            {formatTimestamp(event.timestamp)}
          </div>
        </div>
      </div>,
    );
  }

  // Tool call complete - use ToolOutputCard
  if (event.event_type === "workflow.artifact.manifest") {
    const manifestEvent = event as WorkflowArtifactManifestEvent;
    const payload =
      manifestEvent.payload &&
        typeof manifestEvent.payload === "object" &&
        !Array.isArray(manifestEvent.payload)
        ? (manifestEvent.payload as Record<string, any>)
        : null;
    const manifest =
      manifestEvent.manifest ??
      (payload?.manifest as Record<string, any> | undefined) ??
      payload;
    const rawAttachments =
      manifestEvent.attachments ??
      (payload?.attachments as Record<string, any> | undefined) ??
      (manifest && typeof manifest === "object"
        ? (manifest as Record<string, any>).attachments
        : undefined);
    const attachments =
      rawAttachments && typeof rawAttachments === "object"
        ? (rawAttachments as Record<string, any>)
        : undefined;
    const summary =
      (typeof manifestEvent.result === "string" &&
        manifestEvent.result.trim().length > 0
        ? manifestEvent.result
        : undefined) ??
      (manifest &&
        typeof manifest === "object" &&
        typeof (manifest as Record<string, any>).summary === "string"
        ? (manifest as Record<string, any>).summary
        : undefined) ??
      "Artifact manifest received.";

    return wrapWithSubagentContext(
      <div data-testid="event-workflow.artifact.manifest" className="py-1">
        <ToolOutputCard
          toolName="artifact_manifest"
          result={summary}
          metadata={manifest ? { manifest } : undefined}
          attachments={attachments as Record<string, any> | undefined}
          timestamp={manifestEvent.timestamp}
        />
      </div>,
    );
  }

  if (event.event_type === "workflow.tool.started") {
    if (isNested || isSubagentEvent) {
      const content = formatContent(event);
      const style = getEventStyle(event);
      if (!content) {
        return null;
      }
      return wrapWithSubagentContext(
        <div
          data-testid="event-workflow.tool.started"
          className={cn(
            "text-sm py-0.5 flex gap-3 text-muted-foreground/80 hover:text-foreground/90",
            style.content,
          )}
        >
          <div className="flex-1 leading-relaxed break-words">{content}</div>
        </div>,
      );
    }

    const startedEvent = event as WorkflowToolStartedEvent;
    return wrapWithSubagentContext(
      <div
        data-testid="event-workflow.tool.started"
        className={cn("py-1", !isNested && "pl-2 border-primary/10")}
      >
        <ToolOutputCard
          toolName={startedEvent.tool_name}
          parameters={startedEvent.arguments}
          timestamp={startedEvent.timestamp}
          callId={startedEvent.call_id}
          status="running"
        />
      </div>,
    );
  }

  if (event.event_type === "workflow.tool.completed") {
    const completeEvent = event as WorkflowToolCompletedEvent & {
      arguments?: Record<string, unknown>;
    };
    const pairedArguments =
      completeEvent.arguments &&
        typeof completeEvent.arguments === "object" &&
        !Array.isArray(completeEvent.arguments)
        ? (completeEvent.arguments as Record<string, unknown>)
        : pairedToolStartEvent?.arguments;
    const toolName = (completeEvent.tool_name ?? "").toLowerCase();
    const resultText = resolveToolResultText(completeEvent.result);
    if (toolName === "plan") {
      if (isOrchestratorRetryMessage(resultText)) {
        return null;
      }
      return wrapWithSubagentContext(
        <PlanGoalCard
          goal={resultText ?? ""}
          timestamp={completeEvent.timestamp}
        />,
      );
    }
    if (toolName === "clarify") {
      if (isOrchestratorRetryMessage(resultText)) {
        return null;
      }
      return wrapWithSubagentContext(
        <ClarifyTaskCard
          result={resultText ?? ""}
          metadata={completeEvent.metadata}
          timestamp={completeEvent.timestamp}
        />,
      );
    }
    if (isNested || isSubagentEvent) {
      const resultStr = completeEvent.error
        ? String(completeEvent.error)
        : resolveToolResultText(completeEvent.result);

      return wrapWithSubagentContext(
        <div data-testid="event-workflow.tool.completed">
          <CompactToolCall
            toolName={completeEvent.tool_name}
            success={!completeEvent.error}
            result={resultStr}
            error={completeEvent.error}
            duration={completeEvent.duration}
            parameters={pairedArguments}
          />
        </div>,
      );
    }
    return wrapWithSubagentContext(
      <div
        data-testid="event-workflow.tool.completed"
        className={cn("py-1", !isNested && "pl-2 border-primary/10")}
      >
        <ToolOutputCard
          toolName={completeEvent.tool_name}
          parameters={pairedArguments}
          result={completeEvent.result}
          error={completeEvent.error}
          duration={completeEvent.duration}
          timestamp={completeEvent.timestamp}
          callId={completeEvent.call_id}
          metadata={completeEvent.metadata}
          attachments={completeEvent.attachments ?? undefined}
        />
      </div>,
    );
  }

  // Task complete
  if (event.event_type === "workflow.result.final") {
    return wrapWithSubagentContext(
      <div
        className={cn("py-2", isNested && "py-1")}
        data-testid="event-workflow.result.final"
      >
        <TaskCompleteCard event={event as WorkflowResultFinalEvent} />
      </div>,
    );
  }

  // Assistant streaming (delta)
  if (event.event_type === "workflow.node.output.delta") {
    const delta =
      "delta" in event && typeof event.delta === "string" ? event.delta : "";
    if (!delta.trim()) {
      return null;
    }
    const finalFlag =
      "final" in event && typeof (event as { final?: boolean }).final === "boolean"
        ? (event as { final?: boolean }).final
        : undefined;
    const streamFinished = finalFlag === true;
    const isStreaming = !streamFinished;
    return wrapWithSubagentContext(
      <div
        className={cn("py-2", !isNested && "pl-4 border-l-2 border-primary/10")}
        data-testid="event-workflow.node.output.delta"
      >
        <AgentMarkdown
          content={delta}
          className="prose max-w-none text-base leading-snug text-foreground"
          isStreaming={isStreaming}
          streamFinished={streamFinished}
        />
      </div>,
    );
  }

  // Assistant log (ReAct)
  if (event.event_type === "workflow.node.output.summary") {
    const thinkEvent = event as WorkflowNodeOutputSummaryEvent;
    if (thinkEvent.content && thinkEvent.content.trim().length > 0) {
      const hasAttachments =
        Boolean(thinkEvent.attachments) &&
        typeof thinkEvent.attachments === "object" &&
        Object.keys(thinkEvent.attachments ?? {}).length > 0;
      if (hasAttachments) {
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
          is_streaming: false,
          stream_finished: true,
        };

        return wrapWithSubagentContext(
          <div
            className={cn(
              "py-2",
              !isNested && "pl-4 border-l-2 border-primary/10",
            )}
            data-testid="event-workflow.node.output.summary"
          >
            <TaskCompleteCard event={mockWorkflowResultFinalEvent} />
          </div>,
        );
      }
      return wrapWithSubagentContext(
        <AssistantLogCard
          content={thinkEvent.content}
          timestamp={thinkEvent.timestamp}
          variant={variant}
        />,
      );
    }
  }

  // Other events - use simple line format
  const content = formatContent(event);
  const style = getEventStyle(event);
  if (!content) {
    return null;
  }
  return wrapWithSubagentContext(
    <div
      className={cn(
        "text-sm py-0.5 flex gap-3 text-muted-foreground/80 hover:text-foreground/90",
        style.content,
      )}
    >
      <div className="flex-1 leading-relaxed break-words">{content}</div>
    </div>,
  );
});

function PlanGoalCard({
  goal,
  timestamp,
}: {
  goal: string;
  timestamp?: string;
}) {
  return (
    <div className="py-1 mb-1" data-testid="event-ui-plan">
      <AgentMarkdown
        content={goal}
        className="prose max-w-none text-base leading-snug text-foreground"
      />
    </div>
  );
}

function ClarifyTaskCard({
  result,
  metadata,
  timestamp,
}: {
  result: string;
  metadata?: Record<string, any>;
  timestamp?: string;
}) {
  const taskGoalUI =
    typeof metadata?.task_goal_ui === "string" && metadata.task_goal_ui.trim()
      ? String(metadata.task_goal_ui).trim()
      : (result.split(/\r?\n/)[0]?.trim() ?? "");
  const successCriteria = Array.isArray(metadata?.success_criteria)
    ? (metadata?.success_criteria as unknown[])
      .map((item) => (typeof item === "string" ? item.trim() : ""))
      .filter((item) => item.length > 0)
    : [];
  const needsUserInput = metadata?.needs_user_input === true;
  const questionToUser =
    typeof metadata?.question_to_user === "string" &&
      metadata.question_to_user.trim()
      ? String(metadata.question_to_user).trim()
      : null;

  return (
    <div className="py-2 border-primary/10" data-testid="event-ui-clarify">
      <div className="text-sm text-foreground whitespace-pre-wrap leading-normal">
        {taskGoalUI}
      </div>

      {successCriteria.length > 0 && (
        <ul className="mt-2 list-disc pl-5 text-xs text-muted-foreground/80 space-y-1">
          {successCriteria.map((crit) => (
            <li key={crit}>{crit}</li>
          ))}
        </ul>
      )}

      {needsUserInput && questionToUser ? (
        <div className="mt-2 rounded-md border border-amber-200/60 bg-amber-50/40 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-100">
          {questionToUser}
        </div>
      ) : null}
    </div>
  );
}

function resolveToolResultText(result: unknown): string | undefined {
  if (typeof result === "string") {
    return result;
  }
  if (result && typeof result === "object") {
    const record = result as Record<string, unknown>;
    if ("output" in record) {
      return String(record.output ?? "");
    }
    if ("content" in record) {
      return String(record.content ?? "");
    }
    try {
      return JSON.stringify(record);
    } catch {
      return String(record);
    }
  }
  if (result != null) {
    return String(result);
  }
  return undefined;
}

function AssistantLogCard({
  content,
  timestamp,
  variant = "default",
}: {
  content: string;
  timestamp?: string;
  variant?: "default" | "nested";
}) {
  return (
    <div
      className={cn(
        "py-2",
        variant !== "nested" && "pl-4 border-l-2 border-primary/10",
      )}
      data-testid="event-workflow.node.output.summary"
    >
      <AgentMarkdown
        content={content}
        className="prose max-w-none text-sm leading-snug text-foreground"
      />
    </div>
  );
}

export interface SubagentContext {
  preview?: string;
  concurrency?: string;
  progress?: string;
  stats?: string;
  status?: string;
  statusTone?: "info" | "success" | "warning" | "danger";
}

function resolveSubtaskPreview(event: AnyAgentEvent): string | undefined {
  const direct =
    "subtask_preview" in event && typeof event.subtask_preview === "string"
      ? event.subtask_preview
      : "";
  if (direct.trim()) {
    return direct.trim();
  }

  const payload =
    "payload" in event && event.payload && typeof event.payload === "object"
      ? (event.payload as Record<string, unknown>)
      : null;
  const payloadPreview =
    payload && typeof payload.subtask_preview === "string"
      ? payload.subtask_preview
      : "";
  if (payloadPreview.trim()) {
    return payloadPreview.trim();
  }

  const payloadTask =
    payload && typeof payload.task === "string" ? payload.task : "";
  if (payloadTask.trim()) {
    return payloadTask.trim();
  }

  const task =
    "task" in event && typeof event.task === "string" ? event.task : "";
  if (task.trim()) {
    return task.trim();
  }

  return undefined;
}

function resolveMaxParallel(event: AnyAgentEvent): number | undefined {
  if ("max_parallel" in event && typeof event.max_parallel === "number") {
    return event.max_parallel;
  }
  if (
    "payload" in event &&
    event.payload &&
    typeof event.payload === "object" &&
    typeof (event.payload as Record<string, unknown>).max_parallel === "number"
  ) {
    return (event.payload as Record<string, unknown>).max_parallel as number;
  }
  return undefined;
}

export function getSubagentContext(event: AnyAgentEvent): SubagentContext {
  const preview = resolveSubtaskPreview(event);
  const maxParallel = resolveMaxParallel(event);
  const concurrency: string | undefined = undefined;

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
    statsParts.push(
      `${event.tool_calls} tool call${event.tool_calls === 1 ? "" : "s"}`,
    );
  }
  const tokenCount =
    ("tokens" in event && typeof event.tokens === "number" && event.tokens) ||
    ("total_tokens" in event &&
      typeof event.total_tokens === "number" &&
      event.total_tokens) ||
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
  } else if (isEventType(event, "workflow.node.failed")) {
    status = "Subagent reported an error";
    statusTone = "danger";
  }

  return {
    preview,
    concurrency,
    progress: progressParts.join(" · ") || undefined,
    stats: statsParts.join(" · ") || undefined,
    status,
    statusTone,
  };
}

interface SubagentHeaderProps {
  context: SubagentContext;
}

export function SubagentHeader({ context }: SubagentHeaderProps) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {context.preview && (
        <span className="text-xs text-foreground/70 truncate max-w-[200px]">
          {context.preview}
        </span>
      )}
      {context.concurrency && (
        <Badge
          variant="secondary"
          className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground"
        >
          {context.concurrency}
        </Badge>
      )}
      {context.status && (
        <span
          className={cn(
            "text-[10px] px-1.5 py-0.5 rounded font-medium",
            context.statusTone === "success"
              ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
              : context.statusTone === "danger"
                ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                : "bg-muted text-muted-foreground",
          )}
        >
          {context.status}
        </span>
      )}
    </div>
  );
}
