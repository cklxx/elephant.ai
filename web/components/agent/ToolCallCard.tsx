'use client';

import { useMemo, useState } from 'react';
import { WorkflowToolStartedEvent, WorkflowToolCompletedEvent } from '@/lib/types';
import { isWorkflowToolStartedEvent } from '@/lib/typeGuards';
import { getToolIcon, formatDuration, humanizeToolName } from '@/lib/utils';
import { ChevronRight, Loader2, X, Film } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
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
  const [isExpanded, setIsExpanded] = useState(false);

  const adapter = useMemo(
    () => adaptToolCallForRenderer({ event, pairedStart, status }),
    [event, pairedStart, status]
  );
  const toolName = adapter.toolName;

  // Humanize tool name
  const displayToolName = useMemo(() => {
    return humanizeToolName(toolName);
  }, [toolName]);


  const ToolIcon = getToolIcon(toolName);
  const duration = adapter.durationMs ? formatDuration(adapter.durationMs) : null;
  const renderer = resolveToolRenderer(toolName);

  const showVideoWaitHint =
    status === 'running' && toolName.toLowerCase() === 'video_generate';

  const summaryText = useMemo(() => {
    // Priority: Result Summary > Error > Args Summary > Default Text
    const argsSummary = getArgumentsPreview(event, adapter.context.startEvent ?? undefined);
    const errorSummary = adapter.context.completeEvent?.error?.trim();
    const resultSummary = summarizeResult(adapter.context.completeEvent?.result);

    if (status === 'running') {
      return argsSummary || t('conversation.tool.timeline.summaryRunning', { tool: toolName });
    }
    if (status === 'error') {
      return errorSummary || t('conversation.tool.timeline.summaryErrored', { tool: toolName });
    }
    return resultSummary || argsSummary || t('conversation.tool.timeline.summaryCompleted', { tool: toolName });
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
        "group mb-2 transition-all",
        isFocused && "bg-muted/10"
      )}
      data-testid={`tool-call-card-${displayToolName.toLowerCase().replace(/\s+/g, '-')}`}
    >
      {/* Header Row - Manus Style Gray Pill */}
      <div
        role="button"
        onClick={() => setIsExpanded(!isExpanded)}
        data-testid="tool-call-header"
        className={cn(
          "flex items-center gap-3 px-3 py-2 cursor-pointer select-none rounded-md text-sm",
          "bg-secondary/40 hover:bg-secondary/60 transition-colors border border-border/40",
          status === 'running' && "bg-blue-50/50 border-blue-100/50 text-blue-900 dark:bg-blue-900/20 dark:text-blue-100 dark:border-blue-800/30",
          status === 'error' && "bg-red-50/50 border-red-100/50 text-red-900 dark:bg-red-900/20 dark:text-red-100 dark:border-red-800/30"
        )}
      >
        <div
          className={cn(
            "flex items-center justify-center transition-all",
            status === 'running' ? "text-blue-600 dark:text-blue-400" :
              status === 'error' ? "text-red-600 dark:text-red-400" :
                "text-muted-foreground/70"
          )}
          data-testid={`tool-call-status-${status}`}
        >
          {status === 'running' ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> :
            status === 'error' ? <X className="w-3.5 h-3.5" /> :
              <span className="text-sm leading-none">{ToolIcon}</span>}
        </div>

        <div className="flex-1 min-w-0 overflow-hidden">
          <span className="block font-medium opacity-90 truncate" data-testid="tool-call-name">
            {displayToolName}
          </span>
          {summaryText ? (
            <span className="block truncate text-xs text-muted-foreground/70">
              {summaryText}
            </span>
          ) : null}
        </div>

        <div className="flex items-center gap-2 text-xs text-muted-foreground/50 opacity-0 group-hover:opacity-100 transition-opacity">
          {duration && <span data-testid="tool-call-duration">{duration}</span>}
          <ChevronRight
            className={cn(
              "w-3.5 h-3.5 transition-transform duration-200",
              isExpanded && "rotate-90"
            )}
            data-testid="tool-call-expand-icon"
          />
        </div>
      </div>

      {/* Expanded Details - Keep it clean */}
      {isExpanded && (
        <div className="mt-2 pl-4 pr-1">
          {showVideoWaitHint && (
            <div className="flex items-center gap-2 p-2 mb-2 text-xs rounded-md bg-amber-50 text-amber-800 border border-amber-100">
              <Film className="w-4 h-4" />
              <span>Generating video... this may take a moment.</span>
            </div>
          )}

          <div className="text-sm font-mono bg-muted/30 rounded-lg overflow-hidden border border-border/40 text-xs">
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
}

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
  return res.length > 80 ? res.slice(0, 80) + '...' : res;
}

function formatArgumentValue(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) return `[Array(${value.length})]`;
  if (typeof value === 'object') return '{...}';
  return '';
}

function summarizeResult(result?: string | null): string | undefined {
  if (!result?.trim()) return undefined;
  // If result is huge json, just say "Result"
  if (result.startsWith('{') && result.length > 100) return 'Result Object';
  return result.length > 100 ? result.slice(0, 100) + '...' : result;
}
