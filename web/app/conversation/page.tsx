'use client';

import { Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore, useDeleteSession } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { useConfirmDialog } from '@/components/ui/dialog';
import { useI18n } from '@/lib/i18n';
import { Sidebar, Header, ContentArea } from '@/components/layout';
import { buildToolCallSummaries } from '@/lib/eventAggregation';
import { formatParsedError, getErrorLogPayload, isAPIError, parseError } from '@/lib/errors';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import type { AttachmentPayload, AttachmentUpload } from '@/lib/types';
import { TaskInput } from '@/components/agent/TaskInput';

export function ConversationPageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
  const [isInputManuallyOpened, setIsInputManuallyOpened] = useState(false);
  const [showTimelineDialog, setShowTimelineDialog] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const searchParams = useSearchParams();
  const { t } = useI18n();

  const useMockStream = useMemo(() => searchParams.get('mockSSE') === '1', [searchParams]);

  const { mutate: executeTask, isPending } = useTaskExecution();
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

  const handleTaskSubmit = (task: string, attachments: AttachmentUpload[]) => {
    console.log('[ConversationPage] Task submitted:', { task, attachments });

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
            setCurrentSession(data.session_id);
            addToHistory(data.session_id);
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
              clearCurrentSession();
              removeSession(requestedSessionId);
              clearEvents();

              runExecution(null);
              return;
            }

            console.error(
              '[ConversationPage] Task execution error:',
              getErrorLogPayload(error)
            );
            const parsed = parseError(error, t('common.error.unknown'));
            toast.error(
              t('console.toast.taskFailed'),
              formatParsedError(parsed)
            );
          },
        }
      );
    };

    runExecution(initialSessionId ?? null);
  };

  const handleNewSession = () => {
    setSessionId(null);
    setTaskId(null);
    clearEvents();
    clearCurrentSession();
    setIsInputManuallyOpened(true);
  };

  const handleSessionSelect = (id: string) => {
    if (!id) return;
    clearEvents();
    setSessionId(id);
    setTaskId(null);
    setCurrentSession(id);
    addToHistory(id);
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
          clearCurrentSession();
        }
        toast.success(t('sidebar.session.toast.deleteSuccess'));
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
      }
    }
  };

  const toolSummaries = useMemo(() => buildToolCallSummaries(events), [events]);

  const isSubmitting = useMockStream ? false : isPending;

  const hasConversation = Boolean(resolvedSessionId) || events.length > 0;

  useEffect(() => {
    if (hasConversation && isInputManuallyOpened) {
      setIsInputManuallyOpened(false);
    }
  }, [hasConversation, isInputManuallyOpened]);

  const shouldShowInputBar = hasConversation || isInputManuallyOpened;

  useEffect(() => {
    if (prefillTask && !shouldShowInputBar) {
      setIsInputManuallyOpened(true);
    }
  }, [prefillTask, shouldShowInputBar]);

  const activeSessionLabel = resolvedSessionId
    ? sessionLabels[resolvedSessionId]?.trim()
    : null;
  const sessionBadge = resolvedSessionId
    ? activeSessionLabel || (resolvedSessionId.length > 8 ? `${resolvedSessionId.slice(0, 4)}…${resolvedSessionId.slice(-4)}` : resolvedSessionId)
    : null;

  const emptyState = (
    <div className="flex flex-col items-center justify-center gap-4 text-center">
      <span className="console-quiet-chip">{t('console.empty.badge')}</span>
      <p className="text-base font-semibold text-slate-700">{t('console.empty.title')}</p>
      <p className="console-microcopy max-w-sm text-slate-400">
        {t('console.empty.description')}
      </p>
      {!shouldShowInputBar && (
        <button
          type="button"
          onClick={handleNewSession}
          className="console-button console-button-primary"
        >
          {t('console.connection.newConversation')}
        </button>
      )}
    </div>
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
          title={sessionBadge || t('conversation.header.idle')}
          subtitle={resolvedSessionId ? t('conversation.header.subtitle') : undefined}
          leadingSlot={
            <button
              type="button"
              onClick={() => setIsSidebarOpen((prev) => !prev)}
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
                onClick={() => setShowTimelineDialog(true)}
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
            toolSummaries={toolSummaries}
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
        {shouldShowInputBar && (
          <div className="border-t border-slate-200 bg-white px-6 py-4">
            <TaskInput
              onSubmit={handleTaskSubmit}
              placeholder={
                resolvedSessionId
                  ? t('console.input.placeholder.active')
                  : t('console.input.placeholder.idle')
              }
              disabled={isSubmitting}
              loading={isSubmitting}
              prefill={prefillTask}
              onPrefillApplied={() => setPrefillTask(null)}
            />
          </div>
        )}
      </div>
    </div>
  );
}

export default function ConversationPage() {
  const { t } = useI18n();
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
          {t('app.loading')}
        </div>
      }
    >
      <ConversationPageContent />
    </Suspense>
  );
}
