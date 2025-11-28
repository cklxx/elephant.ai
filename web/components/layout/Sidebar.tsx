"use client";

import { useMemo, useState } from "react";
import { ChevronDown, ChevronRight, Library, ListChecks, Search, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Card } from "@/components/ui/card";
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
            "group flex items-center gap-2 rounded-2xl border border-transparent px-3 py-2 transition",
            isActive
              ? "border-border/80 bg-primary/5 text-foreground"
              : "text-muted-foreground hover:border-border/60 hover:bg-muted/40",
          )}
        >
          <button
            onClick={() => onSessionSelect?.(id)}
            className="flex min-w-0 flex-1 flex-col items-start gap-0.5 text-left focus-visible:outline-none"
            data-testid="session-list-item"
            data-session-id={id}
          >
            <span className="w-full truncate text-sm font-medium text-foreground">
              {label || getSessionBadge(id)}
            </span>
            {label && (
              <span className="w-full truncate text-[10px] font-mono text-muted-foreground">
                …{suffix}
              </span>
            )}
          </button>
          {onSessionDelete && (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => onSessionDelete(id)}
              className="h-8 w-8 opacity-0 transition group-hover:opacity-100"
              title={t("sidebar.session.delete")}
              aria-label={t("sidebar.session.delete")}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      </li>
    );
  };

  const recentSessions = useMemo(
    () => sessionHistory.filter((id) => !pinnedSessions.includes(id)),
    [sessionHistory, pinnedSessions],
  );

  return (
    <aside className="h-full">
      <Card className="flex h-full flex-col rounded-3xl border border-border/60 bg-card/80 p-4">
        <div className="mb-3 flex items-center justify-between text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
          <span>{t("sidebar.session.title") ?? "Sessions"}</span>
          <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
            {sessionHistory.length || 0}
          </span>
        </div>

        <div className="mb-3 grid grid-cols-3 gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          {navigationItems.map((item) => {
            const Icon = item.icon;
            return (
              <div
                key={item.key}
                className="flex items-center gap-1 rounded-xl border border-border/60 bg-background/60 px-2 py-1"
              >
                <Icon className="h-3.5 w-3.5" aria-hidden />
                <span className="truncate">{t(item.label)}</span>
              </div>
            );
          })}
        </div>

        <ScrollArea className="flex-1 rounded-2xl border border-dashed border-border/60 bg-background/40">
          <div className="space-y-4 p-3 pr-2">
            {pinnedSessions.length > 0 && (
              <div className="space-y-2" data-testid="session-list-pinned">
                <button
                  type="button"
                  onClick={() => setIsPinnedCollapsed((prev) => !prev)}
                  className="flex w-full items-center justify-between rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground transition hover:text-foreground"
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
                  className="flex w-full items-center justify-between rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground transition hover:text-foreground"
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
                className="flex min-h-[120px] flex-col items-center justify-center rounded-2xl border border-dashed border-border/60 bg-background/60 p-4 text-center"
                data-testid="session-list-empty"
              >
                <p className="text-sm text-muted-foreground">
                  {t("sidebar.session.empty")}
                </p>
              </div>
            )}
          </div>
        </ScrollArea>

        <Button
          onClick={onNewSession}
          className="mt-4 w-full"
          data-testid="session-list-new"
        >
          {t("sidebar.session.new")}
        </Button>
      </Card>
    </aside>
  );
}
