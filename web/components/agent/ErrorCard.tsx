'use client';

import { WorkflowNodeFailedEvent } from '@/lib/types';
import { AlertCircle } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

interface ErrorCardProps {
  event: WorkflowNodeFailedEvent;
}

export function ErrorCard({ event }: ErrorCardProps) {
  const t = useTranslation();
  const iterationLabel = typeof event.iteration === 'number' ? event.iteration : '—';
  const phaseLabel = event.phase && event.phase.trim().length > 0 ? event.phase : '—';

  return (
    <section className="space-y-4" data-testid="error-event">
      <header className="flex flex-wrap items-center gap-3 text-destructive">
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border-2 border-destructive bg-destructive/5">
          <AlertCircle className="h-4 w-4" aria-hidden />
        </span>
        <h3 className="text-lg font-semibold">
          {t('events.error.title')}
        </h3>
        <span className="text-[11px] font-medium text-destructive/70">
          {t('events.error.label')}
        </span>
        {event.recoverable && (
          <span className="text-[11px] font-semibold text-foreground">
            {t('events.error.recoverable')}
          </span>
        )}
      </header>

      <p className="text-sm font-semibold leading-snug text-destructive">
        {t('events.error.context', { phase: phaseLabel, iteration: iterationLabel })}
      </p>

      <div className="space-y-2">
        <p className="text-[11px] font-semibold text-muted-foreground">
          {t('events.error.details')}
        </p>
        <pre className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 font-mono text-xs leading-relaxed text-destructive">
          {event.error}
        </pre>
      </div>
    </section>
  );
}
