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
    <div className="bg-app-canvas">
      <div className="console-shell">
        <div className="grid flex-1 gap-6 xl:grid-cols-[250px_minmax(0,1fr)_300px]">
          <aside className="flex flex-col gap-6">
            <div className="console-panel p-5 sm:p-6">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                    {t('console.settings.title')}
                  </p>
                  <p className="mt-2 text-base font-semibold text-slate-800">
                    {sessionBadge ?? t('console.settings.sessionNone')}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={handleClear}
                  className="text-[11px] font-semibold uppercase tracking-[0.3em] text-slate-300 transition hover:text-slate-500"
                >
                  {t('console.settings.reset')}
                </button>
              </div>
              <div className="mt-6 space-y-4">
                <div className="space-y-3">
                  <ConnectionStatus
                    connected={isConnected}
                    reconnecting={isReconnecting}
                    reconnectAttempts={reconnectAttempts}
                    error={error}
                    onReconnect={reconnect}
                  />
                  {useMockStream && (
                    <div
                      className="inline-flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.3em] text-amber-700"
                      data-testid="mock-stream-indicator"
                    >
                      {t('console.connection.mock')}
                    </div>
                  )}
                </div>
                <div className="grid gap-3 rounded-2xl border border-slate-200/70 bg-slate-50/60 px-3 py-3 text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                  <div className="flex items-center justify-between">
                    <span>{t('console.settings.timelineStatus')}</span>
                    <span className="text-slate-600">
                      {hasTimeline
                        ? timelineProgressCopy.progressLabel
                        : t('console.timeline.waiting')}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span>{t('console.settings.sessionStatus')}</span>
                    <span className="text-slate-600">
                      {resolvedSessionId
                        ? t('console.thread.sessionPrefix', { id: sessionBadge ?? '' })
                        : t('console.thread.newConversation')}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div className="console-panel p-5 sm:p-6">
              <div className="flex items-center justify-between gap-3">
                <p className="text-sm font-semibold text-slate-700">
                  {t('console.history.title')}
                </p>
                {sessionHistory.length > 0 && (
                  <button
                    type="button"
                    onClick={() => {
                      clearEvents();
                      clearCurrentSession();
                      setSessionId(null);
                      setTaskId(null);
                      setFocusedStepId(null);
                    }}
                    className="text-[11px] font-semibold uppercase tracking-[0.3em] text-slate-300 transition hover:text-slate-500"
                  >
                    {t('console.history.clearSelection')}
                  </button>
                )}
              </div>

              <div className="mt-4 space-y-4">
                {hasPinnedSessions && (
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-300">
                      {t('console.history.pinned')}
                    </p>
                    <ul className="space-y-2">{pinnedSessions.map((id) => renderSessionItem(id, true))}</ul>
                  </div>
                )}

                {hasRecentSessions ? (
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-300">
                      {t('console.history.recent')}
                    </p>
                    <ul className="space-y-2">{recentSessions.map((id) => renderSessionItem(id))}</ul>
                  </div>
                ) : (
                  <div className="flex min-h-[120px] flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 text-center">
                    <span className="console-quiet-chip">{t('console.history.empty')}</span>
                    <p className="console-microcopy max-w-[200px]">{t('console.history.emptyDescription')}</p>
                  </div>
                )}
              </div>
            </div>
          </aside>

          <main className="flex flex-col gap-6">
            <section className="console-panel px-5 py-6 sm:px-8 sm:py-8">
              <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
                <div className="space-y-2">
                  <span className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                    {t('console.hero.label')}
                  </span>
                  <h1 className="text-3xl font-semibold text-slate-900">
                    {resolvedSessionId
                      ? t('console.hero.title.active', { id: sessionBadge ?? '' })
                      : t('console.hero.title.idle')}
                  </h1>
                  <p className="console-microcopy max-w-xl text-slate-500">
                    {t('console.hero.subtitle')}
                  </p>
                </div>
                <div className="flex items-center gap-3 self-start">
                  <LanguageSwitcher variant="toolbar" showLabel={false} />
                  {hasTimeline && (
                    <div className="hidden text-right lg:block">
                      <p className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                        {timelineProgressCopy.statusLabel}
                      </p>
                      <p className="console-microcopy text-slate-500">
                        {timelineProgressCopy.progressLabel}
                      </p>
                    </div>
                  )}
                </div>
              </div>

              <div className="mt-6 grid gap-3 sm:grid-cols-3">
                {quickstartKeys.map((key) => (
                  <span
                    key={key}
                    className="flex items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm font-medium text-slate-600 shadow-[0_10px_30px_rgba(15,23,42,0.05)]"
                  >
                    {t(key)}
                    <ChevronRight className="h-4 w-4 text-slate-300" />
                  </span>
                ))}
              </div>

              <div className="mt-6 rounded-2xl border border-slate-200 bg-slate-50/60 p-4">
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
            </section>

            <section className="console-panel flex min-h-[420px] flex-1 flex-col overflow-hidden">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-5 py-5 sm:px-8">
                <div className="space-y-1">
                  <h2 className="text-lg font-semibold text-slate-900">
                    {resolvedSessionId
                      ? t('console.thread.sessionPrefix', { id: sessionBadge ?? '' })
                      : t('console.thread.newConversation')}
                  </h2>
                  <p className="console-microcopy text-slate-400">
                    {t('console.thread.autosave')} · {new Date().toLocaleTimeString()}
                  </p>
                </div>
                {hasTimeline && (
                  <button
                    type="button"
                    onClick={() => setTimelineDialogOpen(true)}
                    className="inline-flex items-center gap-2 rounded-full border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-500 transition hover:border-sky-200 hover:text-sky-600 lg:hidden"
                    aria-haspopup="dialog"
                    aria-expanded={isTimelineDialogOpen}
                    data-testid="mobile-timeline-trigger"
                  >
                    <Activity className="h-3.5 w-3.5" />
                    <span>{timelineProgressCopy.statusLabel}</span>
                  </button>
                )}
              </div>

              <div className="flex flex-1 flex-col lg:flex-row">
                <div className="flex min-w-0 flex-1 flex-col">
                  <div
                    ref={outputRef}
                    className="console-scrollbar flex-1 overflow-y-auto px-5 py-6 sm:px-8"
                  >
                    {events.length === 0 ? (
                      <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
                        <span className="console-quiet-chip">{t('console.empty.badge')}</span>
                        <p className="text-base font-semibold text-slate-700">{t('console.empty.title')}</p>
                        <p className="console-microcopy max-w-sm text-slate-400">
                          {t('console.empty.description')}
                        </p>
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
                </div>

                {hasTimeline && (
                  <aside className="hidden w-[280px] flex-shrink-0 border-l border-slate-100 px-5 py-6 lg:block">
                    <div className="console-scrollbar sticky top-24 max-h-[calc(100vh-16rem)] overflow-y-auto pr-2">
                      <ResearchTimeline
                        steps={timelineSteps}
                        focusedStepId={focusedStepId}
                        onStepSelect={handleTimelineStepSelect}
                      />
                    </div>
                  </aside>
                )}
              </div>
            </section>
          </main>

          <aside className="flex flex-col gap-6">
            <div className="console-panel p-5 sm:p-6">
              <p className="text-sm font-semibold text-slate-700">
                {t('console.quickstart.title')}
              </p>
              <div className="mt-4 grid gap-3">
                {quickstartKeys.map((key) => (
                  <div
                    key={key}
                    className="rounded-2xl border border-slate-200 bg-slate-50/60 px-4 py-3 text-sm font-medium text-slate-500"
                  >
                    {t(key)}
                  </div>
                ))}
              </div>
            </div>

            <div className="console-panel p-5 sm:p-6">
              <p className="text-sm font-semibold text-slate-700">
                {t('console.timeline.sidebarTitle')}
              </p>
              <p className="console-microcopy mt-2 text-slate-400">
                {t('console.timeline.sidebarDescription')}
              </p>
              {hasTimeline ? (
                <div className="mt-4 hidden lg:block">
                  <p className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                    {timelineProgressCopy.statusLabel}
                  </p>
                  <p className="console-microcopy text-slate-500">
                    {timelineProgressCopy.progressLabel}
                  </p>
                </div>
              ) : (
                <div className="mt-6 flex items-center justify-between rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 px-4 py-3 text-xs font-semibold uppercase tracking-[0.3em] text-slate-300">
                  <span>{t('console.timeline.waiting')}</span>
                  <Activity className="h-4 w-4 text-slate-200" />
                </div>
              )}
            </div>
          </aside>
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
