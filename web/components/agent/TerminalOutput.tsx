'use client';

import { ReactNode, useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AnyAgentEvent, ToolCallStartEvent } from '@/lib/types';
import { ResearchPlanCard } from './ResearchPlanCard';
import { ConnectionBanner } from './ConnectionBanner';
import { apiClient } from '@/lib/api';
import {
  AlertTriangle,
  CheckCircle2,
  ClipboardList,
  Cpu,
  Info,
  Loader2,
  MessageSquare,
  Wrench,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { TranslationKey, TranslationParams, useI18n } from '@/lib/i18n';

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

type DisplayEvent = AnyAgentEvent | ToolCallStartDisplayEvent;

type EventCategory = 'conversation' | 'plan' | 'tools' | 'system' | 'other';

interface ToolCallStartDisplayEvent extends ToolCallStartEvent {
  call_status: 'running' | 'complete' | 'error';
  completion_result?: string;
  completion_error?: string;
  completion_duration?: number;
  stream_content?: string;
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
  const { t } = useI18n();

  const displayEvents = useMemo(() => {
    const aggregated: DisplayEvent[] = [];
    const startEvents = new Map<string, ToolCallStartDisplayEvent>();

    events.forEach((event) => {
      if (event.event_type === 'tool_call_start') {
        const startEvent: ToolCallStartDisplayEvent = {
          ...event,
          call_status: 'running',
          stream_content: '',
        };
        aggregated.push(startEvent);
        startEvents.set(event.call_id, startEvent);
        return;
      }

      if (event.event_type === 'tool_call_stream') {
        const startEvent = startEvents.get(event.call_id);
        if (startEvent) {
          startEvent.stream_content = `${startEvent.stream_content ?? ''}${event.chunk}`;
        }
        return;
      }

      if (event.event_type === 'tool_call_complete') {
        const startEvent = startEvents.get(event.call_id);
        if (startEvent) {
          startEvent.call_status = event.error ? 'error' : 'complete';
          startEvent.completion_result = event.result;
          startEvent.completion_error = event.error;
          startEvent.completion_duration = event.duration;
          startEvents.delete(event.call_id);
          return;
        }
      }

      aggregated.push(event);
    });

    return aggregated;
  }, [events]);

  const activeAction = useMemo(() => {
    for (let index = displayEvents.length - 1; index >= 0; index -= 1) {
      const candidate = displayEvents[index];
      if (isToolCallStartDisplayEvent(candidate) && candidate.call_status === 'running') {
        return candidate;
      }
    }
    return null;
  }, [displayEvents]);

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
    <div className="space-y-4" data-testid="conversation-stream">
      {planState === 'awaiting_approval' && currentPlan && (
        <div className="max-w-xl">
          <ResearchPlanCard plan={currentPlan} loading={isSubmitting} onApprove={handleApprove} />
        </div>
      )}

      {activeAction && (
        <div className="inline-flex items-center gap-2 rounded-full border border-sky-200/70 bg-sky-50/80 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.3em] text-sky-600">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          <span>{t('conversation.status.doing')}</span>
          <span className="text-slate-500 normal-case tracking-normal">{activeAction.tool_name}</span>
        </div>
      )}

      <div className="space-y-3" data-testid="conversation-events">
        {displayEvents.map((event, index) => (
          <EventLine key={`${event.event_type}-${index}`} event={event} t={t} />
        ))}
      </div>

      {isConnected && displayEvents.length > 0 && (
        <div className="flex items-center gap-2 pt-2 text-xs text-slate-400">
          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" />
          <span>{t('conversation.status.listening')}</span>
        </div>
      )}
    </div>
  );
}

// Single event line component
function EventLine({
  event,
  t,
}: {
  event: DisplayEvent;
  t: (key: TranslationKey, params?: TranslationParams) => string;
}) {
  if (event.event_type === 'user_task') {
    const timestamp = formatTimestamp(event.timestamp);
    return (
      <div className="flex justify-end" data-testid="event-line-user">
        <div className="max-w-xl rounded-3xl bg-sky-500 px-4 py-3 text-sm font-medium text-white shadow-sm">
          <p className="whitespace-pre-wrap leading-relaxed">{event.task}</p>
          <time className="mt-2 block text-[10px] font-medium uppercase tracking-[0.3em] text-white/70">
            {timestamp}
          </time>
        </div>
      </div>
    );
  }

  const timestamp = formatTimestamp(event.timestamp);
  const category = getEventCategory(event);
  const presentation = describeEvent(event);
  const meta = EVENT_STYLE_META[category];
  const anchorId = getAnchorId(event);

  let statusLabel = presentation.statusLabel;
  if (isToolCallStartDisplayEvent(event)) {
    statusLabel =
      event.call_status === 'running'
        ? t('conversation.status.doing')
        : event.call_status === 'error'
          ? t('conversation.status.failed')
          : t('conversation.status.completed');
  }

  const isRunningTool = isToolCallStartDisplayEvent(event) && event.call_status === 'running';

  return (
    <article
      className={cn(
        'group relative max-w-3xl rounded-3xl border border-slate-100/70 bg-white/80 px-4 py-3 text-slate-700 shadow-sm transition sm:px-5',
        meta.card,
        presentation.status ? STATUS_VARIANTS[presentation.status] : null,
        anchorId && 'timeline-anchor-target scroll-mt-28',
        isRunningTool && 'ring-1 ring-sky-300'
      )}
      data-testid={`event-line-${event.event_type}`}
      data-category={category}
      data-anchor-id={anchorId ?? undefined}
      id={anchorId ? `event-${anchorId}` : undefined}
      tabIndex={anchorId ? -1 : undefined}
    >
      <div className="flex items-start gap-3">
        <div
          className={cn(
            'mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-2xl bg-white text-sky-500 shadow',
            meta.iconWrapper
          )}
        >
          <meta.icon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <p className={cn('text-sm font-semibold text-slate-800', meta.headline)}>{presentation.headline}</p>
            <span
              className={cn(
                'inline-flex items-center rounded-full px-2 py-0.5 text-[9px] font-semibold uppercase tracking-[0.25em]',
                meta.pill
              )}
            >
              {meta.label}
            </span>
            {presentation.status && <StatusBadge status={presentation.status} label={statusLabel} />}
          </div>
          {presentation.subheading && (
            <p className="console-microcopy text-slate-400">{presentation.subheading}</p>
          )}

          {isToolCallStartDisplayEvent(event) ? (
            <ToolCallContent event={event} statusLabel={statusLabel} />
          ) : (
            <>
              {presentation.summary && (
                <div className="whitespace-pre-line text-sm leading-relaxed text-slate-600">
                  {presentation.summary}
                </div>
              )}
              {presentation.supplementary}
            </>
          )}

          {!isToolCallStartDisplayEvent(event) && <EventMetadata event={event} />}

          <time className="block text-[10px] font-medium uppercase tracking-[0.3em] text-slate-300">
            {timestamp}
          </time>
        </div>
      </div>
    </article>
  );
}

function ToolCallContent({
  event,
  statusLabel,
}: {
  event: ToolCallStartDisplayEvent;
  statusLabel?: string;
}) {
  const argsPreview = formatArgumentsPreview(event.arguments);
  const hasArgsPreview = Boolean(argsPreview);
  const hasStream = Boolean(event.stream_content && event.stream_content.trim().length > 0);
  const hasResult = Boolean(event.completion_result && String(event.completion_result).trim().length > 0);
  const hasError = Boolean(event.completion_error);
  const metadata = <EventMetadata event={event} />;
  const hasDuration = Boolean(event.completion_duration);

  return (
    <div className="space-y-2">
      {statusLabel && (
        <p
          className={cn(
            'text-sm font-medium',
            event.call_status === 'error' ? 'text-destructive' : 'text-slate-600'
          )}
        >
          {statusLabel}
        </p>
      )}

      {hasArgsPreview && (
        <p className="console-microcopy text-slate-400">{argsPreview}</p>
      )}

      <ToolArguments args={event.arguments} callId={event.call_id} />

      {hasStream && (
        <ContentBlock title="Live Output" dataTestId={`tool-call-stream-${event.call_id}`}>
          <pre className="whitespace-pre-wrap font-mono text-[10px] leading-snug text-foreground/90">
            {event.stream_content?.trim()}
          </pre>
        </ContentBlock>
      )}

      {(hasResult || hasError) && (
        <ToolResult
          callId={event.call_id}
          result={event.completion_result}
          error={event.completion_error}
          toolName={event.tool_name}
        />
      )}

      {(metadata || hasDuration) && (
        <div className="space-y-2">
          {metadata}
          {hasDuration && (
            <p className="console-microcopy text-slate-400">
              {`Duration ${formatDuration(event.completion_duration!)}`}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function isToolCallStartDisplayEvent(event: DisplayEvent): event is ToolCallStartDisplayEvent {
  return event.event_type === 'tool_call_start';
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
function getEventCategory(event: DisplayEvent): EventCategory {
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

type EventStatus = 'success' | 'warning' | 'danger' | 'info' | 'pending';

interface EventPresentation {
  headline: string;
  subheading?: string;
  summary?: ReactNode;
  supplementary?: ReactNode;
  status?: EventStatus;
  statusLabel?: string;
}

const EVENT_STYLE_META: Record<
  EventCategory,
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
  pending: 'border-sky-300/50 bg-sky-50/80 dark:border-sky-500/40 dark:bg-sky-500/10',
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
            <pre className="whitespace-pre-wrap font-mono text-[10px] leading-snug text-foreground/90">
              {event.content}
            </pre>
          </ContentBlock>
        ),
      };

    case 'tool_call_start': {
      const startEvent = event as ToolCallStartDisplayEvent;
      const status: EventStatus =
        startEvent.call_status === 'running'
          ? 'pending'
          : startEvent.call_status === 'error'
            ? 'danger'
            : 'success';

      return {
        headline: `${startEvent.tool_name}`,
        subheading: `Call ${startEvent.call_id}`,
        status,
      };
    }

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
          <ContentBlock title="Final Answer" scrollable={false}>
            <pre className="whitespace-pre-wrap font-mono text-[10px] leading-snug text-foreground/90">
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
              <pre className="whitespace-pre-wrap font-mono text-[10px] leading-snug text-foreground/90">
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
    <div className="mt-2 flex flex-wrap gap-1.5">
      {entries.map(({ label, value }) => (
        <span
          key={`${event.timestamp}-${label}`}
          className="inline-flex items-center gap-1 rounded-full bg-slate-100/80 px-2 py-0.5 text-[9px] font-medium uppercase tracking-[0.25em] text-slate-400"
        >
          <span>{label}</span>
          <span className="text-slate-600 normal-case tracking-normal">{value}</span>
        </span>
      ))}
    </div>
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
      <pre className="whitespace-pre-wrap font-mono text-[10px] leading-snug text-emerald-700 dark:text-emerald-200">
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
      <pre className="whitespace-pre-wrap font-mono text-[10px] leading-snug text-emerald-700 dark:text-emerald-200">
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
  scrollable = true,
}: {
  title: string;
  children: ReactNode;
  tone?: 'emerald' | 'slate' | 'destructive';
  dataTestId?: string;
  scrollable?: boolean;
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
        'mt-2 rounded-xl border px-3 py-2 text-[11px] shadow-inner transition-colors sm:px-4',
        toneClasses[tone],
        scrollable && 'console-scrollbar max-h-36 overflow-y-auto pr-1'
      )}
      data-testid={dataTestId}
    >
      <p className="text-[10px] font-semibold uppercase tracking-[0.25em] text-slate-500/80">{title}</p>
      <div className="mt-1.5 space-y-1 text-xs leading-snug text-slate-600">
        {children}
      </div>
    </div>
  );
}

function StatusBadge({ status, label }: { status: EventStatus; label?: string }) {
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
    pending: {
      icon: Loader2,
      label: 'Pending',
      className: 'bg-sky-100 text-sky-600 dark:bg-sky-500/20 dark:text-sky-100',
    },
  };

  const meta = config[status];
  const Icon = meta.icon;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[9px] font-semibold uppercase tracking-[0.25em]',
        meta.className
      )}
    >
      <Icon className="h-3 w-3" />
      {label ?? meta.label}
    </span>
  );
}

function formatTimestamp(timestamp?: string) {
  const value = timestamp ? new Date(timestamp) : new Date();
  return value.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
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
