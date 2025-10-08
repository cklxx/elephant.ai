'use client';

import { Fragment, ReactNode, useCallback, useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AgentEvent, AnyAgentEvent, ToolCallCompleteEvent } from '@/lib/types';
import { ResearchPlanCard } from './ResearchPlanCard';
import { ConnectionBanner } from './ConnectionBanner';
import { EventList } from './EventList';
import { apiClient } from '@/lib/api';
import {
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  ClipboardList,
  Cpu,
  Info,
  Loader2,
  MessageSquare,
  WifiOff,
  Wrench,
} from 'lucide-react';
import { cn } from '@/lib/utils';

interface TerminalOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
  sessionId: string | null;
  taskId: string | null;
}

type DisplayEvent = AnyAgentEvent | ToolStreamCombinedEvent | ToolCallCompleteDisplayEvent;

interface ToolCallCompleteDisplayEvent extends ToolCallCompleteEvent {
  arguments?: Record<string, unknown>;
}

const EVENT_FILTERS = [
  { id: 'conversation', label: 'Conversation' },
  { id: 'plan', label: 'Planning' },
  { id: 'tools', label: 'Tools' },
  { id: 'system', label: 'System' },
] as const;

type EventFilterId = (typeof EVENT_FILTERS)[number]['id'];

interface ToolStreamCombinedEvent extends AgentEvent {
  event_type: 'tool_stream_combined';
  call_id: string;
  content: string;
  tool_name?: string;
}

export function TerminalOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  sessionId,
  taskId,
}: TerminalOutputProps) {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [activeFilters, setActiveFilters] = useState<Set<EventFilterId>>(
    () => new Set(EVENT_FILTERS.map((filter) => filter.id))
  );

  const displayEvents = useMemo(() => {
    const aggregated: DisplayEvent[] = [];
    let streamBuffer: string[] = [];
    let streamCallId: string | null = null;
    let streamTimestamp: string | null = null;
    let streamAgentLevel: AgentEvent['agent_level'] | null = null;
    const callMetadata = new Map<string, { toolName?: string; arguments?: Record<string, unknown> }>();

    const flushBuffer = () => {
      if (!streamBuffer.length || !streamCallId) {
        streamBuffer = [];
        streamCallId = null;
        streamTimestamp = null;
        streamAgentLevel = null;
        return;
      }

      aggregated.push({
        event_type: 'tool_stream_combined',
        agent_level: streamAgentLevel ?? 'core',
        call_id: streamCallId,
        content: streamBuffer.join(''),
        timestamp: streamTimestamp ?? new Date().toISOString(),
        tool_name: callMetadata.get(streamCallId)?.toolName,
      });

      streamBuffer = [];
      streamCallId = null;
      streamTimestamp = null;
      streamAgentLevel = null;
    };

    events.forEach((event) => {
      if (event.event_type === 'tool_call_start') {
        callMetadata.set(event.call_id, {
          toolName: event.tool_name,
          arguments: event.arguments,
        });
      }

      if (event.event_type === 'tool_call_complete') {
        const existing = callMetadata.get(event.call_id);
        callMetadata.set(event.call_id, {
          toolName: event.tool_name,
          arguments: existing?.arguments,
        });
      }

      if (event.event_type === 'tool_call_stream') {
        if (!streamCallId || streamCallId !== event.call_id) {
          flushBuffer();
          streamCallId = event.call_id;
        }

        streamTimestamp = event.timestamp;
        streamAgentLevel = event.agent_level;
        streamBuffer.push(event.chunk);

        if (event.is_complete) {
          flushBuffer();
        }
        return;
      }

      flushBuffer();
      if (event.event_type === 'tool_call_complete') {
        const metadata = callMetadata.get(event.call_id);
        aggregated.push({
          ...event,
          arguments: metadata?.arguments,
        });
        return;
      }

      aggregated.push(event);
    });

    flushBuffer();
    return aggregated;
  }, [events]);

  const toggleFilter = useCallback((filterId: EventFilterId) => {
    setActiveFilters((prev) => {
      const next = new Set(prev);
      if (next.has(filterId)) {
        if (next.size === 1) {
          return prev;
        }
        next.delete(filterId);
      } else {
        next.add(filterId);
      }

      return next;
    });
  }, []);

  const filteredEvents = useMemo(() => {
    if (activeFilters.size === EVENT_FILTERS.length) {
      return displayEvents;
    }

    return displayEvents.filter((event) => {
      const category = getEventCategory(event);
      if (category === 'conversation' || category === 'plan' || category === 'tools' || category === 'system') {
        return activeFilters.has(category);
      }

      return true;
    });
  }, [displayEvents, activeFilters]);

  const hiddenCount = displayEvents.length - filteredEvents.length;

  // Simple approve plan mutation
  const { mutate: approvePlan } = useMutation({
    mutationFn: async ({ sessionId, taskId }: { sessionId: string; taskId: string }) => {
      return apiClient.approvePlan({
        session_id: sessionId,
        task_id: taskId,
        approved: true,
      });
    },
  });

  // Parse plan state from events
  const { planState, currentPlan } = useMemo(() => {
    const lastPlanEvent = [...events]
      .reverse()
      .find((e) => e.event_type === 'research_plan');

    if (!lastPlanEvent || !('plan_steps' in lastPlanEvent)) {
      return { planState: null, currentPlan: null };
    }

    return {
      planState: 'awaiting_approval' as const,
      currentPlan: {
        goal: 'Research task',
        steps: lastPlanEvent.plan_steps,
        estimated_tools: [],
        estimated_iterations: lastPlanEvent.estimated_iterations,
      },
    };
  }, [events]);

  const handleApprove = () => {
    if (!sessionId || !taskId) return;

    setIsSubmitting(true);
    approvePlan(
      { sessionId, taskId },
      {
        onSuccess: () => {
          setIsSubmitting(false);
        },
        onError: () => {
          setIsSubmitting(false);
        },
      }
    );
  };

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
    <div className="space-y-4" data-testid="terminal-output">
      {/* Plan approval card - if awaiting */}
      {planState === 'awaiting_approval' && currentPlan && (
        <div className="mb-4">
          <ResearchPlanCard
            plan={currentPlan}
            loading={isSubmitting}
            onApprove={handleApprove}
          />
        </div>
      )}

      <div
        className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-100 bg-white/70 px-4 py-3 text-xs text-slate-500"
        data-testid="event-visibility-summary"
      >
        <div className="flex items-center gap-1.5">
          <span className="text-slate-700 font-semibold" data-testid="event-count-visible">
            {filteredEvents.length}
          </span>
          <span>events visible</span>
          {hiddenCount > 0 && (
            <span className="text-slate-400" data-testid="event-count-hidden">
              ({hiddenCount} hidden)
            </span>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2 rounded-full border border-slate-200 bg-slate-50/70 px-1 py-1">
          {EVENT_FILTERS.map((filter) => {
            const isActive = activeFilters.has(filter.id);
            return (
              <button
                key={filter.id}
                type="button"
                onClick={() => toggleFilter(filter.id)}
                aria-pressed={isActive}
                data-testid={`event-filter-${filter.id}`}
                className={cn(
                  'rounded-full px-3 py-1 text-[11px] font-medium transition-all duration-150',
                  isActive
                    ? 'bg-white text-sky-600 shadow-sm shadow-sky-100'
                    : 'text-slate-400 hover:text-slate-600'
                )}
              >
                {filter.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Event stream - terminal style */}
      <div className="space-y-3" data-testid="event-list">
        {filteredEvents.map((event, idx) => (
          <EventLine key={`${event.event_type}-${idx}`} event={event} />
        ))}
      </div>

      {filteredEvents.length === 0 && displayEvents.length > 0 && (
        <div
          className="text-xs text-muted-foreground/80 italic"
          data-testid="event-empty-filters"
        >
          All events are hidden by the current filters.
        </div>
      )}

      {/* Active indicator */}
      {isConnected && events.length > 0 && (
        <div className="flex items-center gap-2 pt-2 text-xs text-slate-400">
          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" />
          <span>Listening for events...</span>
        </div>
      )}
    </div>
  );
}

// Single event line component
function EventLine({ event }: { event: DisplayEvent }) {
  const timestamp = new Date(event.timestamp || Date.now()).toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

  const category = getEventCategory(event);
  const presentation = describeEvent(event);
  const meta = EVENT_STYLE_META[category];
  const anchorId = getAnchorId(event);

  return (
    <article
      className={cn(
        'relative overflow-hidden rounded-2xl border border-slate-100 px-5 py-4 shadow-sm transition-colors',
        'bg-white/90 text-slate-700',
        meta.card,
        presentation.status ? STATUS_VARIANTS[presentation.status] : null,
        anchorId && 'scroll-mt-28 timeline-anchor-target'
      )}
      data-testid={`event-line-${event.event_type}`}
      data-category={category}
      data-anchor-id={anchorId ?? undefined}
      id={anchorId ? `event-${anchorId}` : undefined}
      tabIndex={anchorId ? -1 : undefined}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div
            className={cn(
              'mt-0.5 flex h-9 w-9 items-center justify-center rounded-xl border border-slate-100 bg-slate-50 text-sm text-sky-600',
              meta.iconWrapper
            )}
          >
            <meta.icon className="h-4 w-4" />
          </div>
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className={cn('text-sm font-semibold tracking-tight text-slate-700', meta.headline)}>{presentation.headline}</p>
              <span
                className={cn(
                  'rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide',
                  meta.pill
                )}
              >
                {meta.label}
              </span>
              {presentation.status && (
                <StatusBadge status={presentation.status} />
              )}
            </div>
            {presentation.subheading && (
              <p className="text-[11px] text-slate-400">{presentation.subheading}</p>
            )}
          </div>
        </div>
        <time className="text-[11px] font-medium uppercase tracking-wide text-slate-400">
          {timestamp}
        </time>
      </div>

      {presentation.summary && (
        <div className="mt-3 whitespace-pre-wrap text-xs leading-relaxed text-slate-500">
          {presentation.summary}
        </div>
      )}

      {presentation.supplementary}

      <EventMetadata event={event} />
    </article>
  );
}

function isCombinedStreamEvent(event: DisplayEvent): event is ToolStreamCombinedEvent {
  return event.event_type === 'tool_stream_combined';
}

function getAnchorId(event: DisplayEvent): string | null {
  switch (event.event_type) {
    case 'step_started':
    case 'step_completed':
      return typeof event.step_index === 'number'
        ? `step-${event.step_index}`
        : null;
    case 'iteration_start':
    case 'iteration_complete':
      return typeof (event as any).iteration === 'number'
        ? `iteration-${(event as any).iteration}`
        : null;
    case 'error':
      return typeof (event as any).iteration === 'number'
        ? `iteration-${(event as any).iteration}`
        : null;
    default:
      return null;
  }
}

// Helper functions
function getEventCategory(event: DisplayEvent): EventFilterId | 'other' {
  switch (event.event_type) {
    case 'user_task':
    case 'thinking':
    case 'think_complete':
    case 'task_complete':
      return 'conversation';
    case 'task_analysis':
    case 'research_plan':
    case 'step_started':
    case 'step_completed':
      return 'plan';
    case 'tool_call_start':
    case 'tool_call_complete':
    case 'tool_call_stream':
    case 'tool_stream_combined':
    case 'browser_snapshot':
      return 'tools';
    case 'iteration_start':
    case 'iteration_complete':
    case 'error':
      return 'system';
    default:
      return 'other';
  }
}

type EventStatus = 'success' | 'warning' | 'danger' | 'info';

interface EventPresentation {
  headline: string;
  subheading?: string;
  summary?: ReactNode;
  supplementary?: ReactNode;
  status?: EventStatus;
}

const EVENT_STYLE_META: Record<
  EventFilterId | 'other',
  {
    icon: typeof MessageSquare;
    card: string;
    iconWrapper: string;
    pill: string;
    headline: string;
    label: string;
  }
> = {
  conversation: {
    icon: MessageSquare,
    card: 'border-sky-200/80 bg-sky-50/70 dark:border-sky-500/30 dark:bg-sky-500/5',
    iconWrapper: 'border-sky-200/80 bg-sky-100 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-100',
    pill: 'bg-sky-100 text-sky-700 dark:bg-sky-500/20 dark:text-sky-100',
    headline: 'text-sky-900 dark:text-sky-100',
    label: 'Conversation',
  },
  plan: {
    icon: ClipboardList,
    card: 'border-amber-200/80 bg-amber-50/70 dark:border-amber-500/25 dark:bg-amber-500/5',
    iconWrapper: 'border-amber-200/80 bg-amber-100 text-amber-700 dark:border-amber-500/25 dark:bg-amber-500/10 dark:text-amber-100',
    pill: 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-100',
    headline: 'text-amber-900 dark:text-amber-50',
    label: 'Planning',
  },
  tools: {
    icon: Wrench,
    card: 'border-emerald-200/80 bg-emerald-50/70 dark:border-emerald-500/25 dark:bg-emerald-500/5',
    iconWrapper:
      'border-emerald-200/80 bg-emerald-100 text-emerald-700 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-100',
    pill: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-100',
    headline: 'text-emerald-900 dark:text-emerald-50',
    label: 'Tools',
  },
  system: {
    icon: Cpu,
    card: 'border-slate-200/80 bg-slate-50/80 dark:border-slate-600/40 dark:bg-slate-900/40',
    iconWrapper: 'border-slate-200/80 bg-slate-100 text-slate-700 dark:border-slate-600/40 dark:bg-slate-800/60 dark:text-slate-200',
    pill: 'bg-slate-100 text-slate-700 dark:bg-slate-800/80 dark:text-slate-100',
    headline: 'text-slate-900 dark:text-slate-100',
    label: 'System',
  },
  other: {
    icon: Info,
    card: 'border-border bg-card/90',
    iconWrapper: 'border-border bg-muted text-muted-foreground',
    pill: 'bg-muted text-muted-foreground',
    headline: 'text-foreground',
    label: 'Other',
  },
};

const STATUS_VARIANTS: Record<EventStatus, string> = {
  success: 'border-emerald-400/50 bg-emerald-50/90 dark:border-emerald-500/40 dark:bg-emerald-500/10',
  warning: 'border-amber-400/60 bg-amber-50/80 dark:border-amber-500/40 dark:bg-amber-500/10',
  danger: 'border-destructive/40 bg-destructive/10 dark:border-destructive/40 dark:bg-destructive/20',
  info: 'border-primary/40 bg-primary/10 dark:border-primary/40 dark:bg-primary/20',
};

function describeEvent(event: DisplayEvent): EventPresentation {
  switch (event.event_type) {
    case 'user_task':
      if ('task' in event) {
        return {
          headline: 'User Task',
          subheading: 'Initiated by you',
          summary: <strong className="font-semibold text-foreground">{event.task}</strong>,
        };
      }
      return { headline: 'User Task' };

    case 'task_analysis':
      return {
        headline: event.action_name,
        subheading: 'Task Analysis',
        summary: event.goal,
      };

    case 'iteration_start':
      return {
        headline: `Iteration ${event.iteration} Started`,
        subheading: `Total iterations: ${event.total_iters}`,
        status: 'info',
      };

    case 'thinking':
      return {
        headline: 'Model Thinking',
        subheading: `Iteration ${event.iteration}`,
        summary: `Streaming response chunk ${event.message_count}`,
      };

    case 'think_complete':
      return {
        headline: 'Response Ready',
        subheading: `Iteration ${event.iteration}`,
        summary: truncateText(event.content, 220),
        supplementary: (
          <ContentBlock title="Model Response">
            <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-foreground/90">
              {event.content}
            </pre>
          </ContentBlock>
        ),
      };

    case 'tool_call_start':
      return {
        headline: `${event.tool_name} Started`,
        subheading: `Call ${event.call_id}`,
        summary: formatArgumentsPreview(event.arguments),
        supplementary: <ToolArguments callId={event.call_id} args={event.arguments} />,
      };

    case 'tool_stream_combined':
      if (isCombinedStreamEvent(event)) {
        return {
          headline: `${event.tool_name ?? 'Tool'} Output`,
          subheading: `Call ${event.call_id}`,
          summary: truncateText(event.content.trim(), 240),
          supplementary: (
            <ContentBlock title="Live Output" dataTestId={`tool-call-stream-${event.call_id}`}>
              <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-foreground/90">
                {event.content.trim()}
              </pre>
            </ContentBlock>
          ),
        };
      }
      return { headline: 'Tool Output' };

    case 'tool_call_complete':
      return {
        headline: event.error ? `${event.tool_name} Failed` : `${event.tool_name} Completed`,
        subheading: `Call ${event.call_id} • ${formatDuration(event.duration)}`,
        status: event.error ? 'danger' : 'success',
        summary: event.error ? event.error : formatResultPreview(event.result),
        supplementary: (
          <ToolResult
            callId={event.call_id}
            result={event.result}
            error={event.error}
            toolName={event.tool_name}
          />
        ),
      };

    case 'iteration_complete':
      return {
        headline: `Iteration ${event.iteration} Complete`,
        subheading: `${event.tools_run} tools • ${event.tokens_used.toLocaleString()} tokens`,
        status: 'info',
      };

    case 'task_complete':
      return {
        headline: 'Task Complete',
        subheading: `Duration ${formatDuration(event.duration)} • ${event.total_iterations} iterations`,
        status: 'success',
        summary: truncateText(event.final_answer, 240),
        supplementary: (
          <ContentBlock title="Final Answer">
            <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-foreground/90">
              {event.final_answer}
            </pre>
          </ContentBlock>
        ),
      };

    case 'error':
      return {
        headline: 'Execution Error',
        subheading: `Phase: ${event.phase} • Iteration ${event.iteration}`,
        status: 'danger',
        summary: event.error,
      };

    case 'research_plan':
      return {
        headline: 'Research Plan Drafted',
        subheading: `${event.plan_steps.length} steps • ≈${event.estimated_iterations} iterations`,
        supplementary: (
          <ol className="mt-3 list-decimal space-y-1 pl-4 text-xs text-muted-foreground/90">
            {event.plan_steps.map((step, index) => (
              <li key={index}>{step}</li>
            ))}
          </ol>
        ),
      };

    case 'step_started':
      return {
        headline: `Step ${event.step_index + 1} Started`,
        subheading: 'Execution Plan',
        summary: event.step_description,
      };

    case 'step_completed':
      return {
        headline: `Step ${event.step_index + 1} Completed`,
        subheading: 'Execution Plan',
        status: 'info',
        summary: truncateText(event.step_result, 200),
      };

    case 'browser_snapshot':
      return {
        headline: 'Browser Snapshot',
        subheading: event.url,
        supplementary:
          event.html_preview ? (
            <ContentBlock title="HTML Preview">
              <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-foreground/90">
                {event.html_preview}
              </pre>
            </ContentBlock>
          ) : undefined,
      };

    default:
      return {
        headline: formatHeadline(event.event_type),
        summary: JSON.stringify(event, null, 2),
      };
  }
}

function EventMetadata({ event }: { event: DisplayEvent }) {
  const entries = getEventMetadata(event);
  if (!entries.length) return null;

  return (
    <dl className="mt-3 grid grid-cols-1 gap-2 text-[11px] text-muted-foreground/80 sm:grid-cols-2">
      {entries.map(({ label, value }) => (
        <Fragment key={`${event.timestamp}-${label}`}>
          <dt className="font-medium uppercase tracking-wide">{label}</dt>
          <dd className="text-foreground/80 dark:text-foreground/90">{value}</dd>
        </Fragment>
      ))}
    </dl>
  );
}

function getEventMetadata(event: DisplayEvent): Array<{ label: string; value: string }> {
  switch (event.event_type) {
    case 'tool_call_start':
      return [
        { label: 'Call ID', value: event.call_id },
        { label: 'Tool', value: event.tool_name },
      ];
    case 'tool_call_complete':
      return [
        { label: 'Call ID', value: event.call_id },
        { label: 'Duration', value: formatDuration(event.duration) },
      ];
    case 'iteration_complete':
      return [
        { label: 'Tokens Used', value: event.tokens_used.toLocaleString() },
        { label: 'Tools Run', value: event.tools_run.toString() },
      ];
    case 'task_complete':
      return [
        { label: 'Total Tokens', value: event.total_tokens.toLocaleString() },
        { label: 'Stop Reason', value: event.stop_reason },
      ];
    case 'error':
      return [
        { label: 'Recoverable', value: event.recoverable ? 'Yes' : 'No' },
      ];
    case 'browser_snapshot':
      return event.url
        ? [
            {
              label: 'URL',
              value: event.url,
            },
          ]
        : [];
    default:
      return [];
  }
}

function ToolArguments({ args, callId }: { args?: Record<string, any> | string; callId: string }) {
  if (!args || (typeof args === 'object' && Object.keys(args).length === 0)) {
    return null;
  }

  const formatted = typeof args === 'string' ? args : JSON.stringify(args, null, 2);

  return (
    <ContentBlock
      title="Input Arguments"
      tone="emerald"
      dataTestId={`tool-call-arguments-${callId}`}
    >
      <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-emerald-700 dark:text-emerald-200">
        {formatted}
      </pre>
    </ContentBlock>
  );
}

function ToolResult({
  result,
  error,
  callId,
  toolName,
}: {
  result: any;
  error?: string;
  callId: string;
  toolName: string;
}) {
  if (error) {
    return (
      <ContentBlock title="Error Output" tone="destructive" dataTestId={`tool-call-result-${callId}`}>
        <p className="text-xs font-medium text-destructive dark:text-destructive/80">{error}</p>
      </ContentBlock>
    );
  }

  if (!result) return null;

  const formatted = typeof result === 'string' ? result : JSON.stringify(result, null, 2);

  return (
    <ContentBlock
      title={`${toolName} Result`}
      tone="emerald"
      dataTestId={`tool-call-result-${callId}`}
    >
      <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-emerald-700 dark:text-emerald-200">
        {formatted}
      </pre>
    </ContentBlock>
  );
}

function ContentBlock({
  title,
  children,
  tone = 'slate',
  dataTestId,
}: {
  title: string;
  children: ReactNode;
  tone?: 'emerald' | 'slate' | 'destructive';
  dataTestId?: string;
}) {
  const toneClasses = {
    emerald:
      'border-emerald-300/60 bg-emerald-50/80 text-emerald-900 dark:border-emerald-500/40 dark:bg-emerald-500/10 dark:text-emerald-100',
    slate:
      'border-slate-200/80 bg-slate-50/80 text-slate-900 dark:border-slate-600/40 dark:bg-slate-900/40 dark:text-slate-100',
    destructive:
      'border-destructive/40 bg-destructive/10 text-destructive dark:border-destructive/40 dark:bg-destructive/20 dark:text-destructive/80',
  } as const;

  return (
    <div
      className={cn(
        'mt-3 rounded-md border px-3 py-2 text-xs shadow-inner transition-colors',
        toneClasses[tone]
      )}
      data-testid={dataTestId}
    >
      <p className="text-[11px] font-semibold uppercase tracking-wide opacity-80">{title}</p>
      <div className="mt-1.5">{children}</div>
    </div>
  );
}

function StatusBadge({ status }: { status: EventStatus }) {
  const config: Record<EventStatus, { icon: typeof CheckCircle2; label: string; className: string }> = {
    success: {
      icon: CheckCircle2,
      label: 'Success',
      className: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-100',
    },
    warning: {
      icon: AlertTriangle,
      label: 'Warning',
      className: 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-100',
    },
    danger: {
      icon: AlertTriangle,
      label: 'Error',
      className: 'bg-destructive/10 text-destructive dark:bg-destructive/20 dark:text-destructive/80',
    },
    info: {
      icon: Info,
      label: 'Info',
      className: 'bg-sky-100 text-sky-700 dark:bg-sky-500/20 dark:text-sky-100',
    },
  };

  const meta = config[status];
  const Icon = meta.icon;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide',
        meta.className
      )}
    >
      <Icon className="h-3 w-3" />
      {meta.label}
    </span>
  );
}

function formatHeadline(value: string) {
  return value
    .split('_')
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(' ');
}

function truncateText(value: string, length: number) {
  if (value.length <= length) return value;
  return `${value.slice(0, length)}…`;
}

function formatDuration(durationMs: number) {
  if (!Number.isFinite(durationMs)) return '—';
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

function formatArgumentsPreview(args: Record<string, any> | string | undefined) {
  if (!args) {
    return undefined;
  }

  if (typeof args === 'string') {
    return truncateText(args, 120);
  }

  const entries = Object.entries(args).map(([key, value]) => `${key}: ${String(value)}`);
  return truncateText(entries.join(', '), 120);
}

function formatResultPreview(result: any) {
  if (!result) return undefined;
  if (typeof result === 'string') return truncateText(result, 160);
  if (typeof result === 'object') {
    if ('output' in result) {
      return truncateText(String(result.output), 160);
    }
    if ('content' in result) {
      return truncateText(String(result.content), 160);
    }
  }
  return truncateText(JSON.stringify(result), 160);
}
