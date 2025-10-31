'use client';

import { ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface ToolCallLayoutProps {
  toolName: string;
  icon?: ReactNode;
  callId?: string | null;
  statusChip: ReactNode;
  isFocused?: boolean;
  metadata?: ReactNode;
  children: ReactNode;
}

export function ToolCallLayout({
  toolName,
  icon,
  callId,
  statusChip,
  isFocused = false,
  metadata,
  children,
}: ToolCallLayoutProps) {
  return (
    <section className="relative space-y-5" data-testid="tool-call-card">
      {isFocused && (
        <span
          aria-hidden
          className="absolute left-0 top-2 bottom-2 w-1 rounded-full bg-foreground"
        />
      )}
      <header className="flex flex-wrap items-center gap-2 text-foreground text-[10px] uppercase tracking-[0.24em]">
        {icon && <span className="text-sm text-muted-foreground">{icon}</span>}
        <h3 className="text-[11px] font-semibold tracking-[0.2em] text-foreground">{toolName}</h3>
        <span className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
          TOOL CALL
        </span>
        {statusChip}
      </header>

      {callId && (
        <p className="console-microcopy uppercase tracking-[0.24em] text-muted-foreground">
          ID ·{' '}
          <span className="font-mono text-[10px] tracking-normal text-foreground/70">{callId}</span>
        </p>
      )}

      {metadata && <div className={cn('text-[10px] uppercase tracking-[0.2em] text-muted-foreground')}>{metadata}</div>}

      <div className="space-y-4">{children}</div>
    </section>
  );
}
