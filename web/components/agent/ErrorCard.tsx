'use client';

import { ErrorEvent } from '@/lib/types';
import { AlertCircle } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

interface ErrorCardProps {
  event: ErrorEvent;
}

export function ErrorCard({ event }: ErrorCardProps) {
  const t = useTranslation();
  const iterationLabel = typeof event.iteration === 'number' ? event.iteration : '—';
  const phaseLabel = event.phase && event.phase.trim().length > 0 ? event.phase : '—';

  return (
    <section className="space-y-4 px-2 py-3" data-testid="error-event">
      <header className="flex flex-wrap items-baseline gap-3 text-destructive">
        <AlertCircle className="h-6 w-6" aria-hidden />
        <h3 className="text-2xl font-semibold leading-tight tracking-tight sm:text-3xl">
          {t('events.error.title')}
        </h3>
        <span className="text-[10px] uppercase tracking-[0.35em] text-destructive/70">
          {t('events.error.label')}
        </span>
        {event.recoverable && (
          <span className="text-[10px] font-semibold uppercase tracking-[0.35em] text-amber-500">
            {t('events.error.recoverable')}
          </span>
        )}
      </header>

      <p className="text-base font-medium leading-snug text-destructive/80">
        {t('events.error.context', { phase: phaseLabel, iteration: iterationLabel })}
      </p>

      <div className="space-y-2">
        <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-400">
          {t('events.error.details')}
        </p>
        <pre className="whitespace-pre-wrap font-mono text-[12px] leading-relaxed text-destructive">
          {event.error}
        </pre>
      </div>
    </section>
  );
}
