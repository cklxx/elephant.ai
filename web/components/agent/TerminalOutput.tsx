"use client";

import { ReactNode, useMemo, useState } from "react";
import { AnyAgentEvent, ToolCallStartEvent } from "@/lib/types";
import { ConnectionBanner } from "./ConnectionBanner";
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Info,
  Loader2,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  getLanguageLocale,
  TranslationKey,
  TranslationParams,
  useI18n,
} from "@/lib/i18n";
import { SandboxLevel, ToolCallSummary } from "@/lib/eventAggregation";
import { MarkdownRenderer } from "@/components/ui/markdown";

interface TerminalOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  toolSummaries?: ToolCallSummary[];
}

type DisplayEvent = AnyAgentEvent | ToolCallStartDisplayEvent;

type EventCategory = "conversation" | "plan" | "tools" | "system" | "other";

interface ToolCallStartDisplayEvent extends ToolCallStartEvent {
  call_status: "running" | "complete" | "error";
  completion_result?: string;
  completion_error?: string;
  completion_duration?: number;
  completion_timestamp?: string;
  stream_content?: string;
  last_stream_timestamp?: string;
}

export function TerminalOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  toolSummaries = [],
}: TerminalOutputProps) {
  const { t, language } = useI18n();
  const locale = getLanguageLocale(language);
  const toolSummariesById = useMemo(
    () => new Map(toolSummaries.map((summary) => [summary.callId, summary])),
    [toolSummaries],
  );

  const displayEvents = useMemo(() => {
    const aggregated: DisplayEvent[] = [];
    const startEvents = new Map<string, ToolCallStartDisplayEvent>();

    events.forEach((event) => {
      // Filter out noise events - only show meaningful results
      if (shouldSkipEvent(event)) {
        return;
      }
      if (event.event_type === "tool_call_start") {
        const startEvent: ToolCallStartDisplayEvent = {
          ...event,
          call_status: "running",
          stream_content: "",
        };
        aggregated.push(startEvent);
        startEvents.set(event.call_id, startEvent);
        return;
      }

      if (event.event_type === "tool_call_stream") {
        const startEvent = startEvents.get(event.call_id);
        if (startEvent) {
          startEvent.stream_content = `${startEvent.stream_content ?? ""}${event.chunk}`;
          startEvent.last_stream_timestamp = event.timestamp;
        }
        return;
      }

      if (event.event_type === "tool_call_complete") {
        const startEvent = startEvents.get(event.call_id);
        if (startEvent) {
          startEvent.call_status = event.error ? "error" : "complete";
          startEvent.completion_result = event.result;
          startEvent.completion_error = event.error;
          startEvent.completion_duration = event.duration;
          startEvent.completion_timestamp = event.timestamp;
          startEvents.delete(event.call_id);
          return;
        }
      }

      aggregated.push(event);
    });

    const toolEventsToKeep = new Set<number>();
    let hasSettledToolCall = false;

    for (let index = aggregated.length - 1; index >= 0; index -= 1) {
      const event = aggregated[index];

      if (!isToolCallStartDisplayEvent(event)) {
        toolEventsToKeep.add(index);
        continue;
      }

      if (event.call_status === "running") {
        toolEventsToKeep.add(index);
        continue;
      }

      if (!hasSettledToolCall) {
        toolEventsToKeep.add(index);
        hasSettledToolCall = true;
      }
    }

    return aggregated.filter((_, index) => toolEventsToKeep.has(index));
  }, [events]);

  const activeAction = useMemo(() => {
    for (let index = displayEvents.length - 1; index >= 0; index -= 1) {
      const candidate = displayEvents[index];
      if (
        isToolCallStartDisplayEvent(candidate) &&
        candidate.call_status === "running"
      ) {
        return candidate;
      }
    }
    return null;
  }, [displayEvents]);

  // Show connection banner if disconnected
  if (!isConnected || error) {
    return (
      <ConnectionBanner
        isConnected={isConnected}
        isReconnecting={isReconnecting}
        error={error}
        reconnectAttempts={reconnectAttempts}
        onReconnect={onReconnect}
      />
    );
  }

  return (
    <div
      className="console-card space-y-5 px-6 py-5"
      data-testid="conversation-stream"
    >
      {activeAction && (
        <div className="console-quiet-chip text-xs uppercase">
          <Loader2 className="h-4 w-4 animate-spin" />
          <span>{activeAction.tool_name}</span>
        </div>
      )}

      <div className="space-y-2" data-testid="conversation-events">
        {displayEvents.map((event, index) => {
          // Find the last completed tool call to auto-expand it
          const isLastCompletedToolCall = (() => {
            if (
              !isToolCallStartDisplayEvent(event) ||
              event.call_status === "running"
            ) {
              return false;
            }
            // Check if this is the last completed tool call
            for (let i = displayEvents.length - 1; i > index; i--) {
              const laterEvent = displayEvents[i];
              if (
                isToolCallStartDisplayEvent(laterEvent) &&
                laterEvent.call_status !== "running"
              ) {
                return false;
              }
            }
            return true;
          })();

          return (
            <EventLine
              key={`${event.event_type}-${index}`}
              event={event}
              t={t}
              locale={locale}
              toolSummariesById={toolSummariesById}
              isLastCompletedToolCall={isLastCompletedToolCall}
            />
          );
        })}
      </div>

      {isConnected && displayEvents.length > 0 && (
        <div className="flex items-center gap-2 pt-1 text-xs uppercase tracking-[0.24em] text-muted-foreground">
          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-foreground" />
          <span>{t("conversation.status.listening")}</span>
        </div>
      )}
    </div>
  );
}

function SimpleToolSummaryList({
  summaries,
  t,
  locale,
}: {
  summaries: ToolCallSummary[];
  t: (key: TranslationKey, params?: TranslationParams) => string;
  locale: string;
}) {
  return (
    <section
      className="rounded-md border border-slate-200 bg-white p-3 text-[10px] text-slate-600"
      data-testid="tool-summary-list"
    >
      <p className="text-[10px] font-semibold text-slate-700">
        {t("conversation.tools.simpleSummary.heading")}
      </p>
      <p className="mt-1 text-[10px] text-slate-500">
        {t("conversation.tools.simpleSummary.sandboxNote")}
      </p>
      <ol className="mt-3 space-y-3 text-[9px] leading-relaxed">
        {summaries.map((summary) => {
          const statusLabel =
            summary.status === "running"
              ? t("conversation.status.doing")
              : summary.status === "completed"
                ? t("conversation.status.completed")
                : t("conversation.status.failed");
          const timestamp = formatTimestamp(
            summary.completedAt ?? summary.startedAt,
            locale,
          );
          const duration = summary.durationMs
            ? formatDuration(summary.durationMs)
            : null;

          const details: Array<{ label: string; value: string }> = [];
          if (summary.argumentsPreview) {
            details.push({
              label: t("conversation.tools.simpleSummary.inputsLabel"),
              value: summary.argumentsPreview,
            });
          }
          if (summary.resultPreview) {
            details.push({
              label: t("conversation.tools.simpleSummary.outputLabel"),
              value: summary.resultPreview,
            });
          }
          if (summary.errorMessage) {
            details.push({
              label: t("conversation.tools.simpleSummary.errorLabel"),
              value: summary.errorMessage,
            });
          }

          const sandboxPolicy = describeSandboxPolicy(summary.sandboxLevel, t);

          return (
            <li key={summary.callId} className="space-y-1 text-slate-600">
              <p className="text-[9px] text-slate-700">
                {timestamp} · {summary.toolName} · {statusLabel}
                {duration ? ` · ${duration}` : ""}
              </p>
              <p className="text-[8px] text-slate-500">
                {t("conversation.tools.simpleSummary.sandboxLine", {
                  policy: sandboxPolicy,
                })}
              </p>
              {details.map(({ label, value }, index) => (
                <p
                  key={`${summary.callId}-detail-${index}`}
                  className="text-[8px] text-slate-500"
                >
                  - {label}: {value}
                </p>
              ))}
            </li>
          );
        })}
      </ol>
    </section>
  );
}

function describeSandboxPolicy(
  level: SandboxLevel,
  t: (key: TranslationKey, params?: TranslationParams) => string,
) {
  switch (level) {
    case "filesystem":
      return t("conversation.tools.simpleSummary.policy.filesystem");
    case "system":
      return t("conversation.tools.simpleSummary.policy.system");
    default:
      return t("conversation.tools.simpleSummary.policy.standard");
  }
}

// Collapsible tool call component
function CollapsibleToolCall({
  event,
  isLatest = false,
}: {
  event: ToolCallStartDisplayEvent;
  isLatest?: boolean;
}) {
  const [isExpanded, setIsExpanded] = useState(isLatest);
  const timestamp = formatTimestamp(event.timestamp);
  const hasResult = event.call_status === "complete" && event.completion_result;
  const hasError = event.call_status === "error";
  const isRunning = event.call_status === "running";
  const hasDetails = hasResult || hasError;

  return (
    <div className="flex gap-3 py-1.5 group" data-testid="event-line-tool">
      <div className="flex-1 text-sm">
        <button
          type="button"
          onClick={() => setIsExpanded(!isExpanded)}
          disabled={!hasDetails && !isRunning}
          className={cn(
            "flex items-center gap-2 w-full text-left rounded-md px-2 py-1.5 transition-all",
            hasDetails && "hover:bg-slate-50 cursor-pointer",
            !hasDetails && !isRunning && "cursor-default",
          )}
        >
          {/* Expand/Collapse Icon */}
          {hasDetails && (
            <span className="text-slate-400 group-hover:text-slate-600 transition-colors">
              {isExpanded ? (
                <ChevronDown className="h-3.5 w-3.5" />
              ) : (
                <ChevronRight className="h-3.5 w-3.5" />
              )}
            </span>
          )}

          {/* Status Icon */}
          <span className="flex-shrink-0">
            {isRunning && (
              <Loader2 className="h-3.5 w-3.5 animate-spin text-blue-500" />
            )}
            {hasError && <span className="text-red-600 text-sm">✗</span>}
            {event.call_status === "complete" && !hasError && (
              <span className="text-emerald-600 text-sm">✓</span>
            )}
          </span>

          {/* Tool Name */}
          <span
            className={cn(
              "font-mono font-medium",
              hasError && "text-red-700",
              !hasError && event.call_status === "complete" && "text-slate-700",
              isRunning && "text-blue-700",
            )}
          >
            {event.tool_name}
          </span>

          {/* Arguments Preview */}
          {event.arguments_preview && (
            <span className="text-xs text-slate-500 truncate">
              ({event.arguments_preview})
            </span>
          )}

          {/* Duration Badge */}
          {event.completion_duration && (
            <span className="ml-auto text-[10px] text-slate-400 font-mono">
              {formatDuration(event.completion_duration)}
            </span>
          )}
        </button>

        {/* Expanded Details */}
        {isExpanded && hasDetails && (
          <div className="mt-2 ml-6 space-y-2 animate-in fade-in-0 slide-in-from-top-1 duration-200">
            {hasError && event.completion_error && (
              <div className="text-xs text-red-700 bg-red-50 border border-red-200 rounded-md p-3">
                <div className="font-semibold mb-1.5 flex items-center gap-1.5">
                  <AlertTriangle className="h-3.5 w-3.5" />
                  Error Details
                </div>
                <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed">
                  {event.completion_error}
                </pre>
              </div>
            )}
            {hasResult && (
              <div className="text-xs text-slate-700 bg-slate-50 border border-slate-200 rounded-md p-3 max-h-80 overflow-y-auto console-scrollbar">
                <div className="font-semibold text-slate-600 mb-1.5 flex items-center gap-1.5">
                  <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                  Result
                </div>
                <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-slate-800">
                  {event.completion_result}
                </pre>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// Single event line component
function EventLine({
  event,
  t,
  locale,
  toolSummariesById,
  isLastCompletedToolCall = false,
}: {
  event: DisplayEvent;
  t: (key: TranslationKey, params?: TranslationParams) => string;
  locale: string;
  toolSummariesById: Map<string, ToolCallSummary>;
  isLastCompletedToolCall?: boolean;
}) {
  // Handle tool calls with collapsible component
  if (
    event.event_type === "tool_call_start" ||
    isToolCallStartDisplayEvent(event)
  ) {
    return (
      <CollapsibleToolCall
        event={event as ToolCallStartDisplayEvent}
        isLatest={isLastCompletedToolCall}
      />
    );
  }

  if (event.event_type === "user_task") {
    const timestamp = formatTimestamp(event.timestamp, locale);
    return (
      <div className="flex gap-3 py-2" data-testid="event-line-user">
        <span className="text-slate-400 text-xs flex-shrink-0 select-none font-mono">
          {timestamp}
        </span>
        <div className="text-slate-700 font-semibold text-sm leading-normal flex-1 whitespace-pre-wrap">
          {event.task}
        </div>
      </div>
    );
  }

  const timestamp = formatTimestamp(event.timestamp, locale);
  const category = getEventCategory(event);
  const presentation = describeEvent(event, t);

  if (
    !presentation.headline &&
    !presentation.summary &&
    !presentation.supplementary &&
    !presentation.subheading
  ) {
    return null;
  }

  const meta = EVENT_STYLE_META[category];
  const anchorId = getAnchorId(event);
  const statusLabel = presentation.statusLabel;
  const headlineSize = HEADLINE_SIZES[category];

  return (
    <article
      className={cn(
        "group relative max-w-3xl space-y-1.5 border-l border-slate-200/70 pl-3 text-slate-700",
        anchorId && "timeline-anchor-target scroll-mt-28",
      )}
      data-testid={`event-line-${event.event_type}`}
      data-category={category}
      data-anchor-id={anchorId ?? undefined}
      id={anchorId ? `event-${anchorId}` : undefined}
      tabIndex={anchorId ? -1 : undefined}
    >
      <span
        aria-hidden
        className="absolute -left-[3px] top-2 h-1.5 w-1.5 rounded-full bg-slate-300"
      />
      <div className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-[13px]">
          {presentation.headline && (
            <p
              className={cn(
                "font-semibold leading-tight text-foreground",
                meta.headline,
                headlineSize,
              )}
            >
              {presentation.headline}
            </p>
          )}
          {presentation.status && (
            <StatusBadge status={presentation.status} label={statusLabel} />
          )}
        </div>
        {presentation.subheading && (
          <p className="text-[9px] font-medium uppercase tracking-[0.3em] text-slate-400">
            {presentation.subheading}
          </p>
        )}

        {presentation.summary && (
          <div className="whitespace-pre-line text-xs leading-relaxed text-slate-600">
            {presentation.summary}
          </div>
        )}
        {presentation.supplementary}

        {event.event_type !== "task_complete" && (
          <EventMetadata event={event} accentClass={meta.accent} />
        )}

        {event.event_type !== "task_complete" && (
          <time className="block text-[8px] font-medium uppercase tracking-[0.3em] text-slate-300">
            {timestamp}
          </time>
        )}
      </div>
    </article>
  );
}

function ToolCallContent({
  event,
  t,
  summary,
}: {
  event: ToolCallStartDisplayEvent;
  t: (key: TranslationKey, params?: TranslationParams) => string;
  summary?: ToolCallSummary;
}) {
  const effectiveStatus =
    summary?.status ??
    (event.call_status === "error"
      ? "error"
      : event.call_status === "complete"
        ? "completed"
        : "running");

  const isError = effectiveStatus === "error";
  const resultPreview =
    summary?.resultPreview ?? formatResultPreview(event.completion_result);
  const errorText = summary?.errorMessage ?? event.completion_error;

  if (!resultPreview && !errorText) {
    return null;
  }

  return (
    <div className="space-y-3 text-slate-700">
      {resultPreview && (
        <div className="rounded-md border border-slate-200 bg-white/80 p-3 shadow-sm">
          <pre className="whitespace-pre-wrap text-sm leading-relaxed text-slate-800 sm:text-base">
            {resultPreview}
          </pre>
        </div>
      )}
      {errorText && isError && (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-destructive">
          <pre className="whitespace-pre-wrap text-sm leading-relaxed sm:text-base">
            {errorText}
          </pre>
        </div>
      )}
    </div>
  );
}

function isToolCallStartDisplayEvent(
  event: DisplayEvent,
): event is ToolCallStartDisplayEvent {
  return event.event_type === "tool_call_start";
}

function getAnchorId(event: DisplayEvent): string | null {
  switch (event.event_type) {
    case "step_started":
    case "step_completed":
      return typeof event.step_index === "number"
        ? `step-${event.step_index}`
        : null;
    case "iteration_start":
    case "iteration_complete":
      return typeof (event as any).iteration === "number"
        ? `iteration-${(event as any).iteration}`
        : null;
    case "error":
      return typeof (event as any).iteration === "number"
        ? `iteration-${(event as any).iteration}`
        : null;
    default:
      return null;
  }
}

// Helper functions

/**
 * Filter out noise events that don't provide meaningful information to users
 * Only show key results and important milestones
 */
function shouldSkipEvent(event: AnyAgentEvent): boolean {
  switch (event.event_type) {
    // Show user input
    case "user_task":
    // Show thinking process
    case "thinking":
    case "think_complete":
    // Show task completion
    case "task_complete":
    // Allow tool calls for aggregation (needed for loading tip)
    case "tool_call_start":
    case "tool_call_complete":
    case "tool_call_stream":
      return false;
    // Skip everything else
    default:
      return true;
  }
}

function getEventCategory(event: DisplayEvent): EventCategory {
  switch (event.event_type) {
    case "user_task":
    case "thinking":
    case "think_complete":
    case "task_complete":
      return "conversation";
    case "task_analysis":
    case "research_plan":
    case "step_started":
    case "step_completed":
      return "plan";
    case "tool_call_start":
    case "browser_info":
      return "tools";
    case "iteration_start":
    case "iteration_complete":
    case "error":
      return "system";
    default:
      return "other";
  }
}

type EventStatus = "success" | "warning" | "danger" | "info" | "pending";

interface EventPresentation {
  headline?: string;
  subheading?: string;
  summary?: ReactNode;
  supplementary?: ReactNode;
  status?: EventStatus;
  statusLabel?: string;
}

const EVENT_STYLE_META: Record<
  EventCategory,
  {
    headline: string;
    accent: string;
    label: string;
  }
> = {
  conversation: {
    headline: "text-foreground",
    accent: "text-muted-foreground",
    label: "Conversation",
  },
  plan: {
    headline: "text-foreground",
    accent: "text-muted-foreground",
    label: "Planning",
  },
  tools: {
    headline: "text-foreground",
    accent: "text-muted-foreground",
    label: "Tools",
  },
  system: {
    headline: "text-foreground",
    accent: "text-muted-foreground",
    label: "System",
  },
  other: {
    headline: "text-foreground",
    accent: "text-muted-foreground",
    label: "Other",
  },
};

const HEADLINE_SIZES: Record<EventCategory, string> = {
  conversation: "text-base sm:text-lg",
  plan: "text-sm sm:text-base",
  tools: "text-xs sm:text-sm",
  system: "text-sm",
  other: "text-sm",
};

function describeEvent(
  event: DisplayEvent,
  t: (key: TranslationKey, params?: TranslationParams) => string,
): EventPresentation {
  switch (event.event_type) {
    case "user_task":
      if ("task" in event) {
        return {
          headline: "User Task",
          subheading: "Initiated by you",
          summary: (
            <strong className="font-semibold text-foreground">
              {event.task}
            </strong>
          ),
        };
      }
      return { headline: "User Task" };

    case "task_analysis":
      return {
        headline: event.action_name,
        subheading: "Task Analysis",
        summary: event.goal,
      };

    case "iteration_start":
      return {
        headline: `Iteration ${event.iteration} Started`,
        subheading: `Total iterations: ${event.total_iters}`,
        status: "info",
      };

    case "thinking":
      return {};

    case "think_complete":
      return {
        supplementary: (
          <MarkdownRenderer
            content={event.content}
            className="prose prose-slate max-w-none text-slate-500 leading-relaxed"
            components={{
              code: ({ inline, className, children, ...props }: any) => {
                if (inline) {
                  return (
                    <code
                      className="bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded text-sm font-mono whitespace-nowrap"
                      {...props}
                    >
                      {children}
                    </code>
                  );
                }
                return (
                  <code
                    className="block bg-slate-50 text-slate-600 p-4 rounded-md overflow-x-auto font-mono text-sm leading-relaxed border border-slate-200"
                    {...props}
                  >
                    {children}
                  </code>
                );
              },
              pre: ({ children }: any) => (
                <div className="my-4">{children}</div>
              ),
              p: ({ children }: any) => (
                <p className="mb-4 leading-relaxed text-slate-500">
                  {children}
                </p>
              ),
              ul: ({ children }: any) => (
                <ul className="mb-4 space-y-2 leading-relaxed text-slate-500">
                  {children}
                </ul>
              ),
              ol: ({ children }: any) => (
                <ol className="mb-4 space-y-2 leading-relaxed text-slate-500">
                  {children}
                </ol>
              ),
              li: ({ children }: any) => (
                <li className="leading-relaxed text-slate-500">{children}</li>
              ),
              h1: ({ children }: any) => (
                <h1 className="text-2xl font-bold mb-4 mt-6 leading-tight text-slate-600">
                  {children}
                </h1>
              ),
              h2: ({ children }: any) => (
                <h2 className="text-xl font-bold mb-3 mt-5 leading-tight text-slate-600">
                  {children}
                </h2>
              ),
              h3: ({ children }: any) => (
                <h3 className="text-lg font-bold mb-2 mt-4 leading-tight text-slate-600">
                  {children}
                </h3>
              ),
              strong: ({ children }: any) => (
                <strong className="font-bold text-slate-600">{children}</strong>
              ),
            }}
          />
        ),
      };

    case "tool_call_start": {
      const startEvent = event as ToolCallStartDisplayEvent;
      const status: EventStatus =
        startEvent.call_status === "running"
          ? "pending"
          : startEvent.call_status === "error"
            ? "danger"
            : "success";

      return {
        headline: `${startEvent.tool_name}`,
        subheading: `Call ${startEvent.call_id}`,
        status,
      };
    }

    case "tool_call_complete":
      return {};

    case "iteration_complete":
      return {
        headline: `Iteration ${event.iteration} Complete`,
        subheading: `${event.tools_run} tools • ${event.tokens_used.toLocaleString()} tokens`,
        status: "info",
      };

    case "task_complete":
      return {
        supplementary: event.final_answer ? (
          <MarkdownRenderer
            content={event.final_answer}
            className="prose prose-sm max-w-none text-slate-700"
          />
        ) : undefined,
      };

    case "error":
      return {
        headline: "Execution Error",
        subheading: `Phase: ${event.phase} • Iteration ${event.iteration}`,
        status: "danger",
        summary: event.error,
      };

    case "research_plan":
      return {
        headline: "Research Plan Drafted",
        subheading: `${event.plan_steps.length} steps • ≈${event.estimated_iterations} iterations`,
        supplementary: (
          <ol className="mt-3 list-decimal space-y-1 pl-4 text-xs text-muted-foreground/90">
            {event.plan_steps.map((step, index) => (
              <li key={index}>{step}</li>
            ))}
          </ol>
        ),
      };

    case "step_started":
      return {
        headline: `Step ${event.step_index + 1} Started`,
        subheading: "Execution Plan",
        summary: event.step_description,
      };

    case "step_completed":
      return {
        headline: `Step ${event.step_index + 1} Completed`,
        subheading: "Execution Plan",
        status: "info",
        summary: truncateText(event.step_result, 200),
      };

    case "browser_info": {
      const status =
        typeof event.success === "boolean"
          ? event.success
            ? "info"
            : "warning"
          : "info";
      const details: Array<[string, string]> = [];
      if (event.user_agent) {
        details.push(["User Agent", event.user_agent]);
      }
      if (event.cdp_url) {
        details.push(["CDP URL", event.cdp_url]);
      }
      if (event.vnc_url) {
        details.push(["VNC URL", event.vnc_url]);
      }
      if (event.viewport_width && event.viewport_height) {
        details.push([
          "Viewport",
          `${event.viewport_width} × ${event.viewport_height}`,
        ]);
      }

      const summaryParts: string[] = [];
      if (typeof event.success === "boolean") {
        summaryParts.push(
          event.success ? "Browser ready" : "Browser unavailable",
        );
      }
      if (event.message) {
        summaryParts.push(event.message);
      }

      return {
        headline: "Browser Diagnostics",
        subheading: `Captured ${new Date(event.captured).toLocaleString()}`,
        status,
        summary: summaryParts.length ? summaryParts.join(" • ") : undefined,
        supplementary:
          details.length > 0 ? (
            <ContentBlock title="Details">
              <dl className="mt-2 space-y-1 text-xs text-muted-foreground">
                {details.map(([label, value]) => (
                  <div key={label} className="flex flex-col">
                    <dt className="font-semibold text-slate-600">{label}</dt>
                    <dd className="break-words text-slate-500">{value}</dd>
                  </div>
                ))}
              </dl>
            </ContentBlock>
          ) : undefined,
      };
    }

    default:
      return {
        headline: formatHeadline(event.event_type),
        summary: JSON.stringify(event, null, 2),
      };
  }
}

function EventMetadata({
  event,
  accentClass,
}: {
  event: DisplayEvent;
  accentClass?: string;
}) {
  const entries = getEventMetadata(event);
  if (!entries.length) return null;

  const isToolEvent =
    event.event_type === "tool_call_start" ||
    event.event_type === "tool_call_complete";

  return (
    <dl
      className={cn(
        "flex flex-wrap gap-x-4 gap-y-1 uppercase tracking-[0.25em] text-slate-400",
        isToolEvent ? "text-[8px]" : "text-[9px]",
      )}
    >
      {entries.map(({ label, value }) => (
        <div
          key={`${event.timestamp}-${label}`}
          className="flex items-center gap-2"
        >
          <dt className={cn("font-semibold", accentClass)}>{label}</dt>
          <dd
            className={cn(
              "font-mono tracking-normal text-slate-500",
              isToolEvent ? "text-[8px]" : "text-[10px]",
            )}
          >
            {value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function getEventMetadata(
  event: DisplayEvent,
): Array<{ label: string; value: string }> {
  switch (event.event_type) {
    case "tool_call_start":
      return [
        { label: "Call ID", value: event.call_id },
        { label: "Tool", value: event.tool_name },
      ];
    case "tool_call_complete":
      return [
        { label: "Call ID", value: event.call_id },
        { label: "Duration", value: formatDuration(event.duration) },
      ];
    case "iteration_complete":
      return [
        { label: "Tokens Used", value: event.tokens_used.toLocaleString() },
        { label: "Tools Run", value: event.tools_run.toString() },
      ];
    case "task_complete":
      return [
        { label: "Total Tokens", value: event.total_tokens.toLocaleString() },
        { label: "Stop Reason", value: event.stop_reason },
      ];
    case "error":
      return [
        { label: "Recoverable", value: event.recoverable ? "Yes" : "No" },
      ];
    case "browser_info": {
      const entries: Array<{ label: string; value: string }> = [];
      if (typeof event.success === "boolean") {
        entries.push({
          label: "Status",
          value: event.success ? "Available" : "Unavailable",
        });
      }
      if (event.captured) {
        entries.push({
          label: "Captured",
          value: new Date(event.captured).toLocaleString(),
        });
      }
      return entries;
    }
    default:
      return [];
  }
}

function ContentBlock({
  title,
  children,
  tone = "slate",
  dataTestId,
  scrollable = true,
  variant = "default",
}: {
  title: string;
  children: ReactNode;
  tone?: "emerald" | "slate" | "destructive";
  dataTestId?: string;
  scrollable?: boolean;
  variant?: "default" | "compact";
}) {
  const toneClasses = {
    emerald: "border-emerald-300 text-emerald-600",
    slate: "border-slate-200 text-slate-600",
    destructive: "border-destructive/70 text-destructive",
  } as const;

  const isCompact = variant === "compact";

  return (
    <div
      className={cn(
        "mt-3 space-y-2 border-l-2 pl-3 leading-snug",
        isCompact ? "text-[7px] sm:text-[8px]" : "text-[9px] sm:text-[10px]",
        toneClasses[tone],
        scrollable && "console-scrollbar max-h-36 overflow-y-auto pr-1",
      )}
      data-testid={dataTestId}
    >
      <p
        className={cn(
          "font-semibold uppercase tracking-[0.3em] opacity-70",
          isCompact ? "text-[6px] sm:text-[7px]" : "text-[8px] sm:text-[9px]",
        )}
      >
        {title}
      </p>
      <div
        className={cn(
          "space-y-1",
          isCompact ? "text-[7px] sm:text-[8px]" : "text-[10px] sm:text-[11px]",
        )}
      >
        {children}
      </div>
    </div>
  );
}

function StatusBadge({
  status,
  label,
}: {
  status: EventStatus;
  label?: string;
}) {
  const config: Record<
    EventStatus,
    { icon: typeof CheckCircle2; label: string; className: string }
  > = {
    success: {
      icon: CheckCircle2,
      label: "Success",
      className: "text-emerald-600",
    },
    warning: {
      icon: AlertTriangle,
      label: "Warning",
      className: "text-amber-500",
    },
    danger: {
      icon: AlertTriangle,
      label: "Error",
      className: "text-destructive",
    },
    info: {
      icon: Info,
      label: "Info",
      className: "text-sky-500",
    },
    pending: {
      icon: Loader2,
      label: "Pending",
      className: "text-sky-500",
    },
  };

  const meta = config[status];
  const Icon = meta.icon;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.25em] sm:text-[10px]",
        meta.className,
      )}
    >
      <Icon className="h-3 w-3" />
      {label ?? meta.label}
    </span>
  );
}

function formatTimestamp(timestamp?: string, locale = "en-US") {
  const value = timestamp ? new Date(timestamp) : new Date();
  if (Number.isNaN(value.getTime())) {
    return "";
  }

  return value.toLocaleTimeString(locale, {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatHeadline(value?: string) {
  const normalized = value?.trim();
  if (!normalized) {
    return "Event";
  }

  return normalized
    .split("_")
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function truncateText(value: string, length: number) {
  if (value.length <= length) return value;
  return `${value.slice(0, length)}…`;
}

function formatDuration(durationMs: number) {
  if (!Number.isFinite(durationMs)) return "—";
  if (durationMs < 1000) {
    return `${Math.round(durationMs)} ms`;
  }
  const seconds = durationMs / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(1)} s`;
  }
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds.toFixed(0)}s`;
}

function formatArgumentsPreview(
  args: Record<string, any> | string | undefined,
) {
  if (!args) {
    return undefined;
  }

  if (typeof args === "string") {
    return truncateText(args, 120);
  }

  const entries = Object.entries(args).map(
    ([key, value]) => `${key}: ${String(value)}`,
  );
  return truncateText(entries.join(", "), 120);
}

function formatResultPreview(result: any) {
  if (!result) return undefined;
  if (typeof result === "string") return truncateText(result, 160);
  if (typeof result === "object") {
    if ("output" in result) {
      return truncateText(String(result.output), 160);
    }
    if ("content" in result) {
      return truncateText(String(result.content), 160);
    }
  }
  return truncateText(JSON.stringify(result), 160);
}
