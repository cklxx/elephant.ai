'use client';

import { memo, useMemo, useState } from 'react';
import { WorkflowToolStartedEvent, WorkflowToolCompletedEvent } from '@/lib/types';
import { isWorkflowToolStartedEvent } from '@/lib/typeGuards';
import { getToolIcon, formatDuration } from '@/lib/utils';
import { Loader2, X, Film } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { resolveToolRenderer } from './tooling/toolRenderers';
import { adaptToolCallForRenderer } from './tooling/toolDataAdapters';
import { userFacingToolSummary, userFacingToolTitle } from '@/lib/toolPresentation';
import { useElapsedDurationMs } from '@/hooks/useElapsedDurationMs';

interface ToolCallCardProps {
  event: WorkflowToolStartedEvent | WorkflowToolCompletedEvent;
  status: 'running' | 'done' | 'error';
  pairedStart?: WorkflowToolStartedEvent;
  isFocused?: boolean;
}

// Custom comparator for React.memo - compare by event ID and status rather than reference
function arePropsEqual(
  prev: ToolCallCardProps,
  next: ToolCallCardProps
): boolean {
  // Status change always triggers re-render
  if (prev.status !== next.status) return false;
  if (prev.isFocused !== next.isFocused) return false;

  // Compare events by call_id (unique identifier)
  if (prev.event.call_id !== next.event.call_id) return false;

  // For running status, we need to re-render to show progress
  if (next.status === 'running') return false;

  // Compare paired start by call_id if present
  const prevStartId = prev.pairedStart?.call_id;
  const nextStartId = next.pairedStart?.call_id;
  if (prevStartId !== nextStartId) return false;

  // For completed events, compare by timestamp to detect updates
  if ('timestamp' in prev.event && 'timestamp' in next.event) {
    if (prev.event.timestamp !== next.event.timestamp) return false;
  }

  return true;
}

export const ToolCallCard = memo(function ToolCallCard({ event, status, pairedStart, isFocused = false }: ToolCallCardProps) {
  const t = useTranslation();
  const [isExpanded, setIsExpanded] = useState(false);

  const adapter = useMemo(
    () => adaptToolCallForRenderer({ event, pairedStart, status }),
    [event, pairedStart, status]
  );
  const toolName = adapter.toolName;

  // Humanize tool name
  const displayToolName = useMemo(() => {
    return userFacingToolTitle({
      toolName,
      arguments: adapter.context.startEvent?.arguments ?? null,
      metadata: adapter.context.completeEvent?.metadata ?? null,
      attachments: adapter.context.completeEvent?.attachments ?? null,
    });
  }, [
    toolName,
    adapter.context.startEvent?.arguments,
    adapter.context.completeEvent?.metadata,
    adapter.context.completeEvent?.attachments,
  ]);


  const ToolIcon = getToolIcon(toolName);
  const duration = adapter.durationMs ? formatDuration(adapter.durationMs) : null;
  const renderer = resolveToolRenderer(toolName);
  const runningElapsedMs = useElapsedDurationMs({
    startTimestamp: adapter.context.startEvent?.timestamp ?? null,
    running: status === 'running',
    tickMs: 250,
  });
  const runningDurationLabel =
    status === 'running' && typeof runningElapsedMs === 'number'
      ? formatDuration(runningElapsedMs)
      : null;

  const showVideoWaitHint =
    status === 'running' && toolName.toLowerCase() === 'video_generate';

  const summaryMaxLen = 96;
  const summaryText = useMemo(() => {
    // Priority: Result Summary > Error > Args Summary > Default Text
    const argsSummary = getArgumentsPreview(event, adapter.context.startEvent ?? undefined);
    const errorSummary = adapter.context.completeEvent?.error?.trim();
    const resultSummary = userFacingToolSummary({
      toolName,
      result: adapter.context.completeEvent?.result ?? null,
      error: adapter.context.completeEvent?.error ?? null,
      metadata: adapter.context.completeEvent?.metadata ?? null,
      attachments: adapter.context.completeEvent?.attachments ?? null,
    });

    if (status === 'running') {
      return compactOneLine(
        argsSummary || t('conversation.tool.timeline.summaryRunning', { tool: toolName }),
        summaryMaxLen,
      );
    }
    if (status === 'error') {
      return compactOneLine(
        errorSummary || t('conversation.tool.timeline.summaryErrored', { tool: toolName }),
        summaryMaxLen,
      );
    }
    return compactOneLine(
      resultSummary || argsSummary || t('conversation.tool.timeline.summaryCompleted', { tool: toolName }),
      summaryMaxLen,
    );
  }, [adapter, event, status, t, toolName]);

  // Render panels (args, output, etc.)
  const { panels } = renderer({
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
  });

  return (
    <div
      className={cn(
        "group mb-1 transition-all",
        isFocused && "bg-muted/10"
      )}
      data-testid={`tool-call-card-${toolName.toLowerCase().replace(/[^a-z0-9_-]+/g, '-')}`}
    >
      {/* Header Row - Manus Style Gray Pill */}
      <div
        role="button"
        onClick={() => setIsExpanded(!isExpanded)}
        data-testid="tool-call-header"
        className={cn(
          "grid grid-cols-[16px,1fr,auto] items-center gap-x-3 px-3 py-1.5 cursor-pointer select-none rounded-md",
          "text-[13px] leading-snug",
          "bg-secondary/40 hover:bg-secondary/60 transition-colors border border-border/40",
          status === 'running' && "bg-blue-50/50 border-blue-100/50 text-blue-900 dark:bg-blue-900/20 dark:text-blue-100 dark:border-blue-800/30",
          status === 'error' && "bg-red-50/50 border-red-100/50 text-red-900 dark:bg-red-900/20 dark:text-red-100 dark:border-red-800/30"
        )}
      >
        <div
          className={cn(
            "flex h-4 w-4 items-center justify-center transition-all",
            status === 'running' ? "text-blue-600 dark:text-blue-400" :
              status === 'error' ? "text-red-600 dark:text-red-400" :
                "text-muted-foreground/70"
          )}
          data-testid={`tool-call-status-${status}`}
        >
          {status === 'running' ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> :
            status === 'error' ? <X className="w-3.5 h-3.5" /> :
              <span className="text-[13px] leading-none">{ToolIcon}</span>}
        </div>

        <div className="min-w-0 overflow-hidden">
          <span
            className={cn(
              "block text-[13px] font-semibold tracking-tight truncate",
              status === "done" && "text-muted-foreground/80",
            )}
            data-testid="tool-call-name"
          >
            {displayToolName}
          </span>
          {summaryText ? (
            <span className="block truncate text-[12px] text-muted-foreground/60">
              {summaryText}
            </span>
          ) : null}
        </div>

        <div className="flex items-center gap-2 text-[11px] tabular-nums text-muted-foreground/60 transition-opacity">
          {status === 'running' ? (
            runningDurationLabel ? (
              <span data-testid="tool-call-duration">{runningDurationLabel}</span>
            ) : null
          ) : duration ? (
            <span data-testid="tool-call-duration">{duration}</span>
          ) : null}
        </div>
      </div>

      {/* Expanded Details - Keep it clean */}
      {isExpanded && (
        <div className="mt-1 pl-4 pr-1">
          {showVideoWaitHint && (
            <div className="flex items-center gap-2 p-2 mb-2 text-xs rounded-md bg-amber-50 text-amber-800 border border-amber-100">
              <Film className="w-4 h-4" />
              <span>Generating video... this may take a moment.</span>
            </div>
          )}

          <div className="rounded-lg overflow-hidden border border-border/40 bg-muted/30 text-xs">
            {panels.map((panel, i) => (
              <div key={i} className="[&>div]:border-none [&>div]:shadow-none [&>div]:bg-transparent [&_pre]:p-3 [&_pre]:text-xs">
                {panel}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}, arePropsEqual);

function getArgumentsPreview(
  event: WorkflowToolStartedEvent | WorkflowToolCompletedEvent,
  startEvent?: WorkflowToolStartedEvent | null
): string | undefined {
  const preview =
    startEvent?.arguments_preview ??
    (isWorkflowToolStartedEvent(event) ? event.arguments_preview : undefined);
  if (preview && preview.trim().length > 0) return preview.trim();

  const args =
    startEvent?.arguments ??
    (isWorkflowToolStartedEvent(event) ? event.arguments : undefined);
  return returnSummarizeArguments(args);
}

function returnSummarizeArguments(args?: Record<string, unknown>): string | undefined {
  if (!args || Object.keys(args).length === 0) return undefined;

  const entries = Object.entries(args)
    .map(([key, value]) => {
      const v = formatArgumentValue(value);
      return v ? `${key}: ${v}` : null;
    })
    .filter(Boolean) as string[];

  if (entries.length === 0) return undefined;
  const res = entries.join(', ');
  return res.length > 72 ? res.slice(0, 72) + '…' : res;
}

function formatArgumentValue(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) return `[Array(${value.length})]`;
  if (typeof value === 'object') return '{...}';
  return '';
}

function compactOneLine(value: string | undefined, maxLen: number): string | undefined {
  if (!value) return undefined;
  const normalized = value.replace(/\s+/g, ' ').trim();
  if (!normalized) return undefined;
  if (normalized.length <= maxLen) return normalized;
  return `${normalized.slice(0, maxLen)}…`;
}

// Intentionally left without a generic summarize helper:
// tool summaries are normalized via userFacingToolSummary().
