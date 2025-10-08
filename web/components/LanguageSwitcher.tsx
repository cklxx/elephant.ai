'use client';

import { supportedLanguages, useI18n } from '@/lib/i18n';
import { cn } from '@/lib/utils';

interface LanguageSwitcherProps {
  className?: string;
}

export function LanguageSwitcher({ className }: LanguageSwitcherProps) {
  const { language, setLanguage, t } = useI18n();

  return (
    <div className={cn('space-y-2', className)}>
      <span className="text-xs font-medium uppercase tracking-wide text-slate-400">
        {t('language.label')}
      </span>
      <div className="inline-flex gap-1 rounded-2xl border border-slate-200 bg-white/80 p-1 shadow-sm">
        {supportedLanguages.map((option) => {
          const isActive = option.code === language;
          return (
            <button
              key={option.code}
              type="button"
              onClick={() => setLanguage(option.code)}
              className={cn(
                'min-w-[3.25rem] rounded-xl px-3 py-1.5 text-xs font-semibold transition',
                isActive
                  ? 'bg-sky-500 text-white shadow-sm'
                  : 'text-slate-500 hover:bg-slate-100'
              )}
              aria-pressed={isActive}
              title={t(option.labelKey)}
            >
              <span className="hidden sm:inline">{t(option.labelKey)}</span>
              <span className="sm:hidden">{t(option.shortKey)}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
