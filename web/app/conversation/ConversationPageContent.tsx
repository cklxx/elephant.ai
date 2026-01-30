"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Copy, Film, FileText, Image, Loader2 } from "lucide-react";

import { useAgentEventStream } from "@/hooks/useAgentEventStream";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";
import type { AnyAgentEvent } from "@/lib/types";
import { captureEvent } from "@/lib/analytics/posthog";
import { AnalyticsEvent } from "@/lib/analytics/events";
import { performanceMonitor } from "@/lib/analytics/performance";
import { collectAttachmentItems } from "@/components/agent/AttachmentPanel";
import { UserPersonaDialog } from "@/components/agent/UserPersonaDialog";
import { LLMIndicator } from "@/components/agent/LLMIndicator";
import { isEventType } from "@/lib/events/matching";

import { useConversationSession } from "./hooks/useConversationSession";
import { useCancellation } from "./hooks/useCancellation";
import { useTaskSubmission } from "./hooks/useTaskSubmission";
import { useShareDialog } from "./hooks/useShareDialog";
import { useDeleteDialog } from "./hooks/useDeleteDialog";
import { ConversationHeader } from "./components/ConversationHeader";
import { ConversationMainArea } from "./components/ConversationMainArea";
import { EmptyStateView } from "./components/EmptyStateView";
import type { QuickPromptItem } from "./components/QuickPromptButtons";

export function ConversationPageContent() {
  const [, setTaskId] = useState<string | null>(null);
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [isRightPanelOpen, setIsRightPanelOpen] = useState(false);
  const [personaDialogOpen, setPersonaDialogOpen] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const pendingSubmitTsRef = useRef<number | null>(null);
  const submittedAtByTaskRef = useRef<Map<string, number>>(new Map());
  const firstTokenReportedRef = useRef<Set<string>>(new Set());
  const searchParams = useSearchParams();
  const { t } = useI18n();

  const useMockStream = useMemo(
    () => searchParams.get("mockSSE") === "1",
    [searchParams],
  );

  const session = useConversationSession({ useMockStream });

  const {
    cancelRequested,
    setCancelRequested,
    cancelIntentRef,
    activeTaskIdRef,
    performCancellation,
    handleStop,
    isCancelPending,
  } = useCancellation({
    activeTaskId,
    resolvedSessionId: session.resolvedSessionId,
    useMockStream,
    setActiveTaskId,
  });

  const resetTaskState = useCallback(() => {
    setTaskId(null);
    setActiveTaskId(null);
    setCancelRequested(false);
    cancelIntentRef.current = false;
  }, [cancelIntentRef, setCancelRequested]);

  const handleAgentEvent = useCallback(
    (event: AnyAgentEvent) => {
      const currentId = activeTaskIdRef.current;
      if (!currentId || !event.run_id || event.run_id !== currentId) {
        return;
      }

      if (isEventType(event, "workflow.node.output.delta")) {
        if (!firstTokenReportedRef.current.has(currentId)) {
          const submittedAt = submittedAtByTaskRef.current.get(currentId);
          if (typeof submittedAt === "number") {
            firstTokenReportedRef.current.add(currentId);
            const ttftMs = Math.max(0, performance.now() - submittedAt);
            captureEvent(AnalyticsEvent.FirstTokenRendered, {
              session_id: session.resolvedSessionId ?? null,
              task_id: currentId,
              latency_ms: ttftMs,
              mock_stream: useMockStream,
            });
            performanceMonitor.trackTTFT(
              session.resolvedSessionId ?? currentId,
              ttftMs,
            );
          }
        }
      }

      if (
        isEventType(event, "workflow.result.final", "workflow.result.cancelled") ||
        isEventType(event, "workflow.node.failed")
      ) {
        submittedAtByTaskRef.current.delete(currentId);
        firstTokenReportedRef.current.delete(currentId);
        setActiveTaskId(null);
        setCancelRequested(false);
        cancelIntentRef.current = false;
      }
    },
    [session.resolvedSessionId, setCancelRequested, useMockStream, activeTaskIdRef, cancelIntentRef],
  );

  const {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents,
    reconnect,
    addEvent,
  } = useAgentEventStream(session.streamSessionId, {
    useMock: useMockStream,
    onEvent: handleAgentEvent,
  });

  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, [events]);

  const handleNewSession = useCallback(() => {
    session.handleNewSession({ clearEvents, resetTaskState });
  }, [clearEvents, resetTaskState, session]);

  const handleSessionSelect = useCallback(
    (id: string) => {
      session.handleSessionSelect(id, { clearEvents, resetTaskState });
    },
    [clearEvents, resetTaskState, session],
  );

  const { handleTaskSubmit, isCreatePending } = useTaskSubmission({
    useMockStream,
    resolvedSessionId: session.resolvedSessionId,
    sessionId: session.sessionId,
    currentSessionId: session.currentSessionId,
    prewarmSessionId: session.prewarmSessionId,
    prefillTask,
    setSessionId: session.setSessionId,
    setTaskId,
    setActiveTaskId,
    setPrewarmSessionId: session.setPrewarmSessionId,
    setCancelRequested,
    cancelIntentRef,
    addEvent,
    clearEvents,
    setCurrentSession: session.setCurrentSession,
    addToHistory: session.addToHistory,
    clearCurrentSession: session.clearCurrentSession,
    removeSession: session.removeSession,
    performCancellation,
    pendingSubmitTsRef,
    submittedAtByTaskRef,
    firstTokenReportedRef,
  });

  const {
    deleteTargetId,
    deleteInProgress,
    handleSessionDeleteRequest,
    handleDeleteCancel,
    handleConfirmDelete,
  } = useDeleteDialog({
    resolvedSessionId: session.resolvedSessionId,
    setSessionId: session.setSessionId,
    setTaskId,
    setActiveTaskId,
    setCancelRequested,
    cancelIntentRef,
    clearEvents,
    clearCurrentSession: session.clearCurrentSession,
    removeSession: session.removeSession,
  });

  const {
    shareDialogOpen,
    setShareDialogOpen,
    shareLink,
    shareInProgress,
    handleShareRequest,
    handleCopyShareLink,
  } = useShareDialog({ resolvedSessionId: session.resolvedSessionId });

  const hasAttachments = useMemo(
    () => collectAttachmentItems(events).length > 0,
    [events],
  );

  const hasRenderableEvents = useMemo(
    () => events.some((evt) => evt.event_type !== "connected"),
    [events],
  );

  const creationPending = useMockStream ? false : isCreatePending;
  const isTaskRunning = Boolean(activeTaskId);
  const stopPending = cancelRequested || isCancelPending;
  const inputDisabled = cancelRequested || isCancelPending;
  const streamIsRunning = isTaskRunning && !stopPending;

  const activeSessionLabel = session.resolvedSessionId
    ? session.sessionLabels[session.resolvedSessionId]?.trim()
    : null;
  const deleteTargetLabel = deleteTargetId
    ? session.sessionLabels[deleteTargetId]?.trim() ||
      t("console.history.itemPrefix", { id: deleteTargetId.slice(0, 8) })
    : null;
  const headerTitle = session.resolvedSessionId
    ? activeSessionLabel || t("conversation.header.activeLabel")
    : t("conversation.header.idle");

  const quickPrompts = useMemo<QuickPromptItem[]>(
    () => [
      {
        id: "image",
        label: t("console.quickstart.items.image"),
        icon: Image,
        prompt:
          "想要一张“雨夜的霓虹书店”插画，风格偏赛博朋克但保留手绘质感。画面里要有湿润街道反光、玻璃窗内的暖光、行人撑伞的剪影。比例 16:9，避免文字、水印和人脸特写。",
      },
      {
        id: "article",
        label: t("console.quickstart.items.article"),
        icon: FileText,
        prompt:
          "帮我写一篇给产品经理看的短文，主题是如何用“问题-假设-验证”闭环提升功能迭代质量。字数 800-1000，结构是开场痛点 → 三步法解释 → 一个真实场景案例 → 可执行清单。语气务实，少术语，多可落地方法。",
      },
      {
        id: "video",
        label: t("console.quickstart.items.video"),
        icon: Film,
        prompt:
          "想做一段 15 秒的品牌短片：城市清晨 → 通勤人群 → 产品特写 → logo 收尾。风格极简、干净、节奏轻快，比例 9:16（1080x1920），配乐偏轻电子、留白感。",
      },
    ],
    [t],
  );

  const showConnectingState =
    Boolean(session.resolvedSessionId) &&
    !hasRenderableEvents &&
    !isConnected &&
    !isReconnecting &&
    !error &&
    reconnectAttempts === 0;
  const showConnectionBanner =
    Boolean(session.resolvedSessionId) &&
    !hasRenderableEvents &&
    (Boolean(error) || isReconnecting || reconnectAttempts > 0);

  const emptyState = useMemo(
    () => (
      <EmptyStateView
        badge={t("console.empty.badge")}
        title={t("console.empty.title")}
        quickstartTitle={t("console.quickstart.title")}
        hotkeyHint={t("console.input.hotkeyHint")}
        items={quickPrompts}
        onSelect={setPrefillTask}
      />
    ),
    [quickPrompts, setPrefillTask, t],
  );

  const handleSidebarToggle = useCallback(() => {
    setIsSidebarOpen((prev) => {
      const next = !prev;
      captureEvent(AnalyticsEvent.SidebarToggled, {
        next_state: next ? "open" : "closed",
        previous_state: prev ? "open" : "closed",
      });
      return next;
    });
  }, []);

  const handleRightPanelToggle = useCallback(() => {
    setIsRightPanelOpen((prev) => {
      const next = !prev;
      captureEvent(AnalyticsEvent.SidebarToggled, {
        sidebar: "right_panel",
        next_state: next ? "open" : "closed",
        previous_state: prev ? "open" : "closed",
      });
      return next;
    });
  }, []);

  const handleCloseRightPanel = useCallback(() => {
    setIsRightPanelOpen(false);
  }, []);

  const mainStreamProps = useMemo(
    () => ({
      events,
      hasRenderableEvents,
      streamIsRunning,
      streamSessionId: session.streamSessionId,
    }),
    [events, hasRenderableEvents, streamIsRunning, session.streamSessionId],
  );

  const mainConnectionProps = useMemo(
    () => ({
      showConnectingState,
      showConnectionBanner,
      isConnected,
      isReconnecting,
      error,
      reconnectAttempts,
      onReconnect: reconnect,
    }),
    [
      showConnectingState,
      showConnectionBanner,
      isConnected,
      isReconnecting,
      error,
      reconnectAttempts,
      reconnect,
    ],
  );

  const mainSidebarProps = useMemo(
    () => ({
      isSidebarOpen,
      sessionHistory: session.sessionHistory,
      sessionLabels: session.sessionLabels,
      resolvedSessionId: session.resolvedSessionId,
      onSessionSelect: handleSessionSelect,
      onSessionDelete: handleSessionDeleteRequest,
      onNewSession: handleNewSession,
    }),
    [
      isSidebarOpen,
      session.sessionHistory,
      session.sessionLabels,
      session.resolvedSessionId,
      handleSessionSelect,
      handleSessionDeleteRequest,
      handleNewSession,
    ],
  );

  const mainRightPanelProps = useMemo(
    () => ({
      isRightPanelOpen,
      onCloseRightPanel: handleCloseRightPanel,
      hasAttachments,
    }),
    [isRightPanelOpen, handleCloseRightPanel, hasAttachments],
  );

  const mainComposerProps = useMemo(
    () => ({
      emptyState,
      loadingText: t("sessions.details.loading"),
      inputPlaceholder: session.resolvedSessionId
        ? t("console.input.placeholder.active")
        : t("console.input.placeholder.idle"),
      creationPending,
      inputDisabled,
      prefillTask,
      onPrefillApplied: () => setPrefillTask(null),
      onSubmit: handleTaskSubmit,
      onStop: handleStop,
      isTaskRunning,
      stopPending,
      stopDisabled: isCancelPending,
    }),
    [
      emptyState,
      t,
      session.resolvedSessionId,
      creationPending,
      inputDisabled,
      prefillTask,
      handleTaskSubmit,
      handleStop,
      isTaskRunning,
      stopPending,
      isCancelPending,
    ],
  );

  return (
    <div className="relative h-[100dvh] overflow-hidden bg-muted/10 text-foreground">
      <LLMIndicator />
      {shareDialogOpen ? (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) {
              setShareDialogOpen(false);
            }
          }}
        >
          <DialogContent className="max-w-md rounded-3xl">
            <DialogHeader className="space-y-2">
              <DialogTitle className="text-lg font-semibold">
                {t("share.dialog.title")}
              </DialogTitle>
              <DialogDescription className="text-sm text-muted-foreground">
                {t("share.dialog.description")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <Input readOnly value={shareLink ?? ""} />
              <Button
                type="button"
                variant="secondary"
                onClick={handleCopyShareLink}
                disabled={!shareLink}
                className="w-full"
              >
                <Copy className="h-4 w-4" />
                {t("share.dialog.copy")}
              </Button>
            </div>
            <DialogFooter className="sm:justify-end">
              <Button
                type="button"
                variant="outline"
                onClick={() => setShareDialogOpen(false)}
              >
                {t("share.dialog.close")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ) : null}
      {personaDialogOpen ? (
        <UserPersonaDialog
          open={personaDialogOpen}
          onOpenChange={setPersonaDialogOpen}
          sessionId={session.streamSessionId}
        />
      ) : null}
      <Dialog
        open={Boolean(deleteTargetId)}
        onOpenChange={(open) => {
          if (!open) {
            handleDeleteCancel();
          }
        }}
      >
        <DialogContent className="max-w-md rounded-3xl">
          <DialogHeader className="space-y-3">
            <DialogTitle className="text-lg font-semibold">
              {t("sidebar.session.confirmDelete.title")}
            </DialogTitle>
            <DialogDescription className="text-sm text-muted-foreground">
              {t("sidebar.session.confirmDelete.description")}
            </DialogDescription>
            {deleteTargetId && (
              <div className="flex items-center justify-between rounded-2xl border border-border/70 bg-muted/30 px-3 py-2">
                <div className="flex flex-col">
                  <span className="text-sm font-semibold text-foreground">
                    {deleteTargetLabel}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {session.formatSessionBadge(deleteTargetId)}
                  </span>
                </div>
              </div>
            )}
          </DialogHeader>
          <DialogFooter className="sm:justify-end">
            <Button
              variant="outline"
              onClick={handleDeleteCancel}
              disabled={deleteInProgress}
            >
              {t("sidebar.session.confirmDelete.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirmDelete}
              disabled={deleteInProgress}
            >
              {deleteInProgress ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : null}
              {t("sidebar.session.confirmDelete.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <div className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-6 overflow-hidden px-4 pb-10 pt-6 lg:px-8 2xl:px-12">
        <ConversationHeader
          title={headerTitle}
          isSidebarOpen={isSidebarOpen}
          onToggleSidebar={handleSidebarToggle}
          onOpenPersona={() => setPersonaDialogOpen(true)}
          streamSessionId={session.streamSessionId}
          onToggleRightPanel={handleRightPanelToggle}
          isRightPanelOpen={isRightPanelOpen}
          onShare={handleShareRequest}
          shareInProgress={shareInProgress}
          shareDisabled={!session.resolvedSessionId || shareInProgress}
        />

        <ConversationMainArea
          contentRef={contentRef}
          stream={mainStreamProps}
          connection={mainConnectionProps}
          sidebar={mainSidebarProps}
          rightPanel={mainRightPanelProps}
          composer={mainComposerProps}
        />
      </div>
    </div>
  );
}
