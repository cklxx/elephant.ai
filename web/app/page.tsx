'use client';

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { TaskInput } from '@/components/agent/TaskInput';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { ConnectionStatus } from '@/components/agent/ConnectionStatus';
import { ResearchTimeline } from '@/components/agent/ResearchTimeline';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import { LanguageSwitcher } from '@/components/LanguageSwitcher';
import { TranslationKey, useI18n } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Activity, Check, ChevronRight, Pencil, Pin, X } from 'lucide-react';

function HomePageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [focusedStepId, setFocusedStepId] = useState<string | null>(null);
  const [isTimelineDialogOpen, setTimelineDialogOpen] = useState(false);
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState('');
  const outputRef = useRef<HTMLDivElement>(null);
  const highlightTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const highlightedElementRef = useRef<HTMLElement | null>(null);
  const renameInputRef = useRef<HTMLInputElement | null>(null);
  const searchParams = useSearchParams();
  const { t } = useI18n();
  const quickstartKeys: TranslationKey[] = [
    'console.quickstart.items.code',
    'console.quickstart.items.docs',
    'console.quickstart.items.architecture',
  ];

  const useMockStream = useMemo(() => searchParams.get('mockSSE') === '1', [searchParams]);

  const { mutate: executeTask, isPending } = useTaskExecution();
  const {
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
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

  const timelineSteps = useTimelineSteps(events);
  const hasTimeline = timelineSteps.length > 0;

  const timelineProgressCopy = useMemo(() => {
    if (!timelineSteps.length) {
      return {
        statusLabel: t('timeline.label'),
        progressLabel: t('timeline.waiting'),
      };
    }

    const completedCount = timelineSteps.filter((step) => step.status === 'complete').length;
    const statusLabel = (() => {
      const activeStep = timelineSteps.find((step) => step.status === 'active');
      if (activeStep) {
        return t('timeline.status.inProgress', { title: activeStep.title });
      }

      const erroredStep = [...timelineSteps]
        .reverse()
        .find((step) => step.status === 'error');
      if (erroredStep) {
        return t('timeline.status.attention', { title: erroredStep.title });
      }

      const latestComplete = [...timelineSteps]
        .reverse()
        .find((step) => step.status === 'complete');
      if (latestComplete) {
        return t('timeline.status.recent', { title: latestComplete.title });
      }

      return t('timeline.label');
    })();

    return {
      statusLabel,
      progressLabel: t('timeline.progress', {
        completed: completedCount,
        total: timelineSteps.length,
      }),
    };
  }, [timelineSteps, t]);

  const recentSessions = useMemo(() => {
    const pinnedSet = new Set(pinnedSessions);
    return sessionHistory.filter((id) => !pinnedSet.has(id));
  }, [sessionHistory, pinnedSessions]);

  const hasPinnedSessions = pinnedSessions.length > 0;
  const hasRecentSessions = recentSessions.length > 0;

  // Reset focused step when available steps change
  useEffect(() => {
    if (!focusedStepId) {
      const activeStep = timelineSteps.find((step) => step.status === 'active');
      if (activeStep) {
        setFocusedStepId(activeStep.id);
      }
      return;
    }

    const exists = timelineSteps.some((step) => step.id === focusedStepId);
    if (!exists) {
      setFocusedStepId(null);
    }
  }, [timelineSteps, focusedStepId]);

  useEffect(() => {
    if (!hasTimeline) {
      setTimelineDialogOpen(false);
    }
  }, [hasTimeline]);

  useEffect(() => {
    return () => {
      if (highlightTimeoutRef.current) {
        clearTimeout(highlightTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (editingSessionId && renameInputRef.current) {
      renameInputRef.current.focus();
      renameInputRef.current.select();
    }
  }, [editingSessionId]);

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  }, [events]);

  const handleTaskSubmit = (task: string) => {
    console.log('[HomePage] Task submitted:', task);

    // Add user task message to events (manually, since backend doesn't send it)
    const userEvent = {
      event_type: 'user_task' as const,
      timestamp: new Date().toISOString(),
      agent_level: 'core' as const,
      task,
    };
    addEvent(userEvent);

    if (useMockStream) {
      const mockSessionId = sessionId || currentSessionId || `mock-${Date.now().toString(36)}`;
      const mockTaskId = `mock-task-${Date.now().toString(36)}`;
      setSessionId(mockSessionId);
      setTaskId(mockTaskId);
      setCurrentSession(mockSessionId);
      addToHistory(mockSessionId);
      return;
    }

    executeTask(
      {
        task,
        session_id: resolvedSessionId || undefined,
        auto_approve_plan: false,
      },
      {
        onSuccess: (data) => {
          console.log('[HomePage] Task execution started:', data);
          setSessionId(data.session_id);
          setTaskId(data.task_id);
          setCurrentSession(data.session_id);
          addToHistory(data.session_id);
        },
        onError: (error) => {
          console.error('[HomePage] Task execution error:', error);
          toast.error(t('console.toast.taskFailed'), error.message);
        },
      }
    );
  };

  const handleClear = () => {
    setSessionId(null);
    setTaskId(null);
    clearEvents();
    clearCurrentSession();
    setFocusedStepId(null);
    highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
    highlightedElementRef.current = null;
    setEditingSessionId(null);
    setEditingValue('');
  };

  const handleSessionSelect = (id: string) => {
    if (!id) return;
    clearEvents();
    setSessionId(id);
    setTaskId(null);
    setCurrentSession(id);
    addToHistory(id);
    setFocusedStepId(null);
    highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
    highlightedElementRef.current = null;
    setEditingSessionId(null);
    setEditingValue('');
  };

  const handleRenameOpen = (id: string) => {
    setEditingSessionId(id);
    setEditingValue(sessionLabels[id] ?? '');
  };

  const handleRenameSubmit = (id: string) => {
    renameSession(id, editingValue);
    setEditingSessionId(null);
    setEditingValue('');
  };

  const handleRenameCancel = () => {
    setEditingSessionId(null);
    setEditingValue('');
  };

  const handleTimelineStepSelect = useCallback(
    (stepId: string) => {
      setFocusedStepId(stepId);

      const container = outputRef.current;
      if (!container) return;

      const safeId =
        typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
          ? CSS.escape(stepId)
          : stepId;

      const target = container.querySelector<HTMLElement>(
        `[data-anchor-id="${safeId}"]`
      );
      if (!target) return;

      target.scrollIntoView({ behavior: 'smooth', block: 'start', inline: 'nearest' });
      target.focus({ preventScroll: true });

      highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
      highlightedElementRef.current = target;
      target.classList.add('timeline-anchor-highlight');

      if (highlightTimeoutRef.current) {
        clearTimeout(highlightTimeoutRef.current);
      }

      highlightTimeoutRef.current = setTimeout(() => {
        highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
        highlightedElementRef.current = null;
      }, 1800);

      setTimelineDialogOpen(false);
    },
    [setTimelineDialogOpen]
  );

  const isSubmitting = useMockStream ? false : isPending;
  const getSessionBadge = (value: string) =>
    value.length > 8 ? `${value.slice(0, 4)}…${value.slice(-4)}` : value;
  const activeSessionLabel = resolvedSessionId
    ? sessionLabels[resolvedSessionId]?.trim()
    : null;
  const sessionBadge = resolvedSessionId
    ? activeSessionLabel || getSessionBadge(resolvedSessionId)
    : null;

  const renderSessionItem = (id: string, pinned = false) => {
    const isActive = id === resolvedSessionId;
    const label = sessionLabels[id]?.trim();
    const prefix = id.length > 8 ? id.slice(0, 6) : id;
    const suffix = id.length > 4 ? id.slice(-4) : id;
    const isEditing = editingSessionId === id;
    const isPinned = pinned || pinnedSessions.includes(id);

    if (isEditing) {
      return (
        <li key={id}>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              handleRenameSubmit(id);
            }}
            className="flex items-center gap-2 rounded-xl border border-sky-200 bg-white px-3 py-2 shadow-sm"
          >
            <input
              ref={editingSessionId === id ? renameInputRef : undefined}
              value={editingValue}
              onChange={(event) => setEditingValue(event.target.value)}
              placeholder={t('console.history.renamePlaceholder')}
              aria-label={t('console.history.renamePlaceholder')}
              className="flex-1 bg-transparent text-sm text-slate-700 placeholder:text-slate-300 focus:outline-none"
              maxLength={48}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  event.preventDefault();
                  handleRenameCancel();
                }
              }}
            />
            <button
              type="submit"
              className="rounded-full bg-sky-500 p-1.5 text-white shadow-sm transition hover:bg-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
              title={t('console.history.renameConfirm')}
            >
              <Check className="h-3.5 w-3.5" />
              <span className="sr-only">{t('console.history.renameConfirm')}</span>
            </button>
            <button
              type="button"
              onClick={handleRenameCancel}
              className="rounded-full bg-slate-100 p-1.5 text-slate-500 transition hover:bg-slate-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-200"
              title={t('console.history.renameCancel')}
            >
              <X className="h-3.5 w-3.5" />
              <span className="sr-only">{t('console.history.renameCancel')}</span>
            </button>
          </form>
        </li>
      );
    }

    return (
      <li key={id}>
        <div
          className={cn(
            'group flex items-center gap-2 rounded-xl px-2 py-1.5 transition',
            isActive
              ? 'bg-sky-500/10 text-sky-700 ring-1 ring-inset ring-sky-400/50'
              : 'text-slate-600 hover:bg-slate-50'
          )}
        >
          <button
            onClick={() => handleSessionSelect(id)}
            data-testid={`session-history-${id}`}
            aria-current={isActive ? 'true' : undefined}
            className="flex flex-1 flex-col items-start gap-1 rounded-lg px-1 py-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
          >
            <span className="flex items-center gap-1 text-sm font-medium">
              {label || t('console.history.itemPrefix', { id: prefix })}
            </span>
            <span className="text-[10px] font-semibold uppercase tracking-[0.35em] text-slate-400">
              …{suffix}
            </span>
          </button>
          <div className="flex items-center gap-1 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100">
            <button
              type="button"
              onClick={() => togglePinSession(id)}
              className="rounded-full p-1 text-slate-400 transition hover:bg-slate-200 hover:text-slate-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
              title={t(isPinned ? 'console.history.unpin' : 'console.history.pin')}
            >
              <Pin
                className={cn('h-3.5 w-3.5 transition', isPinned ? '-rotate-45 text-sky-500' : '')}
                fill={isPinned ? 'currentColor' : 'none'}
                aria-hidden="true"
              />
              <span className="sr-only">
                {t(isPinned ? 'console.history.unpin' : 'console.history.pin')}
              </span>
            </button>
            <button
              type="button"
              onClick={() => handleRenameOpen(id)}
              className="rounded-full p-1 text-slate-400 transition hover:bg-slate-200 hover:text-slate-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
              title={t('console.history.rename')}
            >
              <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
              <span className="sr-only">{t('console.history.rename')}</span>
            </button>
          </div>
        </div>
      </li>
    );
  };

  return (
    <div className="flex flex-1">
      <div className="console-shell">
          <header className="console-panel flex flex-col gap-4 px-5 py-5 sm:px-6 lg:px-8">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
              <div className="space-y-3">
                <span className="console-quiet-chip tracking-[0.35em] text-slate-500">
                  {t('console.brand')}
                </span>
                <h1 className="text-2xl font-semibold text-slate-900">
                  {resolvedSessionId
                    ? t('console.heading.active', { id: sessionBadge ?? '' })
                    : t('console.heading.idle')}
                </h1>
              </div>
              <LanguageSwitcher variant="toolbar" showLabel={false} />
            </div>
          </header>
          <div className="grid flex-1 gap-6 lg:grid-cols-[minmax(240px,320px),minmax(0,1fr)] xl:grid-cols-[minmax(280px,360px),minmax(0,1fr)]">
            <aside className="console-panel flex h-full flex-col gap-6 p-5 sm:p-6">
              <section className="space-y-4 rounded-2xl border border-slate-100 bg-slate-50/80 p-5 shadow-inner">
                <span className="console-quiet-chip text-slate-500">{t('console.settings.title')}</span>
                <div className="space-y-3 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm">
                  <div className="space-y-0.5">
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                      {t('console.connection.title')}
                    </p>
                    <p className="console-microcopy">{t('console.connection.subtitle')}</p>
                  </div>
                  <ConnectionStatus
                    connected={isConnected}
                    reconnecting={isReconnecting}
                    reconnectAttempts={reconnectAttempts}
                    error={error}
                    onReconnect={reconnect}
                  />
                  {useMockStream && (
                    <div
                      className="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-medium uppercase tracking-wide text-amber-700"
                      data-testid="mock-stream-indicator"
                    >
                      {t('console.connection.mock')}
                    </div>
                  )}
                </div>
                <div className="rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm">
                  <div className="flex items-center justify-between text-xs text-slate-500">
                    <span className="font-semibold uppercase tracking-wide">
                      {t('console.settings.sessionLabel')}
                    </span>
                    <span className="font-medium text-slate-600">
                      {resolvedSessionId ? sessionBadge : t('console.settings.sessionEmpty')}
                    </span>
                  </div>
                  <button
                    onClick={handleClear}
                    className="mt-3 inline-flex w-full items-center justify-center rounded-xl bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-700"
                  >
                    {t('console.connection.newConversation')}
                  </button>
                </div>
              </section>

              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <p className="console-section-title">{t('console.history.title')}</p>
                  <span className="console-microcopy">{t('console.history.subtitle')}</span>
                </div>
                <div className="space-y-2 overflow-hidden rounded-2xl border border-slate-100 bg-white">
                  {hasPinnedSessions && (
                    <div className="space-y-1 border-b border-slate-100 px-3 py-3">
                      <span className="console-microcopy text-[10px] uppercase tracking-[0.3em] text-slate-400">
                        {t('console.history.pinned')}
                      </span>
                      <ul className="space-y-1">
                        {pinnedSessions.map((id) => renderSessionItem(id, true))}
                      </ul>
                    </div>
                  )}

                  {hasRecentSessions && (
                    <div className={cn('space-y-1', hasPinnedSessions ? 'px-3 pb-3' : '')}>
                      {hasPinnedSessions && (
                        <span className="console-microcopy text-[10px] uppercase tracking-[0.3em] text-slate-400">
                          {t('console.history.recents')}
                        </span>
                      )}
                      <ul
                        className={cn(
                          'space-y-1 console-scrollbar',
                          hasPinnedSessions
                            ? 'max-h-40 overflow-y-auto pr-1'
                            : 'max-h-64 overflow-y-auto px-2 py-2'
                        )}
                      >
                        {recentSessions.map((id) => renderSessionItem(id))}
                      </ul>
                    </div>
                  )}

                  {!hasPinnedSessions && !hasRecentSessions && (
                    <div className="px-4 py-6 text-center text-sm text-slate-400">
                      {t('console.history.empty')}
                    </div>
                  )}
                </div>
            </div>

              <div className="space-y-3 rounded-2xl border border-slate-100 bg-white p-4">
                <p className="console-section-title">{t('console.quickstart.title')}</p>
                <div className="flex flex-wrap gap-2">
                  {quickstartKeys.map((key) => (
                    <span key={key} className="console-quiet-chip">
                      {t(key)}
                    </span>
                  ))}
                </div>
              </div>
            </aside>

          <section className="console-panel flex h-full flex-col overflow-hidden">
            <div className="flex flex-col gap-2 border-b border-slate-100 bg-white/80 px-8 py-6">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <h2 className="text-lg font-semibold text-slate-900">
                  {resolvedSessionId
                    ? t('console.thread.sessionPrefix', { id: sessionBadge ?? '' })
                    : t('console.thread.newConversation')}
                </h2>
                <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.3em] text-slate-300">
                  <span>{t('console.thread.autosave')}</span>
                  <span className="h-1 w-1 rounded-full bg-slate-200" />
                  <span>{new Date().toLocaleTimeString()}</span>
                </div>
              </div>
            </div>

            <div className="flex min-h-[420px] flex-1 flex-col">
              <div className="flex flex-1 flex-col gap-6 lg:flex-row">
                {hasTimeline && (
                  <aside className="hidden w-[min(17rem,28vw)] flex-shrink-0 lg:block">
                    <div className="console-scrollbar sticky top-24 max-h-[calc(100vh-14rem)] overflow-y-auto pr-2">
                      <ResearchTimeline
                        steps={timelineSteps}
                        focusedStepId={focusedStepId}
                        onStepSelect={handleTimelineStepSelect}
                      />
                    </div>
                  </aside>
                )}

                <div className="flex min-w-0 flex-1 flex-col">
                  {hasTimeline && (
                    <div className="flex items-center justify-between px-5 pt-6 sm:px-6 lg:px-8 lg:hidden">
                      <div className="text-xs font-medium uppercase tracking-wide text-slate-400">
                        {t('console.timeline.mobileLabel')}
                      </div>
                      <button
                        type="button"
                        onClick={() => setTimelineDialogOpen(true)}
                        className="group inline-flex items-center gap-3 rounded-2xl border border-slate-200 bg-white/90 px-4 py-2 text-left shadow-sm transition hover:border-sky-200 hover:bg-sky-50"
                        aria-haspopup="dialog"
                        aria-expanded={isTimelineDialogOpen}
                        data-testid="mobile-timeline-trigger"
                      >
                        <span className="flex items-center gap-2">
                          <Activity className="h-4 w-4 text-sky-500 transition group-hover:scale-105" />
                          <span className="text-sm font-semibold text-slate-700">
                            {timelineProgressCopy.statusLabel}
                          </span>
                        </span>
                        <span className="flex items-center gap-2 console-microcopy">
                          {timelineProgressCopy.progressLabel}
                          <ChevronRight className="h-3.5 w-3.5 text-slate-400 transition group-hover:translate-x-0.5" />
                        </span>
                      </button>
                    </div>
                  )}
                  <div
                    ref={outputRef}
                    className="console-scrollbar flex-1 overflow-y-auto px-5 py-6 sm:px-6 sm:py-8 lg:px-8"
                  >
                    {events.length === 0 ? (
                      <div className="flex h-full flex-col items-center justify-center gap-5 text-center">
                        <div className="flex items-center gap-3 rounded-full bg-slate-100 px-4 py-2 text-xs font-medium text-slate-500">
                          <span className="inline-flex h-2 w-2 rounded-full bg-sky-400" />
                          {t('console.empty.badge')}
                        </div>
                        <div className="space-y-3">
                          <p className="text-lg font-semibold text-slate-700">{t('console.empty.title')}</p>
                          <p className="console-microcopy max-w-md mx-auto">
                            {t('console.empty.description')}
                          </p>
                        </div>
                      </div>
                    ) : (
                      <TerminalOutput
                        events={events}
                        isConnected={isConnected}
                        isReconnecting={isReconnecting}
                        error={error}
                        reconnectAttempts={reconnectAttempts}
                        onReconnect={reconnect}
                        sessionId={resolvedSessionId}
                        taskId={taskId}
                      />
                    )}
                  </div>

                  <div className="border-t border-slate-100 bg-slate-50/70 px-5 py-5 sm:px-6 sm:py-6 lg:px-8">
                    <TaskInput
                      onSubmit={handleTaskSubmit}
                      disabled={isSubmitting}
                      loading={isSubmitting}
                      placeholder={
                        resolvedSessionId
                          ? t('console.input.placeholder.active')
                          : t('console.input.placeholder.idle')
                      }
                    />
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>

      <Dialog open={isTimelineDialogOpen} onOpenChange={setTimelineDialogOpen}>
        <DialogContent
          className="max-w-2xl"
          onClose={() => setTimelineDialogOpen(false)}
        >
          <DialogHeader>
            <DialogTitle>{t('console.timeline.dialogTitle')}</DialogTitle>
            <DialogDescription className="console-microcopy">
              {t('console.timeline.dialogDescription')}
            </DialogDescription>
          </DialogHeader>
          <div className="console-scrollbar max-h-[70vh] overflow-y-auto pr-2">
            <ResearchTimeline
              steps={timelineSteps}
              focusedStepId={focusedStepId}
              onStepSelect={handleTimelineStepSelect}
            />
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default function HomePage() {
  const { t } = useI18n();
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
          {t('app.loading')}
        </div>
      }
    >
      <HomePageContent />
    </Suspense>
  );
}
