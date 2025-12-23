"use client";

import { useMemo, useState } from "react";
import type {
  AnyAgentEvent,
  WorkflowToolCompletedEvent,
  WorkflowToolStartedEvent,
} from "@/lib/types";
import { Check, Loader2, CircleHelp } from "lucide-react";
import { cn } from "@/lib/utils";
import { EventLine } from "./EventLine";

export interface ClearifyTaskGroup {
  clearifyEvent: WorkflowToolCompletedEvent;
  events: AnyAgentEvent[];
}

interface ClearifyTimelineProps {
  groups: ClearifyTaskGroup[];
  isRunning: boolean;
  resolvePairedToolStart: (
    event: AnyAgentEvent,
  ) => WorkflowToolStartedEvent | undefined;
}

export function ClearifyTimeline({
  groups,
  isRunning,
  resolvePairedToolStart,
}: ClearifyTimelineProps) {
  if (!groups || groups.length === 0) {
    return null;
  }

  return (
    <div className="py-2" data-testid="clearify-timeline">
      {groups.map((group, idx) => (
        <ClearifyTimelineItem
          key={getGroupKey(group, idx)}
          group={group}
          index={idx}
          total={groups.length}
          isRunning={isRunning}
          resolvePairedToolStart={resolvePairedToolStart}
        />
      ))}
    </div>
  );
}

function getGroupKey(group: ClearifyTaskGroup, index: number): string {
  const metaTaskId =
    typeof group.clearifyEvent.metadata?.task_id === "string"
      ? group.clearifyEvent.metadata.task_id.trim()
      : "";
  if (metaTaskId) return `clearify:${metaTaskId}`;
  if (group.clearifyEvent.call_id)
    return `clearify:call:${group.clearifyEvent.call_id}`;
  return `clearify:${group.clearifyEvent.timestamp}:${index}`;
}

function ClearifyTimelineItem({
  group,
  index,
  total,
  isRunning,
  resolvePairedToolStart,
}: {
  group: ClearifyTaskGroup;
  index: number;
  total: number;
  isRunning: boolean;
  resolvePairedToolStart: (
    event: AnyAgentEvent,
  ) => WorkflowToolStartedEvent | undefined;
}) {
  const { taskGoalUI, successCriteria, needsUserInput, questionToUser } =
    useMemo(
      () => parseClearifyMetadata(group.clearifyEvent),
      [group.clearifyEvent],
    );

  const isLast = index === total - 1;
  const isActive = isLast && isRunning;

  const [expanded, setExpanded] = useState<boolean>(
    () => isLast || needsUserInput,
  );
  const showTail = index < total - 1 || expanded;

  return (
    <div
      className="relative grid grid-cols-[24px,1fr] gap-x-3 py-1"
      data-testid="event-ui-clearify"
    >
      {/* Timeline gutter */}
      <div
        className="relative flex w-6 justify-center self-stretch"
        aria-hidden="true"
      >
        {index > 0 ? (
          <div className="absolute left-1/2 top-0 h-[14px] w-px -translate-x-1/2 bg-border/50" />
        ) : null}
        {showTail ? (
          <div className="absolute left-1/2 top-[14px] bottom-0 w-px -translate-x-1/2 bg-border/50" />
        ) : null}
        <div
          className={cn(
            "relative z-10 mt-1 flex h-5 w-5 items-center justify-center rounded-full border bg-background",
            needsUserInput
              ? "border-amber-300/60 bg-amber-50/60 text-amber-800 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-100"
              : isActive
                ? "border-primary/50 bg-primary/10 text-primary"
                : "border-border/60 text-muted-foreground",
          )}
        >
          {needsUserInput ? (
            <CircleHelp className="h-3 w-3" />
          ) : isActive ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <Check className="h-3 w-3" />
          )}
        </div>
      </div>

      {/* Content */}
      <div className="min-w-0 flex-1">
        <button
          type="button"
          onClick={() => setExpanded((prev) => !prev)}
          className={cn(
            "group flex w-full items-center justify-between gap-3 rounded-md px-2 py-0.5 text-left",
            "hover:bg-muted/20 transition-colors",
          )}
          aria-expanded={expanded}
        >
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-sm text-foreground truncate">
                {taskGoalUI}
              </span>
              {needsUserInput ? (
                <span className="text-[10px] tracking-wide text-amber-700 dark:text-amber-200">
                  Needs input
                </span>
              ) : null}
              {isActive ? (
                <span className="text-[10px] tracking-wide text-primary">
                  Active
                </span>
              ) : null}
            </div>

            {!expanded && successCriteria.length > 0 ? (
              <div className="mt-0.5 text-xs text-muted-foreground/70 truncate">
                {successCriteria.join(" · ")}
              </div>
            ) : null}
          </div>
        </button>

        {expanded ? (
          <div className="pl-2 pr-1 pt-1 space-y-2">
            {successCriteria.length > 0 ? (
              <ul className="list-disc pl-5 text-xs text-muted-foreground/80 space-y-1">
                {successCriteria.map((crit) => (
                  <li key={crit}>{crit}</li>
                ))}
              </ul>
            ) : null}

            {needsUserInput && questionToUser ? (
              <div className="rounded-md border border-amber-200/60 bg-amber-50/40 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-100">
                {questionToUser}
              </div>
            ) : null}

            {group.events.length > 0 ? (
              <div className="space-y-1">
                {group.events.map((event, idx) => (
                  <EventLine
                    key={`${event.event_type}-${event.timestamp}-${idx}`}
                    event={event}
                    pairedToolStartEvent={resolvePairedToolStart(event) ?? null}
                    variant="nested"
                    showSubagentContext={false}
                  />
                ))}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function parseClearifyMetadata(event: WorkflowToolCompletedEvent): {
  taskGoalUI: string;
  successCriteria: string[];
  needsUserInput: boolean;
  questionToUser: string | null;
} {
  const metadata = event.metadata ?? null;

  const taskGoalUI =
    typeof metadata?.task_goal_ui === "string" && metadata.task_goal_ui.trim()
      ? String(metadata.task_goal_ui).trim()
      : (event.result.split(/\r?\n/)[0]?.trim() ?? "");

  const successCriteria = Array.isArray(metadata?.success_criteria)
    ? (metadata.success_criteria as unknown[])
        .map((item) => (typeof item === "string" ? item.trim() : ""))
        .filter((item) => item.length > 0)
    : [];

  const needsUserInput = metadata?.needs_user_input === true;

  const questionToUser =
    typeof metadata?.question_to_user === "string" &&
    metadata.question_to_user.trim()
      ? String(metadata.question_to_user).trim()
      : null;

  return {
    taskGoalUI,
    successCriteria,
    needsUserInput,
    questionToUser,
  };
}
