'use client';

import { Brain, Loader2 } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

export function ThinkingIndicator() {
  const t = useTranslation();

  return (
    <div className="flex items-center gap-4" data-testid="thinking-event">
      <span className="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full border-2 border-border bg-card shadow-[3px_3px_0_rgba(0,0,0,0.55)]">
        <Brain className="h-4 w-4 text-foreground" aria-hidden />
      </span>
      <div className="flex flex-col gap-1">
        <span className="text-base font-semibold uppercase tracking-[0.18em] text-foreground">
          {t('events.thinking.title')}
        </span>
        <span className="console-microcopy uppercase tracking-[0.24em] text-muted-foreground">
          {t('events.thinking.hint')}
        </span>
      </div>
      <Loader2 className="ml-auto h-4 w-4 animate-spin text-muted-foreground" aria-hidden />
    </div>
  );
}
