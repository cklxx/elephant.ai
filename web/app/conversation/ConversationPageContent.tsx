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
      if (!currentId || !event.task_id || event.task_id !== currentId) {
        return;
      }

      if (isEventType(event, "workflow.node.output.delta")) {
        if (!firstTokenReportedRef.current.has(currentId)) {
          const submittedAt = submittedAtByTaskRef.current.get(currentId);
          if (typeof submittedAt === "number") {
            firstTokenReportedRef.current.add(currentId);
            captureEvent(AnalyticsEvent.FirstTokenRendered, {
              session_id: session.resolvedSessionId ?? null,
              task_id: currentId,
              latency_ms: Math.max(0, performance.now() - submittedAt),
              mock_stream: useMockStream,
            });
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
          "画图：生成一张“雨夜的霓虹书店”插画。\n风格：赛博朋克 + 手绘质感\n画面要素：湿润街道反光、玻璃窗内暖光、行人撑伞剪影\n尺寸/比例：16:9\n避免：文字、水印、人脸特写\n",
      },
      {
        id: "article",
        label: t("console.quickstart.items.article"),
        icon: FileText,
        prompt:
          "写文章：写一篇面向产品经理的短文。\n主题：如何用“问题-假设-验证”闭环提升功能迭代质量\n字数：800-1000字\n结构：开场痛点 → 三步法解释 → 一个真实场景案例 → 可执行清单\n要求：语气务实、少术语、多可落地方法\n",
      },
      {
        id: "video",
        label: t("console.quickstart.items.video"),
        icon: Film,
        prompt:
          "生成视频：制作一段15秒品牌短片。\n内容/脚本：城市清晨→通勤人群→产品特写→logo收尾\n风格：极简、干净、轻快节奏\n时长：15秒\n比例/分辨率：9:16，1080x1920\n配乐：轻电子，留白感\n",
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

  const emptyState = (
    <EmptyStateView
      badge={t("console.empty.badge")}
      title={t("console.empty.title")}
      quickstartTitle={t("console.quickstart.title")}
      hotkeyHint={t("console.input.hotkeyHint")}
      items={quickPrompts}
      onSelect={setPrefillTask}
    />
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
          events={events}
          hasRenderableEvents={hasRenderableEvents}
          showConnectingState={showConnectingState}
          showConnectionBanner={showConnectionBanner}
          isConnected={isConnected}
          isReconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={reconnect}
          streamIsRunning={streamIsRunning}
          isSidebarOpen={isSidebarOpen}
          sessionHistory={session.sessionHistory}
          sessionLabels={session.sessionLabels}
          resolvedSessionId={session.resolvedSessionId}
          onSessionSelect={handleSessionSelect}
          onSessionDelete={handleSessionDeleteRequest}
          onNewSession={handleNewSession}
          isRightPanelOpen={isRightPanelOpen}
          onCloseRightPanel={() => setIsRightPanelOpen(false)}
          hasAttachments={hasAttachments}
          streamSessionId={session.streamSessionId}
          emptyState={emptyState}
          loadingText={t("sessions.details.loading")}
          inputPlaceholder={
            session.resolvedSessionId
              ? t("console.input.placeholder.active")
              : t("console.input.placeholder.idle")
          }
          creationPending={creationPending}
          inputDisabled={inputDisabled}
          prefillTask={prefillTask}
          onPrefillApplied={() => setPrefillTask(null)}
          onSubmit={handleTaskSubmit}
          onStop={handleStop}
          isTaskRunning={isTaskRunning}
          stopPending={stopPending}
          stopDisabled={isCancelPending}
        />
      </div>
    </div>
  );
}
