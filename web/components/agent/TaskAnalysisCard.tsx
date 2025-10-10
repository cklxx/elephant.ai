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
    <section className="space-y-4 px-2 py-3" data-testid="task-analysis-event">
      <header className="flex flex-wrap items-baseline gap-3 text-slate-900">
        <Target className="h-6 w-6 text-slate-300" aria-hidden />
        <h3 className="text-2xl font-semibold leading-tight tracking-tight sm:text-3xl">
          {t('events.taskAnalysis.title')}
        </h3>
        <span className="text-[10px] uppercase tracking-[0.35em] text-slate-400">
          {t('events.taskAnalysis.label')}
        </span>
      </header>

      {event.action_name && (
        <p className="text-base font-medium leading-snug text-slate-700">
          {event.action_name}
        </p>
      )}

      <div className="space-y-2">
        <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-400">
          {t('events.taskAnalysis.goal')}
        </p>
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-slate-600">
          {event.goal}
        </p>
      </div>
    </section>
  );
}
