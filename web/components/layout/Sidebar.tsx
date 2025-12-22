"use client";

import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";

interface SidebarProps {
  sessionHistory?: string[];
  sessionLabels?: Record<string, string>;
  currentSessionId?: string | null;
  onSessionSelect?: (id: string) => void;
  onSessionDelete?: (id: string) => void;
  onNewSession?: () => void;
}

export function Sidebar({
  sessionHistory = [],
  sessionLabels = {},
  currentSessionId = null,
  onSessionSelect,
  onSessionDelete,
  onNewSession,
}: SidebarProps) {
  const { t } = useI18n();

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
            "group relative flex items-center gap-2 rounded-2xl border border-transparent px-3 py-2 transition",
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
              className="relative z-10 h-8 w-8 shrink-0 text-muted-foreground transition hover:text-destructive focus-visible:text-destructive"
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

  return (
    <aside className="h-full">
      <div className="flex h-full flex-col rounded-2xl border border-border/60 bg-card p-4">
        <ScrollArea
          className="flex-1"
          viewportClassName="[&>div]:!block [&>div]:!min-w-0 [&>div]:!w-full"
        >
          <div className="space-y-2 p-1 pr-2" data-testid="session-list">
            {sessionHistory.length > 0 ? (
              <ul className="space-y-1">
                {sessionHistory.map((id) => renderSessionItem(id))}
              </ul>
            ) : (
              <div
                className="flex min-h-[120px] flex-col items-center justify-center rounded-2xl bg-muted/40 p-4 text-center"
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
      </div>
    </aside>
  );
}
