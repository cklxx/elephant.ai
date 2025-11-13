'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { History, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { useTaskExecution, useCancelTask } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore, useDeleteSession } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { useConfirmDialog } from '@/components/ui/dialog';
import { useI18n } from '@/lib/i18n';
import { Sidebar, Header, ContentArea } from '@/components/layout';
import { TaskInput } from '@/components/agent/TaskInput';
import { formatParsedError, getErrorLogPayload, isAPIError, parseError } from '@/lib/errors';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import type { AnyAgentEvent, AttachmentPayload, AttachmentUpload } from '@/lib/types';
import { captureEvent } from '@/lib/analytics/posthog';
import { AnalyticsEvent } from '@/lib/analytics/events';

export function ConversationPageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [cancelRequested, setCancelRequested] = useState(false);
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
  const [showTimelineDialog, setShowTimelineDialog] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const cancelIntentRef = useRef(false);
  const activeTaskIdRef = useRef<string | null>(null);
  const searchParams = useSearchParams();
  const { t } = useI18n();

  const useMockStream = useMemo(() => searchParams.get('mockSSE') === '1', [searchParams]);

  useEffect(() => {
    activeTaskIdRef.current = activeTaskId;
  }, [activeTaskId]);

  const { mutate: executeTask, isPending: isCreatePending } = useTaskExecution();
  const { mutate: cancelTask, isPending: isCancelPending } = useCancelTask();
  const deleteSessionMutation = useDeleteSession();
  const { confirm, ConfirmDialog } = useConfirmDialog();
  const {
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
    removeSession,
    sessionHistory = [],
    pinnedSessions = [],
    sessionLabels = {},
    renameSession,
    togglePinSession,
  } = useSessionStore();

  const resolvedSessionId = sessionId || currentSessionId;

  const handleAgentEvent = useCallback(
    (event: AnyAgentEvent) => {
      const currentId = activeTaskIdRef.current;
      if (!currentId || !event.task_id || event.task_id !== currentId) {
        return;
      }

      if (
        event.event_type === 'task_complete' ||
        event.event_type === 'task_cancelled' ||
        event.event_type === 'error'
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

  // Auto-set session name from first task_analysis event
  const hasSetSessionNameRef = useRef(false);
  useEffect(() => {
    if (!resolvedSessionId || hasSetSessionNameRef.current) return;

    const taskAnalysisEvent = events.find((e) => e.event_type === 'task_analysis');
    if (taskAnalysisEvent && 'action_name' in taskAnalysisEvent) {
      renameSession(resolvedSessionId, taskAnalysisEvent.action_name);
      hasSetSessionNameRef.current = true;
    }
  }, [events, resolvedSessionId, renameSession]);

  // Reset flag when session changes
  useEffect(() => {
    hasSetSessionNameRef.current = false;
  }, [resolvedSessionId]);

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, [events]);

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
      const attachmentMap = attachments.reduce<Record<string, AttachmentPayload>>((acc, att) => {
        acc[att.name] = {
          name: att.name,
          media_type: att.media_type,
          data: att.data,
          uri: att.uri,
          source: att.source,
          description: att.description,
        };
        return acc;
      }, {});

      addEvent({
        event_type: 'user_task',
        timestamp: new Date().toISOString(),
        agent_level: 'core',
        session_id: resolvedSessionId ?? 'pending-session',
        task_id: taskId ?? undefined,
        task,
        attachments: Object.keys(attachmentMap).length ? attachmentMap : undefined,
      });

      const mockSessionId = sessionId || currentSessionId || `mock-${Date.now().toString(36)}`;
      const mockTaskId = `mock-task-${Date.now().toString(36)}`;
      setSessionId(mockSessionId);
      setTaskId(mockTaskId);
      setActiveTaskId(mockTaskId);
      setCurrentSession(mockSessionId);
      addToHistory(mockSessionId);
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
      pinned_count: pinnedSessions.length,
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
      was_pinned: pinnedSessions.includes(id),
      was_in_history: sessionHistory.includes(id),
    });
  };

  const handleSessionDelete = async (id: string) => {
    const confirmed = await confirm({
      title: t('sidebar.session.confirmDelete.title'),
      description: t('sidebar.session.confirmDelete.description'),
      confirmText: t('sidebar.session.confirmDelete.confirm'),
      cancelText: t('sidebar.session.confirmDelete.cancel'),
      variant: 'danger',
    });

    if (confirmed) {
      try {
        await deleteSessionMutation.mutateAsync(id);
        removeSession(id);
        if (resolvedSessionId === id) {
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
          session_id: id,
          status: 'success',
        });
      } catch (err) {
        console.error(
          '[ConversationPage] Failed to delete session:',
          getErrorLogPayload(err)
        );
        const parsed = parseError(err, t('common.error.unknown'));
        toast.error(
          t('sidebar.session.toast.deleteError'),
          formatParsedError(parsed)
        );
        captureEvent(AnalyticsEvent.SessionDeleted, {
          session_id: id,
          status: 'error',
          error_kind: isAPIError(err) ? 'api' : 'unknown',
          ...(isAPIError(err) ? { status_code: err.status } : {}),
        });
      }
    }
  };

  const creationPending = useMockStream ? false : isCreatePending;
  const isTaskRunning = Boolean(activeTaskId);
  const stopPending = cancelRequested || isCancelPending;
  const inputDisabled = cancelRequested || isCancelPending;

  const hasConversation = Boolean(resolvedSessionId) || events.length > 0;

  const firstTaskAnalysis = useMemo(
    () => events.find((event) => event.event_type === "task_analysis"),
    [events],
  );
  const analysisTitle =
    firstTaskAnalysis && "action_name" in firstTaskAnalysis
      ? firstTaskAnalysis.action_name
      : null;
  const analysisSubtitle =
    firstTaskAnalysis && "goal" in firstTaskAnalysis
      ? firstTaskAnalysis.goal
      : null;

  const activeSessionLabel = resolvedSessionId
    ? sessionLabels[resolvedSessionId]?.trim()
    : null;
  const sessionBadge = resolvedSessionId
    ? activeSessionLabel || (resolvedSessionId.length > 8 ? `${resolvedSessionId.slice(0, 4)}…${resolvedSessionId.slice(-4)}` : resolvedSessionId)
    : null;

  const emptyState = (
    <div
      className="flex flex-col items-center justify-center gap-3 text-center"
      data-testid="conversation-empty-state"
    >
      <p
        className="text-base font-semibold text-slate-700"
        data-testid="conversation-empty-title"
      >
        {t('console.empty.title')}
      </p>
      <p
        className="console-microcopy max-w-sm text-slate-400"
        data-testid="conversation-empty-prompt"
      >
        {t('console.empty.prompt')}
      </p>
    </div>
  );

  const sessionArchiveLink = (
    <Link
      href="/sessions"
      className="console-button console-button-secondary inline-flex items-center gap-2 text-xs uppercase tracking-[0.3em]"
    >
      <History className="h-4 w-4" aria-hidden />
      <span>{t('navigation.sessions')}</span>
    </Link>
  );

  const timelineSteps = useTimelineSteps(events);

  useEffect(() => {
    if (timelineSteps.length === 0 && showTimelineDialog) {
      setShowTimelineDialog(false);
    }
  }, [timelineSteps, showTimelineDialog]);

  return (
    <div className="flex h-screen bg-white">
      <ConfirmDialog />
      {/* Left Sidebar */}
      <div
        id="conversation-sidebar"
        className={`relative z-30 h-full overflow-hidden border-r border-slate-200 transition-[width] duration-300 ease-in-out ${
          isSidebarOpen ? 'w-64' : 'w-0 border-r-0'
        }`}
      >
        <div
          className={`h-full ${isSidebarOpen ? '' : 'pointer-events-none opacity-0'}`}
          aria-hidden={!isSidebarOpen}
        >
          <Sidebar
            sessionHistory={sessionHistory}
            pinnedSessions={pinnedSessions}
            sessionLabels={sessionLabels}
            currentSessionId={resolvedSessionId}
            onSessionSelect={handleSessionSelect}
            onSessionRename={renameSession}
            onSessionPin={togglePinSession}
            onSessionDelete={handleSessionDelete}
            onNewSession={handleNewSession}
          />
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header
          title={analysisTitle || sessionBadge || t('conversation.header.idle')}
          subtitle={
            analysisTitle
              ? analysisSubtitle || sessionBadge || undefined
              : resolvedSessionId
                ? t('conversation.header.subtitle')
                : undefined
          }
          actionsSlot={sessionArchiveLink}
          leadingSlot={
            <button
              type="button"
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
              className="console-button console-button-secondary flex items-center justify-center !px-3 !py-2"
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
            </button>
          }
        />

        {/* Content Area */}
        <ContentArea
          ref={contentRef}
          isEmpty={events.length === 0}
          emptyState={emptyState}
        >
          {timelineSteps.length > 0 && (
            <div className="sm:hidden">
              <button
                type="button"
                data-testid="mobile-timeline-trigger"
                onClick={() => {
                  captureEvent(AnalyticsEvent.TimelineViewed, {
                    session_id: resolvedSessionId ?? null,
                    step_count: timelineSteps.length,
                  });
                  setShowTimelineDialog(true);
                }}
                className="mb-3 inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-[11px] font-medium text-slate-600 shadow-sm"
              >
                {t('console.timeline.mobileLabel')}
              </button>
            </div>
          )}
          <TerminalOutput
            events={events}
            isConnected={isConnected}
            isReconnecting={isReconnecting}
            error={error}
            reconnectAttempts={reconnectAttempts}
            onReconnect={reconnect}
          />
        </ContentArea>

        {showTimelineDialog && (
          <div
            role="dialog"
            aria-modal="true"
            className="fixed inset-0 z-50 flex flex-col justify-end bg-slate-900/30 sm:hidden"
          >
            <button
              type="button"
              className="absolute inset-0 h-full w-full"
              aria-label={t('plan.collapse')}
              onClick={() => setShowTimelineDialog(false)}
            />
            <div className="relative rounded-t-2xl bg-white p-4 shadow-xl">
              <div className="mb-3 flex items-center justify-between">
                <h2 className="text-sm font-semibold text-slate-900">
                  {t('console.timeline.dialogTitle')}
                </h2>
                <button
                  type="button"
                  className="rounded-full border border-slate-200 px-3 py-1 text-xs font-semibold uppercase tracking-[0.3em] text-slate-500"
                  onClick={() => setShowTimelineDialog(false)}
                >
                  {t('plan.collapse')}
                </button>
              </div>
              <div className="space-y-2">
                {timelineSteps.map((step) => (
                  <button
                    key={step.id}
                    type="button"
                    role="button"
                    onClick={() => setShowTimelineDialog(false)}
                    className="w-full rounded-lg border border-slate-200 px-3 py-2 text-left"
                  >
                    <p className="text-sm font-semibold text-slate-800">{step.title}</p>
                    {step.description && (
                      <p className="text-xs text-slate-500">{step.description}</p>
                    )}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Input Bar */}
        <div className="px-4 pb-6 pt-4 sm:px-6 sm:pb-8 sm:pt-6">
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
    </div>
  );
}
