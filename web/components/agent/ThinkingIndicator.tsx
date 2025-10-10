'use client';

import { Brain, Loader2 } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

export function ThinkingIndicator() {
  const t = useTranslation();

  return (
    <div className="flex items-center gap-4 px-1 py-3" data-testid="thinking-event">
      <Brain className="h-5 w-5 flex-shrink-0 text-slate-400" aria-hidden />
      <div className="flex flex-col gap-1">
        <span className="text-xl font-semibold leading-tight text-slate-700 sm:text-2xl">
          {t('events.thinking.title')}
        </span>
        <span className="text-[11px] uppercase tracking-[0.3em] text-slate-400">
          {t('events.thinking.hint')}
        </span>
      </div>
      <Loader2 className="ml-auto h-4 w-4 animate-spin text-slate-400" aria-hidden />
    </div>
  );
}
