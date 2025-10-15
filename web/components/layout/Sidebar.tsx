'use client';

import { useState } from 'react';
import { Search, Library, ListChecks, Pin, Pencil, Check, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { TranslationKey, useI18n } from '@/lib/i18n';

interface SidebarProps {
  sessionHistory?: string[];
  pinnedSessions?: string[];
  sessionLabels?: Record<string, string>;
  currentSessionId?: string | null;
  onSessionSelect?: (id: string) => void;
  onSessionRename?: (id: string, label: string) => void;
  onSessionPin?: (id: string) => void;
  onNewSession?: () => void;
}

type NavigationItem = {
  key: string;
  icon: typeof Search;
  label: TranslationKey;
};

const navigationItems: NavigationItem[] = [
  { key: 'search', icon: Search, label: 'sidebar.nav.search' },
  { key: 'library', icon: Library, label: 'sidebar.nav.library' },
  { key: 'tasks', icon: ListChecks, label: 'sidebar.nav.allTasks' },
];

export function Sidebar({
  sessionHistory = [],
  pinnedSessions = [],
  sessionLabels = {},
  currentSessionId = null,
  onSessionSelect,
  onSessionRename,
  onSessionPin,
  onNewSession,
}: SidebarProps) {
  const { t } = useI18n();
  const [activeNav, setActiveNav] = useState<string>('tasks');
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState('');

  const handleNavClick = (key: string) => {
    setActiveNav(key);
    if (key === 'search') {
      // TODO: Implement search functionality
    }
  };

  const handleRenameOpen = (id: string) => {
    setEditingSessionId(id);
    setEditingValue(sessionLabels[id] ?? '');
  };

  const handleRenameSubmit = (id: string) => {
    onSessionRename?.(id, editingValue);
    setEditingSessionId(null);
    setEditingValue('');
  };

  const handleRenameCancel = () => {
    setEditingSessionId(null);
    setEditingValue('');
  };

  const getSessionBadge = (value: string) =>
    value.length > 8 ? `${value.slice(0, 4)}…${value.slice(-4)}` : value;

  const renderSessionItem = (id: string, pinned = false) => {
    const isActive = id === currentSessionId;
    const label = sessionLabels[id]?.trim();
    const suffix = id.length > 4 ? id.slice(-4) : id;
    const isEditing = editingSessionId === id;
    const isPinned = pinned || pinnedSessions.includes(id);

    if (isEditing) {
      return (
        <li key={id}>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              handleRenameSubmit(id);
            }}
            className="flex items-center gap-2 rounded-lg border border-sky-200 bg-white px-3 py-2"
          >
            <input
              value={editingValue}
              onChange={(event) => setEditingValue(event.target.value)}
              placeholder={t('sidebar.session.renamePlaceholder')}
              className="flex-1 bg-transparent text-sm text-slate-700 placeholder:text-slate-300 focus:outline-none"
              maxLength={48}
              autoFocus
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  event.preventDefault();
                  handleRenameCancel();
                }
              }}
            />
            <button
              type="submit"
              className="rounded-full bg-sky-500 p-1 text-white hover:bg-sky-600"
              title={t('sidebar.session.confirmRename')}
            >
              <Check className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              onClick={handleRenameCancel}
              className="rounded-full bg-slate-100 p-1 text-slate-500 hover:bg-slate-200"
              title={t('sidebar.session.cancelRename')}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </form>
        </li>
      );
    }

    return (
      <li key={id}>
        <div
          className={cn(
            'group flex items-center gap-2 rounded-lg px-3 py-2 transition',
            isActive
              ? 'bg-sky-500/10 text-sky-700 ring-1 ring-inset ring-sky-400/50'
              : 'text-slate-600 hover:bg-slate-50'
          )}
        >
          <button
            onClick={() => onSessionSelect?.(id)}
            className="flex flex-1 flex-col items-start gap-0.5 text-left focus-visible:outline-none"
          >
            <span className="text-sm font-medium">
              {label || getSessionBadge(id)}
            </span>
            {label && (
              <span className="text-[10px] font-mono text-slate-400">
                …{suffix}
              </span>
            )}
          </button>
          <div className="flex items-center gap-1 opacity-0 transition group-hover:opacity-100">
            <button
              type="button"
              onClick={() => onSessionPin?.(id)}
              className="rounded-full p-1 text-slate-400 hover:bg-slate-200 hover:text-slate-600"
              title={t(isPinned ? 'sidebar.session.unpin' : 'sidebar.session.pin')}
            >
              <Pin
                className={cn('h-3.5 w-3.5', isPinned && '-rotate-45 text-sky-500')}
                fill={isPinned ? 'currentColor' : 'none'}
              />
            </button>
            <button
              type="button"
              onClick={() => handleRenameOpen(id)}
              className="rounded-full p-1 text-slate-400 hover:bg-slate-200 hover:text-slate-600"
              title={t('sidebar.session.rename')}
            >
              <Pencil className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </li>
    );
  };

  const recentSessions = sessionHistory.filter((id) => !pinnedSessions.includes(id));

  return (
    <aside className="flex h-screen w-64 flex-col border-r border-slate-200 bg-white">
      {/* Navigation Items */}
      <nav className="border-b border-slate-200 p-4">
        <ul className="space-y-1">
          {navigationItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeNav === item.key;
            return (
              <li key={item.key}>
                <button
                  onClick={() => handleNavClick(item.key)}
                  className={cn(
                    'flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition',
                    isActive
                      ? 'bg-sky-500 text-white'
                      : 'text-slate-600 hover:bg-slate-50'
                  )}
                >
                  <Icon className="h-4 w-4" />
                  <span>{t(item.label)}</span>
                </button>
              </li>
            );
          })}
        </ul>
      </nav>

      {/* Sessions List */}
      <div className="flex-1 overflow-y-auto p-4">
        <div className="space-y-4">
          {pinnedSessions.length > 0 && (
            <div className="space-y-2">
              <p className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">
                {t('sidebar.session.pinned')}
              </p>
              <ul className="space-y-1">
                {pinnedSessions.map((id) => renderSessionItem(id, true))}
              </ul>
            </div>
          )}

          {recentSessions.length > 0 && (
            <div className="space-y-2">
              <p className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">
                {t('sidebar.session.recent')}
              </p>
              <ul className="space-y-1">
                {recentSessions.map((id) => renderSessionItem(id))}
              </ul>
            </div>
          )}

          {sessionHistory.length === 0 && (
            <div className="flex min-h-[120px] flex-col items-center justify-center rounded-lg border border-dashed border-slate-200 bg-slate-50/50 p-4 text-center">
              <p className="text-sm text-slate-500">
                {t('sidebar.session.empty')}
              </p>
            </div>
          )}
        </div>
      </div>

      {/* New Session Button */}
      <div className="border-t border-slate-200 p-4">
        <button
          onClick={onNewSession}
          className="w-full rounded-lg bg-sky-500 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
        >
          {t('sidebar.session.new')}
        </button>
      </div>
    </aside>
  );
}
