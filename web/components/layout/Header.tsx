"use client";

import { Share2, MoreVertical, Download, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { ReactNode, useState, useRef, useEffect } from "react";
import { EnvironmentStrip } from "@/components/status/EnvironmentStrip";

interface HeaderProps {
  title?: string;
  subtitle?: string;
  onShare?: () => void;
  onExport?: () => void;
  onDelete?: () => void;
  className?: string;
  leadingSlot?: ReactNode;
}

export function Header({
  title,
  subtitle,
  onShare,
  onExport,
  onDelete,
  className,
  leadingSlot,
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
      document.addEventListener("mousedown", handleClickOutside);
      return () =>
        document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [showMenu]);

  return (
    <header
      className={cn(
        "flex items-center justify-between border-b-4 border-border bg-card px-6 py-4 shadow-[6px_6px_0_rgba(0,0,0,0.55)]",
        className,
      )}
    >
      <div className="flex flex-1 items-center gap-4">
        {leadingSlot && (
          <div className="flex items-center">{leadingSlot}</div>
        )}
        <div className="flex-1">
          {title && (
            <h1 className="text-lg font-semibold text-foreground uppercase tracking-[0.14em]">
              {title}
            </h1>
          )}
          {subtitle && (
            <p className="mt-0.5 text-sm text-gray-600 uppercase tracking-[0.12em]">
              {subtitle}
            </p>
          )}
          <EnvironmentStrip />
        </div>
      </div>

      <div className="flex items-center gap-2">
        <div className="relative" ref={menuRef}>
          {showMenu && (
            <div className="absolute right-0 top-full z-50 mt-2 w-48 rounded-lg border-2 border-border bg-card shadow-[6px_6px_0_rgba(0,0,0,0.55)]">
              <div className="py-1">
                {onExport && (
                  <button
                    onClick={() => {
                      onExport();
                      setShowMenu(false);
                    }}
                    className="flex w-full items-center gap-3 px-4 py-2 text-left text-sm uppercase tracking-[0.12em] text-foreground hover:bg-gray-200"
                  >
                    <Download className="h-4 w-4" />
                    <span>{t("header.actions.export")}</span>
                  </button>
                )}
                {onDelete && (
                  <button
                    onClick={() => {
                      onDelete();
                      setShowMenu(false);
                    }}
                    className="flex w-full items-center gap-3 px-4 py-2 text-left text-sm uppercase tracking-[0.12em] text-foreground hover:bg-gray-300"
                  >
                    <Trash2 className="h-4 w-4" />
                    <span>{t("header.actions.delete")}</span>
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
