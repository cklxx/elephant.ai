"use client";

import { useState } from "react";
import {
  Search,
  Library,
  ListChecks,
  Trash2,
  ChevronDown,
  ChevronRight,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { TranslationKey, useI18n } from "@/lib/i18n";

interface SidebarProps {
  sessionHistory?: string[];
  pinnedSessions?: string[];
  sessionLabels?: Record<string, string>;
  currentSessionId?: string | null;
  onSessionSelect?: (id: string) => void;
  onSessionRename?: (id: string, label: string) => void;
  onSessionPin?: (id: string) => void;
  onSessionDelete?: (id: string) => void;
  onNewSession?: () => void;
}

type NavigationItem = {
  key: string;
  icon: typeof Search;
  label: TranslationKey;
};

const navigationItems: NavigationItem[] = [
  { key: "search", icon: Search, label: "sidebar.nav.search" },
  { key: "library", icon: Library, label: "sidebar.nav.library" },
  { key: "tasks", icon: ListChecks, label: "sidebar.nav.allTasks" },
];

export function Sidebar({
  sessionHistory = [],
  pinnedSessions = [],
  sessionLabels = {},
  currentSessionId = null,
  onSessionSelect,
  onSessionDelete,
  onNewSession,
}: SidebarProps) {
  const { t } = useI18n();
  const [isPinnedCollapsed, setIsPinnedCollapsed] = useState(false);
  const [isRecentCollapsed, setIsRecentCollapsed] = useState(false);

  const getSessionBadge = (value: string) =>
    value.length > 8 ? `${value.slice(0, 4)}…${value.slice(-4)}` : value;

  const renderSessionItem = (id: string) => {
    const isActive = id === currentSessionId;
    const label = sessionLabels[id]?.trim();
    const suffix = id.length > 4 ? id.slice(-4) : id;

    return (
      <li key={id}>
        <div
          className={cn(
            "group flex items-center gap-2 rounded-2xl px-3 py-2 transition backdrop-blur",
            isActive
              ? "bg-white/15 text-foreground"
              : "text-muted-foreground hover:bg-white/10",
          )}
        >
          <button
            onClick={() => onSessionSelect?.(id)}
            className="flex min-w-0 flex-1 flex-col items-start gap-0.5 text-left focus-visible:outline-none"
            data-testid="session-list-item"
            data-session-id={id}
          >
            <span className="w-full truncate text-sm font-medium">
              {label || getSessionBadge(id)}
            </span>
            {label && (
              <span className="w-full truncate text-[10px] font-mono text-gray-400">
                …{suffix}
              </span>
            )}
          </button>
          {onSessionDelete && (
            <button
              type="button"
              onClick={() => onSessionDelete(id)}
              className="rounded-full p-1 text-gray-400 opacity-0 transition hover:bg-white/20 hover:text-foreground focus-visible:opacity-100 focus-visible:outline-none group-hover:opacity-100"
              title={t("sidebar.session.delete")}
              aria-label={t("sidebar.session.delete")}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </li>
    );
  };

  const recentSessions = sessionHistory.filter(
    (id) => !pinnedSessions.includes(id),
  );

  return (
    <aside className="layout-sidebar flex h-screen w-64 flex-col">
      {/* Sessions List */}
      <div className="flex-1 overflow-y-auto p-4">
        <div className="space-y-4">
          {pinnedSessions.length > 0 && (
            <div className="space-y-2" data-testid="session-list-pinned">
              <button
                type="button"
                onClick={() => setIsPinnedCollapsed((prev) => !prev)}
                className="flex w-full items-center justify-between rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-wider text-gray-400 transition hover:bg-white/10 hover:text-foreground"
                aria-expanded={!isPinnedCollapsed}
              >
                <span>{t("sidebar.session.pinned")}</span>
                {isPinnedCollapsed ? (
                  <ChevronRight className="h-3 w-3" aria-hidden />
                ) : (
                  <ChevronDown className="h-3 w-3" aria-hidden />
                )}
              </button>
              {!isPinnedCollapsed && (
                <ul className="space-y-1">
                  {pinnedSessions.map((id) => renderSessionItem(id))}
                </ul>
              )}
            </div>
          )}

          {recentSessions.length > 0 && (
            <div className="space-y-2" data-testid="session-list-recent">
              <button
                type="button"
                onClick={() => setIsRecentCollapsed((prev) => !prev)}
                className="flex w-full items-center justify-between rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-wider text-gray-400 transition hover:bg-white/10 hover:text-foreground"
                aria-expanded={!isRecentCollapsed}
              >
                <span>{t("sidebar.session.recent")}</span>
                {isRecentCollapsed ? (
                  <ChevronRight className="h-3 w-3" aria-hidden />
                ) : (
                  <ChevronDown className="h-3 w-3" aria-hidden />
                )}
              </button>
              {!isRecentCollapsed && (
                <ul className="space-y-1">
                  {recentSessions.map((id) => renderSessionItem(id))}
                </ul>
              )}
            </div>
          )}

          {sessionHistory.length === 0 && (
            <div
              className="flex min-h-[120px] flex-col items-center justify-center rounded-2xl bg-white/5 p-4 text-center backdrop-blur"
              data-testid="session-list-empty"
            >
              <p className="text-sm text-gray-500">
                {t("sidebar.session.empty")}
              </p>
            </div>
          )}
        </div>
      </div>

      {/* New Session Button */}
      <div className="p-4 backdrop-blur">
        <button
          onClick={onNewSession}
          className="console-primary-action w-full !normal-case tracking-normal"
          data-testid="session-list-new"
        >
          {t("sidebar.session.new")}
        </button>
      </div>
    </aside>
  );
}
