'use client';

import { supportedLanguages, useI18n } from '@/lib/i18n';
import { cn } from '@/lib/utils';

interface LanguageSwitcherProps {
  className?: string;
  variant?: 'stacked' | 'toolbar';
}

export function LanguageSwitcher({ className, variant = 'stacked' }: LanguageSwitcherProps) {
  const { language, setLanguage, t } = useI18n();

  const control = (
    <div
      className={cn(
        'inline-flex items-center gap-1 rounded-full border border-slate-200 bg-white/80 p-1 shadow-sm backdrop-blur',
        variant === 'toolbar' ? 'px-1.5 py-1.5' : 'px-2 py-2'
      )}
    >
      {supportedLanguages.map((option) => {
        const isActive = option.code === language;
        return (
          <button
            key={option.code}
            type="button"
            onClick={() => setLanguage(option.code)}
            className={cn(
              'min-w-[3.25rem] rounded-full px-3 py-1 text-[13px] font-semibold transition',
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
  );

  if (variant === 'toolbar') {
    return (
      <div className={cn('flex items-center gap-2', className)}>
        <span className="text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-400">
          {t('language.label')}
        </span>
        {control}
      </div>
    );
  }

  return (
    <div className={cn('space-y-2', className)}>
      <span className="text-xs font-medium uppercase tracking-wide text-slate-400">
        {t('language.label')}
      </span>
      {control}
    </div>
  );
}
