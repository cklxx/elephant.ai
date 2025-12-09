'use client';

import { ReactNode } from 'react';
import { Badge } from '@/components/ui/badge';

interface ToolCallLayoutProps {
  toolName: string;
  icon?: ReactNode;
  callId?: string | null;
  statusChip: ReactNode;
  summary?: string;
  isFocused?: boolean;
  metadata?: ReactNode;
  children: ReactNode;
}

export function ToolCallLayout({
  toolName,
  icon,
  callId,
  statusChip,
  summary,
  isFocused = false,
  metadata,
  children,
}: ToolCallLayoutProps) {
  return (
    <section className="relative flex flex-col gap-5" data-testid="tool-call-card">
      {isFocused && (
        <span
          aria-hidden
          className="absolute left-0 top-2 bottom-2 w-1 rounded-full bg-foreground"
        />
      )}
      <header className="flex gap-3 text-foreground">
        {icon && (
          <span
            className="inline-flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-2xl bg-muted text-lg"
            aria-hidden
          >
            {icon}
          </span>
        )}
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2 text-base font-semibold leading-tight">
                <span className="truncate">{toolName}</span>
                {callId && (
                  <Badge variant="outline" className="break-all text-[10px] font-medium">
                    {callId}
                  </Badge>
                )}
              </div>
              {summary && (
                <p className="text-sm leading-relaxed text-foreground/80">
                  {summary}
                </p>
              )}
            </div>
            <div className="flex-shrink-0">{statusChip}</div>
          </div>
          {metadata && (
            <p className="text-[11px] text-muted-foreground">
              {metadata}
            </p>
          )}
        </div>
      </header>

      <div className="space-y-4">{children}</div>
    </section>
  );
}
