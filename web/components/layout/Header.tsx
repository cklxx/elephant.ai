'use client';

import { Share2, MoreVertical, Download, Trash2 } from 'lucide-react';
import { LanguageSwitcher } from '@/components/LanguageSwitcher';
import { useI18n } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { useState, useRef, useEffect } from 'react';

interface HeaderProps {
  title?: string;
  subtitle?: string;
  onShare?: () => void;
  onExport?: () => void;
  onDelete?: () => void;
  className?: string;
}

export function Header({
  title,
  subtitle,
  onShare,
  onExport,
  onDelete,
  className,
}: HeaderProps) {
  const { t } = useI18n();
  const [showMenu, setShowMenu] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false);
      }
    };

    if (showMenu) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showMenu]);

  return (
    <header
      className={cn(
        'flex items-center justify-between border-b border-slate-200 bg-white px-6 py-4',
        className
      )}
    >
      <div className="flex-1">
        {title && (
          <h1 className="text-lg font-semibold text-slate-900">
            {title}
          </h1>
        )}
        {subtitle && (
          <p className="mt-0.5 text-sm text-slate-500">
            {subtitle}
          </p>
        )}
      </div>

      <div className="flex items-center gap-2">
        <LanguageSwitcher variant="toolbar" showLabel={false} />

        {onShare && (
          <button
            onClick={onShare}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-600 transition hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
            title={t('header.actions.share')}
          >
            <Share2 className="h-4 w-4" />
            <span className="hidden sm:inline">{t('header.actions.share')}</span>
          </button>
        )}

        <div className="relative" ref={menuRef}>
          <button
            onClick={() => setShowMenu(!showMenu)}
            className="inline-flex items-center justify-center rounded-lg border border-slate-200 p-1.5 text-slate-600 transition hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
            title={t('header.actions.more')}
          >
            <MoreVertical className="h-4 w-4" />
          </button>

          {showMenu && (
            <div className="absolute right-0 top-full z-50 mt-2 w-48 rounded-lg border border-slate-200 bg-white shadow-lg">
              <div className="py-1">
                {onExport && (
                  <button
                    onClick={() => {
                      onExport();
                      setShowMenu(false);
                    }}
                    className="flex w-full items-center gap-3 px-4 py-2 text-left text-sm text-slate-700 hover:bg-slate-50"
                  >
                    <Download className="h-4 w-4" />
                    <span>{t('header.actions.export')}</span>
                  </button>
                )}
                {onDelete && (
                  <button
                    onClick={() => {
                      onDelete();
                      setShowMenu(false);
                    }}
                    className="flex w-full items-center gap-3 px-4 py-2 text-left text-sm text-red-600 hover:bg-red-50"
                  >
                    <Trash2 className="h-4 w-4" />
                    <span>{t('header.actions.delete')}</span>
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
