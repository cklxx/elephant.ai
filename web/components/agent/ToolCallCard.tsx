'use client';

import { useMemo } from 'react';
import { ToolCallStartEvent, ToolCallCompleteEvent } from '@/lib/types';
import { getToolIcon, formatDuration } from '@/lib/utils';
import { CheckCircle2, Loader2, XCircle, Film } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { ToolCallLayout } from './tooling/ToolCallLayout';
import { resolveToolRenderer } from './tooling/toolRenderers';
import { adaptToolCallForRenderer } from './tooling/toolDataAdapters';

interface ToolCallCardProps {
  event: ToolCallStartEvent | ToolCallCompleteEvent;
  status: 'running' | 'done' | 'error';
  pairedStart?: ToolCallStartEvent;
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
        'inline-flex items-center gap-1 rounded-full border border-border px-2 py-0.5 text-[9px] font-semibold tracking-[0.2em]',
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
    className: 'bg-muted text-foreground',
  },
  done: {
    icon: CheckCircle2,
    className: 'bg-muted text-foreground',
  },
  error: {
    icon: XCircle,
    className: 'bg-destructive/20 text-destructive',
  },
} as const;

const STATUS_LABELS = {
  running: (t: ReturnType<typeof useTranslation>) => t('conversation.status.doing'),
  done: (t: ReturnType<typeof useTranslation>) => t('conversation.status.completed'),
  error: (t: ReturnType<typeof useTranslation>) => t('conversation.status.failed'),
} as const;

const VIDEO_GENERATION_TOOLS = new Set(['seedream_video_generate']);

function VideoWaitHint() {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-amber-200/80 bg-amber-50/80 px-3 py-2 text-[11px] font-medium text-amber-800">
      <Film className="h-4 w-4 text-amber-500" aria-hidden />
      <span>视频生成较慢，请耐心等待...</span>
    </div>
  );
}
