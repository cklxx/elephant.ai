'use client';

import { TaskAnalysisEvent } from '@/lib/types';
import { Target } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

interface TaskAnalysisCardProps {
  event: TaskAnalysisEvent;
}

export function TaskAnalysisCard({ event }: TaskAnalysisCardProps) {
  const t = useTranslation();

  return (
    <section className="space-y-4" data-testid="task-analysis-event">
      <header className="flex flex-wrap items-center gap-3 text-foreground">
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border-2 border-border bg-card shadow-[3px_3px_0_rgba(0,0,0,0.6)]">
          <Target className="h-4 w-4" aria-hidden />
        </span>
        <h3 className="text-lg font-semibold uppercase tracking-[0.2em]">
          {t('events.taskAnalysis.title')}
        </h3>
        <span className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
          {t('events.taskAnalysis.label')}
        </span>
      </header>

      {event.action_name && (
        <p className="text-base font-semibold leading-snug text-foreground">
          {event.action_name}
        </p>
      )}

      <div className="space-y-2">
        <p className="console-microcopy font-semibold uppercase tracking-[0.28em] text-muted-foreground">
          {t('events.taskAnalysis.goal')}
        </p>
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/80">
          {event.goal}
        </p>
      </div>
    </section>
  );
}
