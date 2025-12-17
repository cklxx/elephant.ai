'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import dynamic from 'next/dynamic';
import { useSearchParams } from 'next/navigation';
import {
  Loader2,
  Film,
  FileText,
  Image,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
} from 'lucide-react';
import { useTaskExecution, useCancelTask } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore, useDeleteSession } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useI18n } from '@/lib/i18n';
import { Sidebar, Header, ContentArea } from '@/components/layout';
import { TaskInput } from '@/components/agent/TaskInput';
import { formatParsedError, getErrorLogPayload, isAPIError, parseError } from '@/lib/errors';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import type { AnyAgentEvent, AttachmentPayload, AttachmentUpload } from '@/lib/types';
import { eventMatches } from '@/lib/types';
import { captureEvent } from '@/lib/analytics/posthog';
import { AnalyticsEvent } from '@/lib/analytics/events';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  AttachmentPanel,
  collectAttachmentItems,
} from '@/components/agent/AttachmentPanel';
import { SkillsPanel } from '@/components/agent/SkillsPanel';

const LazyTerminalOutput = dynamic(
  () => import('@/components/agent/TerminalOutput').then((mod) => mod.TerminalOutput),
  {
    ssr: false,
    loading: () => (
      <div className="rounded-2xl border border-dashed border-border/60 bg-card/60 p-4 text-sm text-muted-foreground">
        Preparing event stream…
      </div>
    ),
  },
);
export function ConversationPageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [cancelRequested, setCancelRequested] = useState(false);
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
  const [showTimelineDialog, setShowTimelineDialog] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [isRightPanelOpen, setIsRightPanelOpen] = useState(false);
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);
  const [deleteInProgress, setDeleteInProgress] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const cancelIntentRef = useRef(false);
  const activeTaskIdRef = useRef<string | null>(null);
  const searchParams = useSearchParams();
  const { t } = useI18n();

  const useMockStream = useMemo(() => searchParams.get('mockSSE') === '1', [searchParams]);

  const buildAttachmentMap = useCallback(
    (uploads: AttachmentUpload[]) =>
      uploads.reduce<Record<string, AttachmentPayload>>((acc, att) => {
        const { name, ...rest } = att;

        acc[name] = {
          name,
          ...rest,
        } as AttachmentPayload;
        return acc;
      }, {}),
    []
  );

  useEffect(() => {
    activeTaskIdRef.current = activeTaskId;
  }, [activeTaskId]);

  const { mutate: executeTask, isPending: isCreatePending } = useTaskExecution();
  const { mutate: cancelTask, isPending: isCancelPending } = useCancelTask();
  const deleteSessionMutation = useDeleteSession();
  const {
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
    removeSession,
    sessionHistory = [],
    sessionLabels = {},
  } = useSessionStore();

  const resolvedSessionId = sessionId || currentSessionId;
  const formatSessionBadge = useCallback(
    (value: string) =>
      value.length > 8 ? `${value.slice(0, 4)}…${value.slice(-4)}` : value,
    []
  );

  const handleAgentEvent = useCallback(
    (event: AnyAgentEvent) => {
      const currentId = activeTaskIdRef.current;
      if (!currentId || !event.task_id || event.task_id !== currentId) {
        return;
      }

      if (
        eventMatches(event, 'workflow.result.final', 'workflow.result.final') ||
        eventMatches(event, 'workflow.result.cancelled', 'workflow.result.cancelled') ||
        eventMatches(event, 'workflow.node.failed')
      ) {
        setActiveTaskId(null);
        setCancelRequested(false);
        cancelIntentRef.current = false;
      }
    },
    [setActiveTaskId, setCancelRequested]
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
  } = useAgentEventStream(resolvedSessionId, {
    useMock: useMockStream,
    onEvent: handleAgentEvent,
  });
  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, [events]);

  const hasAttachments = useMemo(
    () => collectAttachmentItems(events).length > 0,
    [events]
  );

  const performCancellation = useCallback(
    (taskId: string) => {
      cancelIntentRef.current = false;

      if (useMockStream) {
        setActiveTaskId(null);
        setCancelRequested(false);
        toast.success(
          t('console.toast.taskCancelRequested.title'),
          t('console.toast.taskCancelRequested.description')
        );
        captureEvent(AnalyticsEvent.TaskCancelRequested, {
          session_id: resolvedSessionId ?? null,
          task_id: taskId,
          status: 'success',
          mock_stream: true,
        });
        return;
      }

      cancelTask(taskId, {
        onSuccess: () => {
          const currentActiveTaskId = activeTaskIdRef.current;

          if (!currentActiveTaskId || currentActiveTaskId === taskId) {
            setActiveTaskId((prevActiveTaskId) =>
              prevActiveTaskId === taskId ? null : prevActiveTaskId
            );
            setCancelRequested(false);
          }
          toast.success(
            t('console.toast.taskCancelRequested.title'),
            t('console.toast.taskCancelRequested.description')
          );
          captureEvent(AnalyticsEvent.TaskCancelRequested, {
            session_id: resolvedSessionId ?? null,
            task_id: taskId,
            status: 'success',
            mock_stream: false,
          });
        },
        onError: (cancelError) => {
          console.error(
            '[ConversationPage] Task cancellation error:',
            getErrorLogPayload(cancelError)
          );
          setCancelRequested(false);
          const parsed = parseError(cancelError, t('common.error.unknown'));
          toast.error(
            t('console.toast.taskCancelError.title'),
            t('console.toast.taskCancelError.description', {
              message: formatParsedError(parsed),
            })
          );
          captureEvent(AnalyticsEvent.TaskCancelFailed, {
            session_id: resolvedSessionId ?? null,
            task_id: taskId,
            error_kind: isAPIError(cancelError) ? 'api' : 'unknown',
            ...(isAPIError(cancelError) ? { status_code: cancelError.status } : {}),
          });
        },
      });
    },
    [
      cancelTask,
      resolvedSessionId,
      setActiveTaskId,
      setCancelRequested,
      t,
      useMockStream,
    ]
  );

  const handleTaskSubmit = (task: string, attachments: AttachmentUpload[]) => {
    console.log('[ConversationPage] Task submitted:', { task, attachments });

    captureEvent(AnalyticsEvent.TaskSubmitted, {
      session_id: resolvedSessionId ?? null,
      has_active_session: Boolean(resolvedSessionId),
      attachment_count: attachments.length,
      has_attachments: attachments.length > 0,
      input_length: task.length,
      mock_stream: useMockStream,
      prefill_present: Boolean(prefillTask),
    });

    cancelIntentRef.current = false;
    setCancelRequested(false);

    if (useMockStream) {
      const submissionTimestamp = new Date();
      const provisionalSessionId =
        sessionId || currentSessionId || `mock-${submissionTimestamp.getTime().toString(36)}`;
      const mockTaskId = `mock-task-${submissionTimestamp.getTime().toString(36)}`;

      const attachmentMap = buildAttachmentMap(attachments);

      addEvent({
        event_type: 'workflow.input.received',
        timestamp: submissionTimestamp.toISOString(),
        agent_level: 'core',
        session_id: provisionalSessionId,
        task_id: mockTaskId,
        task,
        attachments: Object.keys(attachmentMap).length ? attachmentMap : undefined,
      });

      setSessionId(provisionalSessionId);
      setTaskId(mockTaskId);
      setActiveTaskId(mockTaskId);
      setCurrentSession(provisionalSessionId);
      addToHistory(provisionalSessionId);
      return;
    }

    const initialSessionId = resolvedSessionId;
    let retriedWithoutSession = false;

    const runExecution = (requestedSessionId: string | null) => {
      executeTask(
        {
          task,
          session_id: requestedSessionId ?? undefined,
          attachments: attachments.length ? attachments : undefined,
        },
        {
          onSuccess: (data) => {
            console.log('[ConversationPage] Task execution started:', data);
            setSessionId(data.session_id);
            setTaskId(data.task_id);
            setActiveTaskId(data.task_id);
            setCurrentSession(data.session_id);
            addToHistory(data.session_id);

            const attachmentMap = buildAttachmentMap(attachments);
            addEvent({
              event_type: 'workflow.input.received',
              timestamp: new Date().toISOString(),
              agent_level: 'core',
              session_id: data.session_id,
              task_id: data.task_id,
              parent_task_id: data.parent_task_id ?? undefined,
              task,
              attachments: Object.keys(attachmentMap).length ? attachmentMap : undefined,
            });
            if (cancelIntentRef.current) {
              setCancelRequested(true);
              performCancellation(data.task_id);
            }
          },
          onError: (error) => {
            const isStaleSession =
              !retriedWithoutSession &&
              !!requestedSessionId &&
              isAPIError(error) &&
              error.status === 404;

            if (isStaleSession) {
              retriedWithoutSession = true;
            console.warn('[ConversationPage] Session not found, retrying without session_id', {
              sessionId: requestedSessionId,
              error: getErrorLogPayload(error),
            });

            setSessionId(null);
            setTaskId(null);
            setActiveTaskId(null);
            setCancelRequested(false);
            cancelIntentRef.current = false;
            clearCurrentSession();
            removeSession(requestedSessionId);
            clearEvents();

            captureEvent(AnalyticsEvent.TaskRetriedWithoutSession, {
              session_id: requestedSessionId,
              error_status: 404,
              mock_stream: useMockStream,
            });

            runExecution(null);
            return;
          }

          console.error(
            '[ConversationPage] Task execution error:',
            getErrorLogPayload(error)
          );
          cancelIntentRef.current = false;
          setCancelRequested(false);
          setActiveTaskId(null);
          const parsed = parseError(error, t('common.error.unknown'));
          toast.error(
            t('console.toast.taskFailed'),
            formatParsedError(parsed)
          );
          captureEvent(AnalyticsEvent.TaskSubmissionFailed, {
            session_id: requestedSessionId ?? null,
            is_api_error: isAPIError(error),
            mock_stream: useMockStream,
            ...(isAPIError(error) ? { status_code: error.status } : {}),
          });
        },
      }
    );
  };

    runExecution(initialSessionId ?? null);
  };

  const handleStop = useCallback(() => {
    if (isCancelPending) {
      return;
    }

    captureEvent(AnalyticsEvent.TaskCancelRequested, {
      session_id: resolvedSessionId ?? null,
      task_id: activeTaskId ?? null,
      status: 'initiated',
      mock_stream: useMockStream,
      request_state: activeTaskId ? 'inflight' : 'queued',
    });

    setCancelRequested(true);
    if (activeTaskId) {
      performCancellation(activeTaskId);
    } else {
      cancelIntentRef.current = true;
    }
  }, [activeTaskId, isCancelPending, performCancellation, resolvedSessionId, useMockStream]);

  const handleNewSession = () => {
    setSessionId(null);
    setTaskId(null);
    setActiveTaskId(null);
    setCancelRequested(false);
    cancelIntentRef.current = false;
    clearEvents();
    clearCurrentSession();
    captureEvent(AnalyticsEvent.SessionCreated, {
      previous_session_id: resolvedSessionId ?? null,
      had_active_session: Boolean(resolvedSessionId),
      history_count: sessionHistory.length,
    });
  };

  const handleSessionSelect = (id: string) => {
    if (!id) return;
    clearEvents();
    setSessionId(id);
    setTaskId(null);
    setActiveTaskId(null);
    setCancelRequested(false);
    cancelIntentRef.current = false;
    setCurrentSession(id);
    addToHistory(id);
    captureEvent(AnalyticsEvent.SessionSelected, {
      session_id: id,
      previous_session_id: resolvedSessionId ?? null,
      was_in_history: sessionHistory.includes(id),
    });
  };

  const handleSessionDeleteRequest = (id: string) => {
    setDeleteTargetId(id);
  };

  const handleDeleteCancel = () => {
    if (deleteInProgress) return;
    setDeleteTargetId(null);
  };

  const handleConfirmDelete = async () => {
    if (!deleteTargetId) return;
    setDeleteInProgress(true);
    try {
      await deleteSessionMutation.mutateAsync(deleteTargetId);
      removeSession(deleteTargetId);
      if (resolvedSessionId === deleteTargetId) {
        clearEvents();
        setSessionId(null);
        setTaskId(null);
        setActiveTaskId(null);
        setCancelRequested(false);
        cancelIntentRef.current = false;
        clearCurrentSession();
      }
      toast.success(t('sidebar.session.toast.deleteSuccess'));
      captureEvent(AnalyticsEvent.SessionDeleted, {
        session_id: deleteTargetId,
        status: 'success',
      });
      setDeleteTargetId(null);
    } catch (err) {
      console.error('[ConversationPage] Failed to delete session:', getErrorLogPayload(err));
      const parsed = parseError(err, t('common.error.unknown'));
      toast.error(t('sidebar.session.toast.deleteError'), formatParsedError(parsed));
      captureEvent(AnalyticsEvent.SessionDeleted, {
        session_id: deleteTargetId,
        status: 'error',
        error_kind: isAPIError(err) ? 'api' : 'unknown',
        ...(isAPIError(err) ? { status_code: err.status } : {}),
      });
    } finally {
      setDeleteInProgress(false);
    }
  };

  const creationPending = useMockStream ? false : isCreatePending;
  const isTaskRunning = Boolean(activeTaskId);
  const stopPending = cancelRequested || isCancelPending;
  const inputDisabled = cancelRequested || isCancelPending;

  const activeSessionLabel = resolvedSessionId
    ? sessionLabels[resolvedSessionId]?.trim()
    : null;
  const deleteTargetLabel = deleteTargetId
    ? sessionLabels[deleteTargetId]?.trim() ||
      t('console.history.itemPrefix', { id: deleteTargetId.slice(0, 8) })
    : null;
  const headerTitle = resolvedSessionId
    ? activeSessionLabel || t('conversation.header.activeLabel')
    : t('conversation.header.idle');

  const quickPrompts = useMemo(
    () => [
      {
        id: 'image',
        label: t('console.quickstart.items.image'),
        icon: Image,
        prompt:
          '/plan 画图：请根据以下要求生成一张图片。\n主题：\n风格：\n尺寸/比例：\n需要包含/避免：\n',
      },
      {
        id: 'article',
        label: t('console.quickstart.items.article'),
        icon: FileText,
        prompt:
          '/plan 写文章：请根据以下要求写一篇文章。\n主题：\n受众：\n字数：\n结构/要点：\n',
      },
      {
        id: 'video',
        label: t('console.quickstart.items.video'),
        icon: Film,
        prompt:
          '/plan 生成视频：请根据以下要求生成一段视频。\n内容/脚本：\n风格：\n时长：\n比例/分辨率：\n',
      },
    ],
    [t],
  );

  const emptyState = (
    <div
      className="w-full max-w-md"
      data-testid="conversation-empty-state"
    >
      <div className="rounded-3xl border border-border/60 bg-background/70 p-6 text-center">
        <div className="mx-auto inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
          <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400/70" />
          {t('console.empty.badge')}
        </div>

        <div className="mt-4 space-y-2">
          <p
            className="text-lg font-semibold tracking-tight text-foreground"
            data-testid="conversation-empty-title"
          >
            {t('console.empty.title')}
          </p>
          <p className="text-sm text-muted-foreground" data-testid="conversation-empty-description">
            {t('console.empty.description')}
          </p>
          <p className="text-xs text-muted-foreground/80" data-testid="conversation-empty-prompt">
            {t('console.empty.prompt')}
          </p>
        </div>

        <div className="mt-6">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
            {t('console.quickstart.title')}
          </p>
          <div className="mt-3 flex flex-wrap justify-center gap-2">
            {quickPrompts.map((item) => (
              <Button
                key={item.id}
                type="button"
                variant="outline"
                size="sm"
                className="h-9 rounded-full border-border/40 bg-secondary/40 text-xs font-semibold shadow-none hover:bg-secondary/60"
                onClick={() => setPrefillTask(item.prompt)}
              >
                <item.icon className="h-4 w-4" aria-hidden />
                {item.label}
              </Button>
            ))}
          </div>

          <p className="mt-4 text-xs text-muted-foreground">
            {t('console.input.hotkeyHint')}
          </p>
        </div>
      </div>
    </div>
  );

  const timelineSteps = useTimelineSteps(events);

  useEffect(() => {
    if (timelineSteps.length === 0 && showTimelineDialog) {
      setShowTimelineDialog(false);
    }
  }, [timelineSteps, showTimelineDialog]);

  return (
    <div className="relative h-[100dvh] overflow-hidden bg-muted/10 text-foreground">
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
              {t('sidebar.session.confirmDelete.title')}
            </DialogTitle>
            <DialogDescription className="text-sm text-muted-foreground">
              {t('sidebar.session.confirmDelete.description')}
            </DialogDescription>
            {deleteTargetId && (
              <div className="flex items-center justify-between rounded-2xl border border-border/70 bg-muted/30 px-3 py-2">
                <div className="flex flex-col">
                  <span className="text-sm font-semibold text-foreground">
                    {deleteTargetLabel}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {formatSessionBadge(deleteTargetId)}
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
              {t('sidebar.session.confirmDelete.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirmDelete}
              disabled={deleteInProgress}
            >
              {deleteInProgress && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('sidebar.session.confirmDelete.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <div className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-6 overflow-hidden px-4 pb-10 pt-6 lg:px-8 2xl:px-12">
        <Header
          title={headerTitle}
          showEnvironmentStrip={false}
          leadingSlot={
            <Button
              type="button"
              variant="ghost"
              size="icon"
              data-testid="session-list-toggle"
              onClick={() =>
                setIsSidebarOpen((prev) => {
                  const next = !prev;
                  captureEvent(AnalyticsEvent.SidebarToggled, {
                    next_state: next ? 'open' : 'closed',
                    previous_state: prev ? 'open' : 'closed',
                  });
                  return next;
                })
              }
              className="h-10 w-10 rounded-full border border-border/60"
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
                  ? t('sidebar.toggle.close')
                  : t('sidebar.toggle.open')}
              </span>
            </Button>
          }
          actionsSlot={
            <Button
              type="button"
              variant="ghost"
              size="icon"
              data-testid="right-panel-toggle"
              onClick={() =>
                setIsRightPanelOpen((prev) => {
                  const next = !prev;
                  captureEvent(AnalyticsEvent.SidebarToggled, {
                    sidebar: 'right_panel',
                    next_state: next ? 'open' : 'closed',
                    previous_state: prev ? 'open' : 'closed',
                  });
                  return next;
                })
              }
              className="h-10 w-10 rounded-full border border-border/60"
              aria-expanded={isRightPanelOpen}
              aria-controls="conversation-right-panel"
            >
              {isRightPanelOpen ? (
                <PanelRightClose className="h-4 w-4" />
              ) : (
                <PanelRightOpen className="h-4 w-4" />
              )}
              <span className="sr-only">
                {isRightPanelOpen ? 'Close right panel' : 'Open right panel'}
              </span>
            </Button>
          }
        />

        <div className="flex flex-1 min-h-0 flex-col gap-5 overflow-hidden lg:flex-row">
          <div
            id="conversation-sidebar"
            className={cn(
              "overflow-hidden transition-all duration-300 lg:w-72 lg:flex-none",
              isSidebarOpen ? "block" : "hidden"
            )}
            aria-hidden={!isSidebarOpen}
          >
            <Sidebar
              sessionHistory={sessionHistory}
              sessionLabels={sessionLabels}
              currentSessionId={resolvedSessionId}
              onSessionSelect={handleSessionSelect}
              onSessionDelete={handleSessionDeleteRequest}
              onNewSession={handleNewSession}
            />
          </div>

          <div className="flex flex-1 min-h-0 min-w-0 flex-col overflow-hidden rounded-3xl border border-border/60 bg-card">
            <ContentArea
              ref={contentRef}
              className="flex-1 min-h-0 min-w-0"
              fullWidth
              contentClassName="space-y-4"
            >
              {timelineSteps.length > 0 && (
                <div className="sm:hidden">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    data-testid="mobile-timeline-trigger"
                    onClick={() => {
                      captureEvent(AnalyticsEvent.TimelineViewed, {
                        session_id: resolvedSessionId ?? null,
                        step_count: timelineSteps.length,
                      });
                      setShowTimelineDialog(true);
                    }}
                    className="mb-3 rounded-full border-border/70 bg-background/60 text-[11px] font-semibold"
                  >
                    {t('console.timeline.mobileLabel')}
                  </Button>
                </div>
              )}
              {events.length === 0 ? (
                <div className="flex min-h-[60vh] items-center justify-center">
                  {emptyState}
                </div>
              ) : (
                <LazyTerminalOutput
                  events={events}
                  isConnected={isConnected}
                  isReconnecting={isReconnecting}
                  error={error}
                  reconnectAttempts={reconnectAttempts}
                  onReconnect={reconnect}
                  isRunning={isTaskRunning}
                />
              )}
            </ContentArea>

            {showTimelineDialog && (
              <div
                role="dialog"
                aria-modal="true"
                className="fixed inset-0 z-50 flex flex-col justify-end bg-black/30 sm:hidden"
              >
                <button
                  type="button"
                  className="absolute inset-0 h-full w-full"
                  aria-label={t('plan.collapse')}
                  onClick={() => setShowTimelineDialog(false)}
                />
                <div className="relative rounded-t-3xl border border-border/60 bg-card/80 p-4 text-foreground">
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="text-sm font-semibold text-foreground">
                      {t('console.timeline.dialogTitle')}
                    </h2>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      className="rounded-full"
                      onClick={() => setShowTimelineDialog(false)}
                    >
                      {t('plan.collapse')}
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {timelineSteps.map((step) => (
                      <button
                        key={step.id}
                        type="button"
                        role="button"
                        onClick={() => setShowTimelineDialog(false)}
                        className="w-full rounded-xl border border-border/60 bg-background/80 px-3 py-2 text-left text-foreground transition hover:bg-background"
                      >
                        <p className="text-sm font-semibold text-foreground">{step.title}</p>
                        {step.description && (
                          <p className="text-xs text-foreground/70">{step.description}</p>
                        )}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            )}

            <div className="border-t border-border/60 bg-background/70 px-3 py-4 sm:px-6 sm:py-6">
              <TaskInput
                onSubmit={handleTaskSubmit}
                placeholder={
                  resolvedSessionId
                    ? t('console.input.placeholder.active')
                    : t('console.input.placeholder.idle')
                }
                disabled={inputDisabled}
                loading={creationPending}
                prefill={prefillTask}
                onPrefillApplied={() => setPrefillTask(null)}
                onStop={handleStop}
                isRunning={isTaskRunning}
                stopPending={stopPending}
                stopDisabled={isCancelPending}
              />
            </div>
          </div>
          <div
            id="conversation-right-panel"
            className={cn(
              "hidden lg:flex flex-none justify-end overflow-hidden transition-all duration-300",
              isRightPanelOpen ? "w-[380px] xl:w-[440px]" : "w-0"
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
        </div>
      </div>

      {isRightPanelOpen && (
        <div className="fixed inset-0 z-50 flex lg:hidden">
          <button
            type="button"
            className="absolute inset-0 h-full w-full bg-black/30"
            aria-label="Close right panel"
            onClick={() => setIsRightPanelOpen(false)}
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
                onClick={() => setIsRightPanelOpen(false)}
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
}
