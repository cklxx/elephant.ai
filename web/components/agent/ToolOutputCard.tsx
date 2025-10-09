'use client';

import { useMemo, useState } from 'react';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { formatDuration } from '@/lib/utils';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

interface ToolOutputCardProps {
  toolName: string;
  parameters?: Record<string, unknown>;
  result?: string;
  error?: string;
  duration?: number;
  timestamp: string;
  callId?: string;
}

export function ToolOutputCard({
  toolName,
  parameters,
  result,
  error,
  duration,
  timestamp,
  callId,
}: ToolOutputCardProps) {
  const hasResult = Boolean(result && result.trim().length > 0);
  const hasParameters = Boolean(parameters && Object.keys(parameters).length > 0);
  const hasError = Boolean(error && error.trim().length > 0);
  const [isExpanded, setIsExpanded] = useState(() => Boolean(error) || !hasResult);
  const t = useTranslation();

  const language = useMemo(
    () => detectLanguage(toolName, parameters, result),
    [toolName, parameters, result]
  );

  const formattedParams = useMemo(() => formatParams(parameters), [parameters]);
  const formattedTimestamp = useMemo(() => formatTimestamp(timestamp), [timestamp]);
  const resultLength = result?.length ?? 0;
  const errorLength = error?.length ?? 0;
  const shouldShowToggle =
    (hasResult && resultLength > 0) || (hasError && (errorLength > 160 || hasResult));

  const previewText = useMemo(() => {
    if (error) {
      return error;
    }
    if (!result) {
      return '';
    }
    const trimmed = result.trim();
    if (trimmed.length <= 160) {
      return trimmed;
    }
    return `${trimmed.slice(0, 160)}…`;
  }, [error, result]);

  return (
    <Card className="console-card border-l-4 border-primary/50 animate-fadeIn overflow-hidden">
      <CardHeader className="px-5 py-4 space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 font-mono text-sm">
              <span className={error ? 'text-destructive font-semibold' : 'text-primary font-semibold'}>
                {error ? '✗' : '▸'} {toolName}
              </span>
              {formattedParams && (
                <span className="truncate text-muted-foreground">
                  {formattedParams}
                </span>
              )}
            </div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>{formattedTimestamp}</span>
            {typeof duration === 'number' && duration >= 0 && (
              <Badge variant="info" className="font-mono text-[11px]">
                {formatDuration(duration)}
              </Badge>
            )}
            {callId && (
              <span className="font-mono text-muted-foreground/70">#{callId}</span>
            )}
          </div>
        </div>
        <Badge variant={error ? 'error' : 'success'} className="shrink-0">
          {error ? t('tool.status.failed') : t('tool.status.completed')}
        </Badge>
      </div>
      {previewText && (
        <p
          className={`text-sm leading-relaxed whitespace-pre-wrap break-words ${
            error ? 'text-destructive' : 'text-muted-foreground'
          }`}
        >
          {previewText}
        </p>
      )}
    </CardHeader>

    {(hasResult || hasParameters || hasError) && (
      <div className="border-t border-border/60 bg-muted/40">
        {shouldShowToggle && (
          <button
            type="button"
            onClick={() => setIsExpanded((prev) => !prev)}
            className="flex w-full items-center justify-between gap-2 px-5 py-2 text-xs font-medium text-muted-foreground hover:bg-muted/60 transition-colors"
          >
            <span>
                {isExpanded ? t('tool.toggle.collapse') : t('tool.toggle.expand')}
                {hasResult &&
                  t('tool.toggle.length', { count: resultLength.toLocaleString() })}
            </span>
            {isExpanded ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </button>
        )}

        {(isExpanded || !shouldShowToggle) && (
          <CardContent className="space-y-4 px-5 pb-5 pt-4">
            {hasParameters && (
              <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    {t('tool.section.parameters')}
                  </p>
                  <pre className="console-card bg-background p-3 text-xs font-mono overflow-x-auto">
                    {JSON.stringify(parameters, null, 2)}
                  </pre>
              </div>
            )}

            {hasError && (
              <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-destructive">
                    {t('tool.section.error')}
                  </p>
                  <pre className="console-card bg-destructive/10 border border-destructive/30 p-3 text-xs font-mono text-destructive overflow-x-auto">
                    {error}
                  </pre>
              </div>
            )}

            {hasResult && (
              <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    {t('tool.section.output')}
                  </p>
                  <SyntaxHighlighter
                    language={language}
                    style={vscDarkPlus}
                    customStyle={{
                      margin: 0,
                      borderRadius: '0.5rem',
                      fontSize: '0.75rem',
                      maxHeight: 400,
                    }}
                    showLineNumbers={language !== 'text'}
                  >
                    {result ?? ''}
                  </SyntaxHighlighter>
                </div>
              )}
            </CardContent>
          )}
        </div>
      )}
    </Card>
  );
}

function detectLanguage(
  toolName: string,
  parameters?: Record<string, unknown>,
  result?: string
): string {
  if (toolName === 'bash' || toolName === 'shell' || toolName === 'terminal') {
    return 'bash';
  }
  if (toolName === 'code_execute' || toolName === 'python_execute') {
    return 'python';
  }
  if (toolName === 'web_fetch' || toolName === 'web_search') {
    return 'html';
  }
  if (toolName === 'file_read') {
    const path = typeof parameters?.path === 'string' ? parameters.path : '';
    const ext = path.split('.').pop();
    if (ext) {
      const normalized = ext.toLowerCase();
      if (normalized === 'ts' || normalized === 'tsx') return 'typescript';
      if (normalized === 'js' || normalized === 'jsx') return 'javascript';
      if (normalized === 'json') return 'json';
      if (normalized === 'py') return 'python';
      if (normalized === 'go') return 'go';
      if (normalized === 'rs') return 'rust';
      if (normalized === 'sh' || normalized === 'bash') return 'bash';
      if (normalized === 'md') return 'markdown';
      if (normalized === 'html' || normalized === 'htm') return 'html';
      if (normalized === 'css') return 'css';
      if (normalized === 'sql') return 'sql';
      if (normalized === 'yml' || normalized === 'yaml') return 'yaml';
    }
  }

  if (result) {
    const trimmed = result.trim();
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      return 'json';
    }
    if (/^<([a-z-]+)(\s|>)/i.test(trimmed)) {
      return 'html';
    }
  }

  return 'text';
}

function formatParams(parameters?: Record<string, unknown>): string | null {
  if (!parameters) return null;
  const entries = Object.entries(parameters);
  if (entries.length === 0) return null;
  return entries
    .slice(0, 2)
    .map(([key, value]) => `${key}: ${formatParamValue(value)}`)
    .join(', ');
}

function formatParamValue(value: unknown): string {
  if (value == null) return 'null';
  if (typeof value === 'string') {
    return value.length > 24 ? `${value.slice(0, 24)}…` : value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return JSON.stringify(value).slice(0, 24);
}

function formatTimestamp(timestamp: string): string {
  try {
    return new Date(timestamp).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  } catch (error) {
    return timestamp;
  }
}
