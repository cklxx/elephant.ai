import { memo, useEffect } from "react";
import dynamic from "next/dynamic";
import { Loader2, PanelRightClose } from "lucide-react";

import { Sidebar, ContentArea } from "@/components/layout";
import { TaskInput } from "@/components/agent/TaskInput";
import { AttachmentPanel } from "@/components/agent/AttachmentPanel";
import { SkillsPanel } from "@/components/agent/SkillsPanel";
import { ConnectionBanner } from "@/components/agent/ConnectionBanner";
import { SmartErrorBoundary } from "@/components/SmartErrorBoundary";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { AnyAgentEvent, AttachmentUpload } from "@/lib/types";

const LazyConversationEventStream = dynamic(
  () =>
    import("@/components/agent/ConversationEventStream").then(
      (mod) => mod.ConversationEventStream,
    ),
  {
    ssr: false,
    loading: () => (
      <div className="rounded-2xl border border-dashed border-border/60 bg-card/60 p-4 text-sm text-muted-foreground">
        Preparing event stream…
      </div>
    ),
  },
);

interface ConversationStreamProps {
  events: AnyAgentEvent[];
  hasRenderableEvents: boolean;
  streamIsRunning: boolean;
  streamSessionId: string | null;
}

interface ConversationConnectionProps {
  showConnectingState: boolean;
  showConnectionBanner: boolean;
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts: number;
  onReconnect: () => void;
}

interface ConversationSidebarProps {
  isSidebarOpen: boolean;
  sessionHistory: string[];
  sessionLabels: Record<string, string | undefined>;
  resolvedSessionId: string | null;
  onSessionSelect: (id: string) => void;
  onSessionDelete: (id: string) => void;
  onNewSession: () => void;
}

interface ConversationRightPanelProps {
  isRightPanelOpen: boolean;
  onCloseRightPanel: () => void;
  hasAttachments: boolean;
}

interface ConversationComposerProps {
  emptyState: React.ReactNode;
  loadingText: string;
  inputPlaceholder: string;
  creationPending: boolean;
  inputDisabled: boolean;
  prefillTask: string | null;
  onPrefillApplied: () => void;
  onSubmit: (task: string, attachments: AttachmentUpload[]) => void;
  onStop: () => void;
  isTaskRunning: boolean;
  stopPending: boolean;
  stopDisabled: boolean;
}

interface ConversationMainAreaProps {
  contentRef: React.RefObject<HTMLDivElement | null>;
  stream: ConversationStreamProps;
  connection: ConversationConnectionProps;
  sidebar: ConversationSidebarProps;
  rightPanel: ConversationRightPanelProps;
  composer: ConversationComposerProps;
}

export const ConversationMainArea = memo(function ConversationMainArea({
  contentRef,
  stream,
  connection,
  sidebar,
  rightPanel,
  composer,
}: ConversationMainAreaProps) {
  useEffect(() => {
    void import("@/components/agent/ConversationEventStream");
  }, []);

  const {
    events,
    hasRenderableEvents,
    streamIsRunning,
    streamSessionId,
  } = stream;
  const {
    showConnectingState,
    showConnectionBanner,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    onReconnect,
  } = connection;
  const {
    isSidebarOpen,
    sessionHistory,
    sessionLabels,
    resolvedSessionId,
    onSessionSelect,
    onSessionDelete,
    onNewSession,
  } = sidebar;
  const { isRightPanelOpen, onCloseRightPanel, hasAttachments } = rightPanel;
  const {
    emptyState,
    loadingText,
    inputPlaceholder,
    creationPending,
    inputDisabled,
    prefillTask,
    onPrefillApplied,
    onSubmit,
    onStop,
    isTaskRunning,
    stopPending,
    stopDisabled,
  } = composer;

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-5 overflow-hidden lg:flex-row">
      <div
        id="conversation-sidebar"
        className={cn(
          "overflow-hidden transition-all duration-300 lg:w-72 lg:flex-none",
          isSidebarOpen ? "block" : "hidden",
        )}
        aria-hidden={!isSidebarOpen}
      >
        <Sidebar
          sessionHistory={sessionHistory}
          sessionLabels={sessionLabels}
          currentSessionId={resolvedSessionId}
          onSessionSelect={onSessionSelect}
          onSessionDelete={onSessionDelete}
          onNewSession={onNewSession}
        />
      </div>

      <div className="flex flex-1 min-h-0 min-w-0 flex-col overflow-hidden rounded-3xl">
        <ContentArea
          ref={contentRef}
          className="flex-1 min-h-0 min-w-0"
          fullWidth
          contentClassName="space-y-4"
        >
          {!hasRenderableEvents ? (
            <div className="flex min-h-[60vh] items-center justify-center">
              {showConnectingState ? (
                <div className="flex flex-col items-center gap-3 rounded-3xl border border-border/60 bg-background/70 px-8 py-6 text-center">
                  <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">{loadingText}</p>
                </div>
              ) : showConnectionBanner ? (
                <ConnectionBanner
                  isConnected={isConnected}
                  isReconnecting={isReconnecting}
                  error={error}
                  reconnectAttempts={reconnectAttempts}
                  onReconnect={onReconnect}
                />
              ) : (
                emptyState
              )}
            </div>
          ) : (
            <SmartErrorBoundary level="section">
              <LazyConversationEventStream
                events={events}
                isConnected={isConnected}
                isReconnecting={isReconnecting}
                error={error}
                reconnectAttempts={reconnectAttempts}
                onReconnect={onReconnect}
                isRunning={streamIsRunning}
              />
            </SmartErrorBoundary>
          )}
        </ContentArea>

        <div className="border-t px-3 py-4 sm:px-6 sm:py-6">
          <div className="space-y-4">
            <TaskInput
              onSubmit={onSubmit}
              placeholder={inputPlaceholder}
              disabled={inputDisabled}
              loading={creationPending}
              prefill={prefillTask}
              onPrefillApplied={onPrefillApplied}
              onStop={onStop}
              isRunning={isTaskRunning}
              stopPending={stopPending}
              stopDisabled={stopDisabled}
            />
          </div>
        </div>
      </div>

      <div
        id="conversation-right-panel"
        className={cn(
          "hidden lg:flex flex-none justify-end overflow-hidden transition-all duration-300",
          isRightPanelOpen ? "w-[380px] xl:w-[440px]" : "w-0",
        )}
        aria-hidden={!isRightPanelOpen}
      >
        {isRightPanelOpen ? (
          <div className="sticky top-24 w-full max-w-[440px] space-y-4">
            <SkillsPanel />
            {hasAttachments ? (
              <AttachmentPanel events={events} />
            ) : (
              <div className="rounded-3xl border border-dashed border-border/60 bg-card/60 p-4 text-sm text-muted-foreground">
                No attachments yet.
              </div>
            )}
          </div>
        ) : null}
      </div>

      {isRightPanelOpen && (
        <div className="fixed inset-0 z-50 flex lg:hidden">
          <button
            type="button"
            className="absolute inset-0 h-full w-full bg-black/30"
            aria-label="Close right panel"
            onClick={onCloseRightPanel}
          />
          <aside
            className="relative ml-auto flex h-full w-full max-w-[440px] flex-col border-l border-border/60 bg-card"
            aria-label="Resources panel"
          >
            <header className="flex items-center justify-between gap-3 border-b border-border/60 px-4 py-3">
              <h2 className="text-sm font-semibold text-foreground">
                Resources
              </h2>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-9 w-9 rounded-full"
                onClick={onCloseRightPanel}
                aria-label="Close resources panel"
              >
                <PanelRightClose className="h-4 w-4" />
              </Button>
            </header>

            <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4">
              <SkillsPanel />
              {hasAttachments ? (
                <AttachmentPanel events={events} />
              ) : (
                <div className="rounded-3xl border border-dashed border-border/60 bg-card/60 p-4 text-sm text-muted-foreground">
                  No attachments yet.
                </div>
              )}
            </div>
          </aside>
        </div>
      )}
    </div>
  );
});
