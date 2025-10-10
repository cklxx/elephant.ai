'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { ToolCallStartEvent, ToolCallCompleteEvent } from '@/lib/types';
import { getToolIcon, formatDuration, formatJSON } from '@/lib/utils';
import { isToolCallStartEvent } from '@/lib/typeGuards';
import { CheckCircle2, Clipboard, ClipboardCheck, Loader2, XCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';

interface ToolCallCardProps {
  event: ToolCallStartEvent | ToolCallCompleteEvent;
  status: 'running' | 'complete' | 'error';
  pairedStart?: ToolCallStartEvent;
  isFocused?: boolean;
}

export function ToolCallCard({ event, status, pairedStart, isFocused = false }: ToolCallCardProps) {
  const t = useTranslation();
  const startEvent = useMemo(() => {
    if (isToolCallStartEvent(event)) return event;
    return pairedStart ?? null;
  }, [event, pairedStart]);
  const completeEvent = event.event_type === 'tool_call_complete' ? event : null;
  const toolName = completeEvent?.tool_name ?? startEvent?.tool_name ?? event.tool_name;
  const statusLabel = STATUS_LABELS[status](t);
  const toolGlyph = getToolIcon(toolName);

  const startStreamContent =
    startEvent && typeof (startEvent as any).stream_content === 'string'
      ? ((startEvent as any).stream_content as string)
      : undefined;
  const startStreamTimestamp =
    startEvent && typeof (startEvent as any).last_stream_timestamp === 'string'
      ? ((startEvent as any).last_stream_timestamp as string)
      : undefined;

  const startedAt = startEvent?.timestamp;
  const completedAt = completeEvent?.timestamp;
  const callId = completeEvent?.call_id ?? startEvent?.call_id ?? event.call_id;
  const durationLabel = completeEvent?.duration ? formatDuration(completeEvent.duration) : null;

  const hasArguments = Boolean(startEvent?.arguments && Object.keys(startEvent.arguments).length > 0);
  const argumentsPreview = startEvent?.arguments_preview;
  const hasResult = Boolean(completeEvent?.result && String(completeEvent.result).trim().length > 0);
  const hasError = Boolean(completeEvent?.error && completeEvent.error.trim().length > 0);
  const hasStream = Boolean(startStreamContent && startStreamContent.trim().length > 0);

  const formattedArguments = useMemo(() => {
    if (!hasArguments || !startEvent?.arguments) return null;
    return formatJSON(startEvent.arguments);
  }, [hasArguments, startEvent?.arguments]);

  const stages: TimelineStage[] = [];

  if (startEvent) {
    stages.push({
      id: 'start',
      title: t('conversation.tool.timeline.started', { tool: toolName }),
      timestamp: startEvent.timestamp,
      tone: status === 'running' ? 'active' : 'default',
      description:
        argumentsPreview ?? (hasArguments ? createPreview(startEvent.arguments) : undefined),
      content: hasArguments && formattedArguments ? (
        <ToolArguments
          args={formattedArguments}
          label={t('conversation.tool.timeline.arguments')}
          copyLabel={t('events.toolCall.copyArguments')}
          copiedLabel={t('events.toolCall.copied')}
        />
      ) : undefined,
    });
  }

  if (hasStream && startEvent) {
    stages.push({
      id: 'stream',
      title: t('conversation.tool.timeline.streaming'),
      timestamp: startStreamTimestamp ?? startEvent.timestamp,
      tone: status === 'running' ? 'active' : 'default',
      content: (
        <SimplePanel>
          <PanelHeader title={t('conversation.tool.timeline.liveOutput')} />
          <pre className="console-scrollbar max-h-48 overflow-auto whitespace-pre-wrap font-mono text-[8px] leading-snug text-slate-600 sm:text-[9px]">
            {startStreamContent?.trim()}
          </pre>
        </SimplePanel>
      ),
    });
  }

  if (completeEvent) {
    stages.push({
      id: 'completion',
      title: hasError ? t('conversation.tool.timeline.errored') : t('conversation.tool.timeline.completed'),
      timestamp: completeEvent.timestamp,
      tone: hasError ? 'error' : 'success',
      meta:
        completeEvent.duration && !Number.isNaN(completeEvent.duration)
          ? t('conversation.tool.timeline.duration', { duration: formatDuration(completeEvent.duration) })
          : undefined,
      content: (
        <ToolResult
          result={hasResult ? completeEvent.result : null}
          error={completeEvent.error}
          resultTitle={t('conversation.tool.timeline.result', { tool: toolName })}
          errorTitle={t('conversation.tool.timeline.errorOutput')}
          copyLabel={t('events.toolCall.copyResult')}
          copyErrorLabel={t('events.toolCall.copyError')}
          copiedLabel={t('events.toolCall.copied')}
        />
      ),
    });
  }

  return (
    <section
      className={cn(
        'relative space-y-3 py-2.5 pl-1 pr-1 sm:pl-2 sm:pr-2 transition',
        isFocused ? 'pl-2 sm:pl-3' : null
      )}
      data-testid="tool-call-card"
    >
      {isFocused && (
        <span
          aria-hidden
          className="absolute left-0 top-2 bottom-2 w-1 rounded-full bg-sky-300"
        />
      )}
      <header className="flex flex-wrap items-baseline gap-2.5 text-slate-900 text-[9px] sm:text-[10px]">
        {toolGlyph && <span className="text-[14px] text-slate-500 sm:text-[15px]">{toolGlyph}</span>}
        <h3 className="text-[10px] font-semibold tracking-tight text-slate-900 sm:text-[11px]">{toolName}</h3>
        <span className="text-[8px] uppercase tracking-[0.32em] text-slate-400 sm:text-[9px]">
          {t('events.toolCall.label')}
        </span>
        <StatusText status={status} label={statusLabel} />
      </header>

      {callId && (
        <p className="text-[9px] uppercase tracking-[0.25em] text-slate-400">
          {t('events.toolCall.id')} ·{' '}
          <span className="font-mono text-[8px] tracking-normal text-slate-500">{callId}</span>
        </p>
      )}

      <ul className="space-y-0">
        {stages.map((stage, index) => (
          <TimelineStageItem key={stage.id} stage={stage} isLast={index === stages.length - 1} />
        ))}
      </ul>

      <footer className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[8px] uppercase tracking-[0.25em] text-slate-400 sm:text-[9px]">
        {startedAt && (
          <time className="font-mono text-[9px] tracking-normal text-slate-500">
            {t('events.toolCall.start')}: {formatTimestamp(startedAt)}
          </time>
        )}
        {completedAt && (
          <time className="font-mono text-[9px] tracking-normal text-slate-500">
            {t('events.toolCall.end')}: {formatTimestamp(completedAt)}
          </time>
        )}
        {durationLabel && <span>{durationLabel}</span>}
      </footer>
    </section>
  );
}

type TimelineTone = 'default' | 'active' | 'success' | 'error';

interface TimelineStage {
  id: string;
  title: string;
  timestamp?: string;
  meta?: string;
  description?: string;
  content?: ReactNode;
  tone: TimelineTone;
}

function TimelineStageItem({ stage, isLast }: { stage: TimelineStage; isLast: boolean }) {
  return (
    <li className="relative pl-2 pb-5 last:pb-0 sm:pl-3">
      <div className="absolute left-0 top-0 flex h-full w-2 flex-col items-center sm:w-3">
        <span
          aria-hidden
          className={cn('mt-1.5 h-2 w-2 rounded-full', TIMELINE_TONES[stage.tone])}
        />
        {!isLast && <span aria-hidden className="mt-2 flex-1 w-px bg-slate-200/70" />}
      </div>
      <div className="pl-1 pr-2 sm:pl-2">
        <div className="space-y-1.5">
          <div className="flex flex-wrap items-baseline gap-2 text-[9px] sm:text-[10px]">
            <p className="font-medium text-slate-800">{stage.title}</p>
            {stage.timestamp && (
              <time className="text-[8px] font-medium text-slate-400 sm:text-[9px]">
                {formatTimestamp(stage.timestamp)}
              </time>
            )}
            {stage.meta && (
              <span className="text-[8px] font-medium text-slate-400 sm:text-[9px]">{stage.meta}</span>
            )}
          </div>
          {stage.description && (
            <p className="text-[9px] text-slate-500 sm:text-[10px]">{stage.description}</p>
          )}
          {stage.content}
        </div>
      </div>
    </li>
  );
}

const TIMELINE_TONES: Record<TimelineTone, string> = {
  default: 'bg-slate-300',
  active: 'bg-sky-400',
  success: 'bg-emerald-400',
  error: 'bg-destructive',
};

function StatusText({ status, label }: { status: 'running' | 'complete' | 'error'; label: string }) {
  const meta = STATUS_META[status];
  const Icon = meta.icon;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 text-[8px] font-medium sm:text-[9px]',
        meta.className
      )}
    >
      <Icon className={cn('h-2.5 w-2.5 sm:h-3 sm:w-3', status === 'running' && 'animate-spin')} />
      {label}
    </span>
  );
}

const STATUS_META = {
  running: {
    icon: Loader2,
    className: 'text-sky-600',
  },
  complete: {
    icon: CheckCircle2,
    className: 'text-emerald-600',
  },
  error: {
    icon: XCircle,
    className: 'text-destructive',
  },
} as const;

const STATUS_LABELS = {
  running: (t: ReturnType<typeof useTranslation>) => t('conversation.status.doing'),
  complete: (t: ReturnType<typeof useTranslation>) => t('conversation.status.completed'),
  error: (t: ReturnType<typeof useTranslation>) => t('conversation.status.failed'),
} as const;

function ToolArguments({
  args,
  label,
  copyLabel,
  copiedLabel,
}: {
  args: string;
  label: string;
  copyLabel: string;
  copiedLabel: string;
}) {
  return (
    <SimplePanel>
      <PanelHeader
        title={label}
        action={<CopyButton label={copyLabel} successLabel={copiedLabel} value={args} />}
      />
      <pre className="console-scrollbar max-h-56 overflow-auto whitespace-pre-wrap font-mono text-[8px] leading-snug text-slate-600 sm:text-[9px]">
        {args}
      </pre>
    </SimplePanel>
  );
}

function ToolResult({
  result,
  error,
  resultTitle,
  errorTitle,
  copyLabel,
  copyErrorLabel,
  copiedLabel,
}: {
  result: any;
  error?: string | null;
  resultTitle: string;
  errorTitle: string;
  copyLabel: string;
  copyErrorLabel: string;
  copiedLabel: string;
}) {
  if (error) {
    return (
      <SimplePanel>
        <PanelHeader
          title={errorTitle}
          action={<CopyButton label={copyErrorLabel} successLabel={copiedLabel} value={error} />}
        />
        <p className="text-[9px] font-medium text-destructive sm:text-[10px]">{error}</p>
      </SimplePanel>
    );
  }

  if (!result) return null;

  const formatted = typeof result === 'string' ? result : JSON.stringify(result, null, 2);

  return (
    <SimplePanel>
      <PanelHeader
        title={resultTitle}
        action={<CopyButton label={copyLabel} successLabel={copiedLabel} value={formatted} />}
      />
      <pre className="console-scrollbar max-h-56 overflow-auto whitespace-pre-wrap font-mono text-[8px] leading-snug text-slate-600 sm:text-[9px]">
        {formatted}
      </pre>
    </SimplePanel>
  );
}

function SimplePanel({ children }: { children: ReactNode }) {
  return (
    <div className="space-y-2 rounded-none bg-transparent text-[9px] text-slate-600 sm:text-[10px]">
      {children}
    </div>
  );
}

function PanelHeader({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <p className="text-[8px] font-semibold uppercase tracking-[0.25em] text-slate-400 sm:text-[9px]">{title}</p>
      {action}
    </div>
  );
}

function CopyButton({
  label,
  successLabel,
  value,
}: {
  label: string;
  successLabel: string;
  value?: string | null;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    if (!value) return;

    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
      } else {
        fallbackCopy(value);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch (error) {
      fallbackCopy(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    }
  }, [value]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="inline-flex items-center gap-1 text-[9px] font-medium text-sky-600 transition hover:text-sky-700 sm:text-[10px]"
      aria-label={copied ? successLabel : label}
    >
      {copied ? (
        <ClipboardCheck className="h-2.5 w-2.5 sm:h-3 sm:w-3" />
      ) : (
        <Clipboard className="h-2.5 w-2.5 sm:h-3 sm:w-3" />
      )}
      <span className="tracking-[0.1em]">{copied ? successLabel : label}</span>
    </button>
  );
}

function fallbackCopy(text: string) {
  try {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'absolute';
    textarea.style.left = '-9999px';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
  } catch (error) {
    console.error('Failed to copy tool output', error);
  }
}

function formatTimestamp(value?: string) {
  if (!value) return '';
  try {
    return new Date(value).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  } catch (error) {
    return value;
  }
}

function createPreview(args?: Record<string, any> | string): string | undefined {
  if (!args) return undefined;

  if (typeof args === 'string') {
    return truncate(args, 140);
  }

  const preview = Object.entries(args)
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join(', ');

  return truncate(preview, 140);
}

function truncate(value: string, length: number) {
  if (value.length <= length) return value;
  return `${value.slice(0, length)}…`;
}
