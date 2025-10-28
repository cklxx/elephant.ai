'use client';

import { ReactNode, useMemo } from 'react';
import { AnyAgentEvent, ToolCallStartEvent } from '@/lib/types';
import { ConnectionBanner } from './ConnectionBanner';
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
import { getLanguageLocale, TranslationKey, TranslationParams, useI18n } from '@/lib/i18n';
import { SandboxLevel, ToolCallSummary } from '@/lib/eventAggregation';

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

type EventCategory = 'conversation' | 'plan' | 'tools' | 'system' | 'other';

interface ToolCallStartDisplayEvent extends ToolCallStartEvent {
  call_status: 'running' | 'complete' | 'error';
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
    [toolSummaries]
  );

  const displayEvents = useMemo(() => {
    const aggregated: DisplayEvent[] = [];
    const startEvents = new Map<string, ToolCallStartDisplayEvent>();

    events.forEach((event) => {
      // Filter out noise events - only show meaningful results
      if (shouldSkipEvent(event)) {
        return;
      }
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
          startEvent.last_stream_timestamp = event.timestamp;
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
          startEvent.completion_timestamp = event.timestamp;
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
    <div className="space-y-6" data-testid="conversation-stream">
      {activeAction && (
        <div className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-2.5 py-1 text-[9px] font-semibold uppercase tracking-[0.3em] text-slate-600 shadow-sm">
          <Loader2 className="h-3.5 w-3.5 animate-spin text-slate-500" />
          <span>{t('conversation.status.doing')}</span>
          <span className="text-slate-500 normal-case tracking-normal">{activeAction.tool_name}</span>
        </div>
      )}

      {toolSummaries.length > 0 && (
        <SimpleToolSummaryList summaries={toolSummaries} t={t} locale={locale} />
      )}

      <div className="space-y-6" data-testid="conversation-events">
        {displayEvents.map((event, index) => (
          <EventLine
            key={`${event.event_type}-${index}`}
            event={event}
            t={t}
            locale={locale}
            toolSummariesById={toolSummariesById}
          />
        ))}
      </div>

      {isConnected && displayEvents.length > 0 && (
        <div className="flex items-center gap-2 pt-2 text-xs uppercase tracking-[0.3em] text-slate-400">
          <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" />
          <span>{t('conversation.status.listening')}</span>
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
          {t('conversation.tools.simpleSummary.heading')}
        </p>
        <p className="mt-1 text-[10px] text-slate-500">
          {t('conversation.tools.simpleSummary.sandboxNote')}
        </p>
        <ol className="mt-3 space-y-3 text-[9px] leading-relaxed">
        {summaries.map((summary) => {
          const statusLabel =
            summary.status === 'running'
              ? t('conversation.status.doing')
              : summary.status === 'completed'
                ? t('conversation.status.completed')
                : t('conversation.status.failed');
          const timestamp = formatTimestamp(summary.completedAt ?? summary.startedAt, locale);
          const duration = summary.durationMs ? formatDuration(summary.durationMs) : null;

          const details: Array<{ label: string; value: string }> = [];
          if (summary.argumentsPreview) {
            details.push({
              label: t('conversation.tools.simpleSummary.inputsLabel'),
              value: summary.argumentsPreview,
            });
          }
          if (summary.resultPreview) {
            details.push({
              label: t('conversation.tools.simpleSummary.outputLabel'),
              value: summary.resultPreview,
            });
          }
          if (summary.errorMessage) {
            details.push({
              label: t('conversation.tools.simpleSummary.errorLabel'),
              value: summary.errorMessage,
            });
          }

          const sandboxPolicy = describeSandboxPolicy(summary.sandboxLevel, t);

          return (
            <li key={summary.callId} className="space-y-1 text-slate-600">
              <p className="text-[9px] text-slate-700">
                {timestamp} · {summary.toolName} · {statusLabel}
                {duration ? ` · ${duration}` : ''}
              </p>
              <p className="text-[8px] text-slate-500">
                {t('conversation.tools.simpleSummary.sandboxLine', { policy: sandboxPolicy })}
              </p>
              {details.map(({ label, value }, index) => (
                <p key={`${summary.callId}-detail-${index}`} className="text-[8px] text-slate-500">
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
  t: (key: TranslationKey, params?: TranslationParams) => string
) {
  switch (level) {
    case 'filesystem':
      return t('conversation.tools.simpleSummary.policy.filesystem');
    case 'system':
      return t('conversation.tools.simpleSummary.policy.system');
    default:
      return t('conversation.tools.simpleSummary.policy.standard');
  }
}

// Single event line component
function EventLine({
  event,
  t,
  locale,
  toolSummariesById,
}: {
  event: DisplayEvent;
  t: (key: TranslationKey, params?: TranslationParams) => string;
  locale: string;
  toolSummariesById: Map<string, ToolCallSummary>;
}) {
  if (event.event_type === 'user_task') {
    const timestamp = formatTimestamp(event.timestamp, locale);
    return (
        <div className="flex justify-end" data-testid="event-line-user">
          <div className="max-w-xl rounded-2xl border border-slate-200 bg-white px-5 py-3.5 text-sm font-normal text-slate-800 shadow-sm">
            <p className="whitespace-pre-wrap leading-relaxed">{event.task}</p>
            <time className="mt-2 block text-[9px] font-medium uppercase tracking-[0.3em] text-slate-400">
              {timestamp}
            </time>
          </div>
        </div>
    );
  }

  const timestamp = formatTimestamp(event.timestamp, locale);
  const category = getEventCategory(event);
  const presentation = describeEvent(event, t);
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

  const headlineSize = HEADLINE_SIZES[category];

  return (
    <article
      className={cn(
        'group relative max-w-3xl space-y-3 border-l border-slate-200/70 pl-5 text-slate-700',
        anchorId && 'timeline-anchor-target scroll-mt-28'
      )}
      data-testid={`event-line-${event.event_type}`}
      data-category={category}
      data-anchor-id={anchorId ?? undefined}
      id={anchorId ? `event-${anchorId}` : undefined}
      tabIndex={anchorId ? -1 : undefined}
    >
      <span
        aria-hidden
        className="absolute -left-[5px] top-3 h-2 w-2 rounded-full bg-slate-300"
      />
      <div className="flex items-start gap-4">
        <div className={cn('relative mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center', meta.iconTone)}>
          <meta.icon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-[13px]">
            <p className={cn('font-semibold leading-tight text-slate-900', meta.headline, headlineSize)}>
              {presentation.headline}
            </p>
            <span
              className={cn(
                'text-[8px] font-semibold uppercase tracking-[0.3em] text-slate-400',
                meta.pill
              )}
            >
              {meta.label}
            </span>
            {presentation.status && <StatusBadge status={presentation.status} label={statusLabel} />}
          </div>
          {presentation.subheading && (
            <p className="text-[9px] font-medium uppercase tracking-[0.3em] text-slate-400">
              {presentation.subheading}
            </p>
          )}

          {isToolCallStartDisplayEvent(event) ? (
            <ToolCallContent
              event={event}
              statusLabel={statusLabel}
              t={t}
              locale={locale}
              summary={toolSummariesById.get(event.call_id)}
            />
          ) : (
            <>
              {presentation.summary && (
                <div className="whitespace-pre-line text-xs leading-relaxed text-slate-600">
                  {presentation.summary}
                </div>
              )}
              {presentation.supplementary}
            </>
          )}

          {!isToolCallStartDisplayEvent(event) && <EventMetadata event={event} accentClass={meta.accent} />}

          <time className="block text-[8px] font-medium uppercase tracking-[0.3em] text-slate-300">
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
  t,
  locale,
  summary,
}: {
  event: ToolCallStartDisplayEvent;
  statusLabel?: string;
  t: (key: TranslationKey, params?: TranslationParams) => string;
  locale: string;
  summary?: ToolCallSummary;
}) {
  const effectiveStatus = summary?.status
    ?? (event.call_status === 'error'
      ? 'error'
      : event.call_status === 'complete'
        ? 'completed'
        : 'running');

  const statusText =
    effectiveStatus === 'running'
      ? t('conversation.status.doing')
      : effectiveStatus === 'error'
        ? t('conversation.status.failed')
        : t('conversation.status.completed');

  const durationLabel = summary?.durationMs
    ? formatDuration(summary.durationMs)
    : event.completion_duration
      ? formatDuration(event.completion_duration)
      : null;

  const timestampLabel = summary?.completedAt
    ? formatTimestamp(summary.completedAt, locale)
    : formatTimestamp(event.timestamp, locale);

  const argsPreview = summary?.argumentsPreview ?? formatArgumentsPreview(event.arguments_preview ?? event.arguments);
  const resultPreview = summary?.resultPreview ?? formatResultPreview(event.completion_result);
  const errorText = summary?.errorMessage ?? event.completion_error;

  return (
    <div className="space-y-2 text-[9px] leading-relaxed text-slate-600">
      <p className="font-semibold text-slate-700">
        {event.tool_name} · {statusText}
        {durationLabel ? ` · ${durationLabel}` : ''}
      </p>
      <p className="text-[8px] uppercase tracking-[0.3em] text-slate-400">{timestampLabel}</p>
      {statusLabel && (
        <p className="text-[8px] uppercase tracking-[0.3em] text-slate-500">{statusLabel}</p>
      )}
      {argsPreview && (
        <p>
          <span className="font-semibold uppercase tracking-[0.25em] text-slate-500">
            {t('conversation.tool.timeline.arguments')}:
          </span>{' '}
          <span>{argsPreview}</span>
        </p>
      )}
      {resultPreview && (
        <p>
          <span className="font-semibold uppercase tracking-[0.25em] text-slate-500">
            {t('conversation.tool.timeline.result', { tool: event.tool_name })}:
          </span>{' '}
          <span>{resultPreview}</span>
        </p>
      )}
      {errorText && (
        <p className="text-destructive">
          <span className="font-semibold uppercase tracking-[0.25em] text-destructive">
            {t('conversation.tool.timeline.errorOutput')}:
          </span>{' '}
          <span>{errorText}</span>
        </p>
      )}
      {summary?.requiresSandbox && (
        <p className="text-[8px] uppercase tracking-[0.25em] text-slate-400">
          {t('conversation.environment.sandbox.inlineNotice')}
        </p>
      )}
      <EventMetadata event={event} accentClass="text-slate-400" />
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

/**
 * Filter out noise events that don't provide meaningful information to users
 * Only show key results and important milestones
 */
function shouldSkipEvent(event: AnyAgentEvent): boolean {
  switch (event.event_type) {
    // Skip all system iteration events - too noisy
    case 'iteration_start':
    case 'iteration_complete':
      return true;

    // Skip task analysis - just internal planning
    case 'task_analysis':
      return true;

    // Skip connected event - just connection status
    case 'connected':
      return true;

    // Keep these events for now to debug:
    // - thinking/think_complete: Show what agent is thinking
    // - user_task: User's input (important to show what was asked)
    // - tool_call_start/complete: Tool execution results (actual work being done)
    // - task_complete: Final answer (the result)
    // - error: Errors (problems user needs to know about)
    // - research_plan/step_*: Research workflow milestones

    default:
      return false;
  }
}

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
    iconTone: string;
    pill: string;
    headline: string;
    accent: string;
    label: string;
  }
> = {
  conversation: {
    icon: MessageSquare,
    iconTone: 'text-sky-500',
    pill: 'text-sky-400',
    headline: 'text-sky-900',
    accent: 'text-sky-400',
    label: 'Conversation',
  },
  plan: {
    icon: ClipboardList,
    iconTone: 'text-amber-500',
    pill: 'text-amber-400',
    headline: 'text-amber-900',
    accent: 'text-amber-400',
    label: 'Planning',
  },
  tools: {
    icon: Wrench,
    iconTone: 'text-emerald-500',
    pill: 'text-emerald-400',
    headline: 'text-emerald-900',
    accent: 'text-emerald-400',
    label: 'Tools',
  },
  system: {
    icon: Cpu,
    iconTone: 'text-slate-500',
    pill: 'text-slate-400',
    headline: 'text-slate-900',
    accent: 'text-slate-400',
    label: 'System',
  },
  other: {
    icon: Info,
    iconTone: 'text-slate-400',
    pill: 'text-slate-400',
    headline: 'text-slate-900',
    accent: 'text-slate-400',
    label: 'Other',
  },
};

const HEADLINE_SIZES: Record<EventCategory, string> = {
  conversation: 'text-xl sm:text-2xl',
  plan: 'text-lg sm:text-xl',
  tools: 'text-sm sm:text-base',
  system: 'text-base sm:text-lg',
  other: 'text-base',
};

function describeEvent(
  event: DisplayEvent,
  t: (key: TranslationKey, params?: TranslationParams) => string
): EventPresentation {
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
        supplementary: (
          <ContentBlock title="Model Response">
            <pre className="whitespace-pre-wrap font-mono text-[8px] leading-snug text-foreground/90">
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
            t={t}
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
        supplementary: (
          <ContentBlock title="Final Answer" scrollable={false}>
            <pre className="whitespace-pre-wrap font-mono text-[8px] leading-snug text-foreground/90">
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
                <pre className="whitespace-pre-wrap font-mono text-[8px] leading-snug text-foreground/90">
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

function EventMetadata({ event, accentClass }: { event: DisplayEvent; accentClass?: string }) {
  const entries = getEventMetadata(event);
  if (!entries.length) return null;

  const isToolEvent =
    event.event_type === 'tool_call_start' || event.event_type === 'tool_call_complete';

  return (
    <dl
      className={cn(
        'flex flex-wrap gap-x-4 gap-y-1 uppercase tracking-[0.25em] text-slate-400',
        isToolEvent ? 'text-[8px]' : 'text-[9px]'
      )}
    >
      {entries.map(({ label, value }) => (
        <div key={`${event.timestamp}-${label}`} className="flex items-center gap-2">
          <dt className={cn('font-semibold', accentClass)}>{label}</dt>
          <dd
            className={cn(
              'font-mono tracking-normal text-slate-500',
              isToolEvent ? 'text-[8px]' : 'text-[10px]'
            )}
          >
            {value}
          </dd>
        </div>
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

function ToolArguments({
  args,
  callId,
  t,
}: {
  args?: Record<string, any> | string;
  callId: string;
  t: (key: TranslationKey, params?: TranslationParams) => string;
}) {
  if (!args || (typeof args === 'object' && Object.keys(args).length === 0)) {
    return null;
  }

  const formatted = typeof args === 'string' ? args : JSON.stringify(args, null, 2);

  return (
    <ContentBlock
      title={t('conversation.tool.timeline.arguments')}
      tone="emerald"
      dataTestId={`tool-call-arguments-${callId}`}
      variant="compact"
    >
      <pre className="whitespace-pre-wrap font-mono text-[8px] leading-snug text-current sm:text-[9px]">
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
  t,
}: {
  result: any;
  error?: string;
  callId: string;
  toolName: string;
  t: (key: TranslationKey, params?: TranslationParams) => string;
}) {
  if (error) {
    return (
      <ContentBlock
        title={t('conversation.tool.timeline.errorOutput')}
        tone="destructive"
        dataTestId={`tool-call-result-${callId}`}
        variant="compact"
      >
        <p className="text-[8px] font-medium text-destructive dark:text-destructive/80 sm:text-[9px]">{error}</p>
      </ContentBlock>
    );
  }

  if (!result) return null;

  const formatted = typeof result === 'string' ? result : JSON.stringify(result, null, 2);

  return (
    <ContentBlock
      title={t('conversation.tool.timeline.result', { tool: toolName })}
      tone="emerald"
      dataTestId={`tool-call-result-${callId}`}
      variant="compact"
    >
      <pre className="whitespace-pre-wrap font-mono text-[7px] leading-snug text-current sm:text-[8px]">
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
  variant = 'default',
}: {
  title: string;
  children: ReactNode;
  tone?: 'emerald' | 'slate' | 'destructive';
  dataTestId?: string;
  scrollable?: boolean;
  variant?: 'default' | 'compact';
}) {
  const toneClasses = {
    emerald: 'border-emerald-300 text-emerald-600',
    slate: 'border-slate-200 text-slate-600',
    destructive: 'border-destructive/70 text-destructive',
  } as const;

  const isCompact = variant === 'compact';

  return (
    <div
      className={cn(
        'mt-3 space-y-2 border-l-2 pl-3 leading-snug',
        isCompact ? 'text-[7px] sm:text-[8px]' : 'text-[9px] sm:text-[10px]',
        toneClasses[tone],
        scrollable && 'console-scrollbar max-h-36 overflow-y-auto pr-1'
      )}
      data-testid={dataTestId}
    >
      <p
        className={cn(
          'font-semibold uppercase tracking-[0.3em] opacity-70',
          isCompact ? 'text-[6px] sm:text-[7px]' : 'text-[8px] sm:text-[9px]'
        )}
      >
        {title}
      </p>
      <div className={cn('space-y-1', isCompact ? 'text-[7px] sm:text-[8px]' : 'text-[10px] sm:text-[11px]')}>
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
      className: 'text-emerald-600',
    },
    warning: {
      icon: AlertTriangle,
      label: 'Warning',
      className: 'text-amber-500',
    },
    danger: {
      icon: AlertTriangle,
      label: 'Error',
      className: 'text-destructive',
    },
    info: {
      icon: Info,
      label: 'Info',
      className: 'text-sky-500',
    },
    pending: {
      icon: Loader2,
      label: 'Pending',
      className: 'text-sky-500',
    },
  };

  const meta = config[status];
  const Icon = meta.icon;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.25em] sm:text-[10px]',
        meta.className
      )}
    >
      <Icon className="h-3 w-3" />
      {label ?? meta.label}
    </span>
  );
}

function formatTimestamp(timestamp?: string, locale = 'en-US') {
  const value = timestamp ? new Date(timestamp) : new Date();
  if (Number.isNaN(value.getTime())) {
    return '';
  }

  return value.toLocaleTimeString(locale, {
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
