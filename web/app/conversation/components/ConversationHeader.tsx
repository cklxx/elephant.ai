import {
  Loader2,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  Share2,
  Sparkles,
} from "lucide-react";
import { memo } from "react";

import { Header } from "@/components/layout";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

interface ConversationHeaderProps {
  title: string;
  isSidebarOpen: boolean;
  onToggleSidebar: () => void;
  onOpenPersona: () => void;
  streamSessionId: string | null;
  onToggleRightPanel: () => void;
  isRightPanelOpen: boolean;
  onShare: () => void;
  shareInProgress: boolean;
  shareDisabled: boolean;
}

export const ConversationHeader = memo(function ConversationHeader({
  title,
  isSidebarOpen,
  onToggleSidebar,
  onOpenPersona,
  streamSessionId,
  onToggleRightPanel,
  isRightPanelOpen,
  onShare,
  shareInProgress,
  shareDisabled,
}: ConversationHeaderProps) {
  const { t } = useI18n();

  return (
    <Header
      title={title}
      showEnvironmentStrip
      leadingSlot={
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            data-testid="session-list-toggle"
            onClick={onToggleSidebar}
            className="h-10 w-10 rounded-full border border-border/60 bg-background/50 shadow-sm hover:bg-background/70 hover:text-foreground"
            aria-expanded={isSidebarOpen}
            aria-controls="conversation-sidebar"
          >
            {isSidebarOpen ? (
              <PanelLeftClose className="h-4 w-4" />
            ) : (
              <PanelLeftOpen className="h-4 w-4" />
            )}
            <span className="sr-only">
              {isSidebarOpen
                ? t("sidebar.toggle.close")
                : t("sidebar.toggle.open")}
            </span>
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onOpenPersona}
            className="h-10 rounded-full border border-border/60 bg-background/50 px-3 text-xs font-semibold shadow-sm hover:bg-background/70"
            disabled={!streamSessionId}
          >
            <Sparkles className="h-4 w-4" aria-hidden />
            主动询问
          </Button>
        </div>
      }
      actionsSlot={
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onShare}
            disabled={shareDisabled}
            className="h-10 w-10 rounded-full border border-border/60 bg-background/50 shadow-sm hover:bg-background/70 hover:text-foreground"
            aria-label={t("header.actions.share")}
          >
            {shareInProgress ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Share2 className="h-4 w-4" />
            )}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            data-testid="right-panel-toggle"
            onClick={onToggleRightPanel}
            className="h-10 w-10 rounded-full border border-border/60 bg-background/50 shadow-sm hover:bg-background/70 hover:text-foreground"
            aria-expanded={isRightPanelOpen}
            aria-controls="conversation-right-panel"
          >
            {isRightPanelOpen ? (
              <PanelRightClose className="h-4 w-4" />
            ) : (
              <PanelRightOpen className="h-4 w-4" />
            )}
            <span className="sr-only">
              {isRightPanelOpen ? "Close right panel" : "Open right panel"}
            </span>
          </Button>
        </div>
      }
    />
  );
});
