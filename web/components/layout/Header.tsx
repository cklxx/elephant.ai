"use client";

import { type ReactNode } from "react";
import {
  Download,
  MoreVertical,
  Trash2,
} from "lucide-react";

import { EnvironmentStrip } from "@/components/status/EnvironmentStrip";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface HeaderProps {
  title?: string;
  subtitle?: string;
  onExport?: () => void;
  onDelete?: () => void;
  className?: string;
  leadingSlot?: ReactNode;
  actionsSlot?: ReactNode;
  showEnvironmentStrip?: boolean;
}

export function Header({
  title,
  subtitle,
  onExport,
  onDelete,
  className,
  leadingSlot,
  actionsSlot,
  showEnvironmentStrip = true,
}: HeaderProps) {
  const { t } = useI18n();

  const hasMenuActions = Boolean(onExport || onDelete);

  return (
    <header
      className={cn(
        "layout-header flex items-center justify-between rounded-3xl px-4 py-3 shadow-sm backdrop-blur supports-[backdrop-filter]:bg-card/60",
        className,
      )}
    >
      <div className="flex flex-1 min-w-0 items-center gap-3">
        {leadingSlot && <div className="flex items-center">{leadingSlot}</div>}
        <div className="min-w-0 flex-1">
          {title && (
            <h1
              className="truncate text-[15px] font-semibold tracking-tight text-foreground"
              data-testid="console-header-title"
            >
              {title}
            </h1>
          )}
          {subtitle && (
            <p
              className="mt-0.5 text-[12px] text-muted-foreground"
              data-testid="console-header-subtitle"
            >
              {subtitle}
            </p>
          )}
          {showEnvironmentStrip && (
            <div className="mt-2">
              <EnvironmentStrip />
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center gap-3">
        {actionsSlot}
        {hasMenuActions && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-9 w-9 rounded-full border border-border/60 bg-background/50 shadow-sm hover:bg-background/70 hover:text-foreground"
                aria-label={t("header.actions.more")}
              >
                <MoreVertical className="h-4 w-4" aria-hidden />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48 rounded-2xl">
              {onExport && (
                <DropdownMenuItem
                  onClick={() => {
                    onExport();
                  }}
                  className="gap-2"
                >
                  <Download className="h-4 w-4" />
                  {t("header.actions.export")}
                </DropdownMenuItem>
              )}
              {onDelete && (
                <DropdownMenuItem
                  onClick={() => {
                    onDelete();
                  }}
                  className="gap-2 text-destructive focus:text-destructive"
                >
                  <Trash2 className="h-4 w-4" />
                  {t("header.actions.delete")}
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </header>
  );
}
