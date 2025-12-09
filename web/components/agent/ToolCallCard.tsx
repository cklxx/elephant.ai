'use client';

import { useMemo } from 'react';
import { WorkflowToolStartedEvent, WorkflowToolCompletedEvent } from '@/lib/types';
import { isWorkflowToolStartedEvent } from '@/lib/typeGuards';
import { getToolIcon, formatDuration } from '@/lib/utils';
import { CheckCircle2, Loader2, XCircle, Film } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { ToolCallLayout } from './tooling/ToolCallLayout';
import { resolveToolRenderer } from './tooling/toolRenderers';
import { adaptToolCallForRenderer } from './tooling/toolDataAdapters';

interface ToolCallCardProps {
  event: WorkflowToolStartedEvent | WorkflowToolCompletedEvent;
  status: 'running' | 'done' | 'error';
  pairedStart?: WorkflowToolStartedEvent;
  isFocused?: boolean;
}

export function ToolCallCard({ event, status, pairedStart, isFocused = false }: ToolCallCardProps) {
  const t = useTranslation();
  const adapter = useMemo(
    () => adaptToolCallForRenderer({ event, pairedStart, status }),
    [event, pairedStart, status]
  );
  const toolName = adapter.toolName;
  const toolGlyph = getToolIcon(toolName);
  const callId = adapter.callId;
  const renderer = resolveToolRenderer(toolName);

  const statusLabel = STATUS_LABELS[status](t);
  const metadata = adapter.durationMs
    ? t('conversation.tool.timeline.duration', { duration: formatDuration(adapter.durationMs) })
    : undefined;
  const showVideoWaitHint =
    status === 'running' && VIDEO_GENERATION_TOOLS.has(toolName.toLowerCase());

  const summaryText = useMemo(() => {
    const argsSummary = getArgumentsPreview(event, adapter.context.startEvent ?? undefined);
    const errorSummary = adapter.context.completeEvent?.error?.trim();
    const resultSummary = summarizeResult(adapter.context.completeEvent?.result);

    if (status === 'running') {
      return argsSummary || t('conversation.tool.timeline.summaryRunning', { tool: toolName });
    }

    if (status === 'error') {
      return errorSummary || resultSummary || argsSummary || t('conversation.tool.timeline.summaryErrored', { tool: toolName });
    }

    return resultSummary || argsSummary || t('conversation.tool.timeline.summaryCompleted', { tool: toolName });
  }, [adapter, event, status, t, toolName]);

  const panels = renderer({
    ...adapter.context,
    labels: {
      arguments: t('conversation.tool.timeline.arguments'),
      stream: t('conversation.tool.timeline.liveOutput'),
      result: t('conversation.tool.timeline.result', { tool: toolName }),
      error: t('conversation.tool.timeline.errorOutput'),
      copyArgs: t('events.toolCall.copyArguments'),
      copyResult: t('events.toolCall.copyResult'),
      copyError: t('events.toolCall.copyError'),
      copied: t('events.toolCall.copied'),
      metadataTitle: t('conversation.tool.timeline.metadata'),
    },
  }).panels;

  return (
    <ToolCallLayout
      toolName={toolName}
      icon={toolGlyph}
      callId={callId}
      statusChip={<StatusChip status={status} label={statusLabel} />}
      summary={summaryText}
      metadata={metadata}
      isFocused={isFocused}
    >
      {showVideoWaitHint && <VideoWaitHint />}
      {panels.map((panel, index) => (
        <div key={index}>{panel}</div>
      ))}
    </ToolCallLayout>
  );
}

function StatusChip({ status, label }: { status: 'running' | 'done' | 'error'; label: string }) {
  const meta = STATUS_META[status];
  const Icon = meta.icon;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full border px-3 py-1 text-[11px] font-semibold text-foreground/80',
        meta.className,
      )}
    >
      <Icon className={cn('h-3 w-3', status === 'running' && 'animate-spin')} />
      {label}
    </span>
  );
}

const STATUS_META = {
  running: {
    icon: Loader2,
    className: 'border-amber-200 bg-amber-50/80 text-amber-800',
  },
  done: {
    icon: CheckCircle2,
    className: 'border-emerald-200 bg-emerald-50/80 text-emerald-800',
  },
  error: {
    icon: XCircle,
    className: 'border-destructive/30 bg-destructive/10 text-destructive',
  },
} as const;

const STATUS_LABELS = {
  running: (t: ReturnType<typeof useTranslation>) => t('conversation.status.doing'),
  done: (t: ReturnType<typeof useTranslation>) => t('conversation.status.completed'),
  error: (t: ReturnType<typeof useTranslation>) => t('conversation.status.failed'),
} as const;

const VIDEO_GENERATION_TOOLS = new Set(['video_generate']);

function VideoWaitHint() {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-amber-200/80 bg-amber-50/80 px-3 py-2 text-[11px] font-medium text-amber-800">
      <Film className="h-4 w-4 text-amber-500" aria-hidden />
      <span>视频生成较慢，请耐心等待...</span>
    </div>
  );
}

function getArgumentsPreview(
  event: WorkflowToolStartedEvent | WorkflowToolCompletedEvent,
  startEvent?: WorkflowToolStartedEvent | null
): string | undefined {
  const preview =
    startEvent?.arguments_preview ??
    (isWorkflowToolStartedEvent(event) ? event.arguments_preview : undefined);
  if (preview && preview.trim().length > 0) {
    return preview.trim();
  }

  const args =
    startEvent?.arguments ??
    (isWorkflowToolStartedEvent(event) ? event.arguments : undefined);
  return summarizeArguments(args);
}

function summarizeArguments(args?: Record<string, unknown>): string | undefined {
  if (!args || Object.keys(args).length === 0) {
    return undefined;
  }

  const entries = Object.entries(args)
    .map(([key, value]) => {
      const normalized = formatArgumentValue(value);
      if (!normalized) {
        return null;
      }
      return `${key}: ${normalized}`;
    })
    .filter(Boolean) as string[];

  if (entries.length === 0) {
    return undefined;
  }

  const preview = entries.join(' · ');
  return preview.length > 200 ? `${preview.slice(0, 200)}…` : preview;
}

function formatArgumentValue(value: unknown): string {
  if (value == null) {
    return '';
  }
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (!trimmed) {
      return '';
    }
    return trimmed.length > 80 ? `${trimmed.slice(0, 80)}…` : trimmed;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  if (Array.isArray(value)) {
    const formatted = value
      .slice(0, 3)
      .map((item) => formatArgumentValue(item))
      .filter(Boolean)
      .join(', ');
    if (!formatted) {
      return '';
    }
    return value.length > 3 ? `${formatted}…` : formatted;
  }
  if (typeof value === 'object') {
    try {
      const json = JSON.stringify(value);
      if (!json) {
        return '';
      }
      return json.length > 80 ? `${json.slice(0, 80)}…` : json;
    } catch {
      return '';
    }
  }
  return '';
}

function summarizeResult(result?: string | null): string | undefined {
  if (!result) {
    return undefined;
  }
  const trimmed = result.trim();
  if (!trimmed) {
    return undefined;
  }
  return trimmed.length > 200 ? `${trimmed.slice(0, 200)}…` : trimmed;
}
