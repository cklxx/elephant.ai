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
    <section className="space-y-5" data-testid="task-complete-event">
      <header className="flex flex-wrap items-center gap-3 text-foreground">
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border-2 border-foreground bg-card shadow-[3px_3px_0_rgba(0,0,0,0.6)]">
          <CheckCircle2 className="h-4 w-4" aria-hidden />
        </span>
        <h3 className="text-lg font-semibold uppercase tracking-[0.2em]">
          {t('events.taskComplete.title')}
        </h3>
        <span className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
          {t('events.taskComplete.label')}
        </span>
      </header>

      {metrics.length > 0 && (
        <ul className="flex flex-wrap items-center gap-x-4 gap-y-2 text-[11px] font-semibold uppercase tracking-[0.28em] text-foreground">
          {metrics.map((metric, index) => (
            <li key={`${metric}-${index}`} className="console-quiet-chip">
              {metric}
            </li>
          ))}
        </ul>
      )}

      <div className="space-y-3">
        <p className="console-microcopy font-semibold uppercase tracking-[0.28em] text-muted-foreground">
          {t('events.taskComplete.finalAnswer')}
        </p>
        {event.final_answer ? (
          <MarkdownRenderer
            content={event.final_answer}
            className="prose prose-sm max-w-none text-foreground/80"
          />
        ) : (
          <p className="text-sm text-muted-foreground">{t('events.taskComplete.empty')}</p>
        )}
      </div>

      <footer className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
        {t('events.taskComplete.stopReason', { reason: stopReason })}
      </footer>
    </section>
  );
}
