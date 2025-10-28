'use client';

import { TaskCompleteEvent } from '@/lib/types';
import { CheckCircle2 } from 'lucide-react';
import { formatDuration } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import { MarkdownRenderer } from '@/components/ui/markdown';

interface TaskCompleteCardProps {
  event: TaskCompleteEvent;
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  const t = useTranslation();

  const metrics: string[] = [];
  if (typeof event.total_iterations === 'number') {
    metrics.push(t('events.taskComplete.metrics.iterations', { count: event.total_iterations }));
  }
  if (typeof event.total_tokens === 'number') {
    metrics.push(t('events.taskComplete.metrics.tokens', { count: event.total_tokens }));
  }
  if (typeof event.duration === 'number') {
    metrics.push(t('events.taskComplete.metrics.duration', { duration: formatDuration(event.duration) }));
  }

  const stopReason = event.stop_reason && event.stop_reason.trim().length > 0 ? event.stop_reason : '—';

  return (
    <section className="space-y-5 px-2 py-3" data-testid="task-complete-event">
      <header className="flex flex-wrap items-baseline gap-3 text-emerald-700">
        <CheckCircle2 className="h-6 w-6" aria-hidden />
        <h3 className="text-2xl font-semibold leading-tight tracking-tight sm:text-3xl">
          {t('events.taskComplete.title')}
        </h3>
        <span className="text-[10px] uppercase tracking-[0.35em] text-emerald-400">
          {t('events.taskComplete.label')}
        </span>
      </header>

      {metrics.length > 0 && (
        <ul className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] font-medium uppercase tracking-[0.28em] text-emerald-500">
          {metrics.map((metric, index) => (
            <li key={`${metric}-${index}`} className="text-emerald-600">
              {metric}
            </li>
          ))}
        </ul>
      )}

      <div className="space-y-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-400">
          {t('events.taskComplete.finalAnswer')}
        </p>
        {event.final_answer ? (
          <MarkdownRenderer
            content={event.final_answer}
            className="prose prose-sm max-w-none text-slate-700"
          />
        ) : (
          <p className="text-sm text-slate-500">{t('events.taskComplete.empty')}</p>
        )}
      </div>

      <footer className="text-[11px] uppercase tracking-[0.28em] text-slate-400">
        {t('events.taskComplete.stopReason', { reason: stopReason })}
      </footer>
    </section>
  );
}
