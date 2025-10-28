'use client';

import { Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { TaskInput } from '@/components/agent/TaskInput';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { ConnectionStatus } from '@/components/agent/ConnectionStatus';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { TranslationKey, useI18n } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { Check, Pencil, Pin, X } from 'lucide-react';

function ConversationPageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [editingSessionId, setEditingSessionId] = useState<string | null>(null);
  const [editingValue, setEditingValue] = useState('');
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
  const outputRef = useRef<HTMLDivElement>(null);
  const renameInputRef = useRef<HTMLInputElement | null>(null);
  const searchParams = useSearchParams();
  const { t } = useI18n();
  const quickstartKeys: TranslationKey[] = [
    'console.quickstart.items.code',
    'console.quickstart.items.docs',
    'console.quickstart.items.architecture',
  ];
  const pinnedHistoryKey: TranslationKey = 'console.history.pinned';
  const recentHistoryKey: TranslationKey = 'console.history.recents';
  const emptyHistoryKey: TranslationKey = 'console.history.empty';

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

  const recentSessions = useMemo(() => {
    const pinnedSet = new Set(pinnedSessions);
    return sessionHistory.filter((id) => !pinnedSet.has(id));
  }, [sessionHistory, pinnedSessions]);

  const hasPinnedSessions = pinnedSessions.length > 0;
  const hasRecentSessions = recentSessions.length > 0;

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
    console.log('[ConversationPage] Task submitted:', task);

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
          console.error('[ConversationPage] Task execution error:', error);
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

  const isSubmitting = useMockStream ? false : isPending;

  const handleQuickstartSelect = (key: TranslationKey) => {
    const suggestion = t(key);
    if (!suggestion || isSubmitting) {
      return;
    }

    setPrefillTask(suggestion);

    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  };

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
        <div className="grid flex-1 gap-6 xl:grid-cols-[260px_minmax(0,1fr)]">
          <aside className="flex flex-col gap-6">
            <div className="console-panel p-5 sm:p-6">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
                    {t('console.settings.sessionLabel')}
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
                    }}
                    className="text-[11px] font-semibold uppercase tracking-[0.3em] text-slate-300 transition hover:text-slate-500"
                  >
                    {t('console.history.clearSelection')}
                  </button>
                )}
              </div>
              <p className="console-microcopy mt-1 text-slate-400">
                {t('console.history.subtitle')}
              </p>

              <div className="mt-4 space-y-4">
                {hasPinnedSessions && (
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-300">
                      {t(pinnedHistoryKey)}
                    </p>
                    <ul className="space-y-2">{pinnedSessions.map((id) => renderSessionItem(id, true))}</ul>
                  </div>
                )}

                {hasRecentSessions ? (
                  <div className="space-y-2">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.35em] text-slate-300">
                      {t(recentHistoryKey)}
                    </p>
                    <ul className="space-y-2">{recentSessions.map((id) => renderSessionItem(id))}</ul>
                  </div>
                ) : (
                  <div className="flex min-h-[120px] flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-slate-200 bg-slate-50/70 text-center">
                    <span className="console-quiet-chip">{t(emptyHistoryKey)}</span>
                  </div>
                )}
              </div>
            </div>

            <div className="console-panel p-5 sm:p-6">
              <span className="console-quiet-chip uppercase tracking-[0.35em] text-slate-400">
                {t('console.quickstart.title')}
              </span>
              <div className="mt-4 space-y-2">
                {quickstartKeys.map((key) => (
                  <button
                    key={key}
                    type="button"
                    onClick={() => handleQuickstartSelect(key)}
                    className="flex w-full items-center justify-between gap-3 rounded-2xl border border-slate-200 bg-white px-4 py-3 text-left text-sm font-medium text-slate-600 shadow-sm transition hover:border-sky-200 hover:text-sky-600"
                    aria-label={t('conversation.quickstart.useSuggestion', {
                      suggestion: t(key),
                    })}
                    title={t('conversation.quickstart.useSuggestion', {
                      suggestion: t(key),
                    })}
                  >
                    <span>{t(key)}</span>
                  </button>
                ))}
              </div>
            </div>
          </aside>

          <main className="flex flex-col gap-6">
            <section className="console-panel flex min-h-[520px] flex-1 flex-col overflow-hidden">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-5 py-5 sm:px-8">
                <div className="space-y-1">
                  <span className="console-quiet-chip uppercase tracking-[0.3em] text-slate-400">
                    {t('conversation.header.label')}
                  </span>
                  <p className="text-lg font-semibold text-slate-900">
                    {resolvedSessionId
                      ? t('conversation.header.active', { id: sessionBadge ?? '' })
                      : t('conversation.header.idle')}
                  </p>
                </div>
              </div>

              <div className="flex flex-1 flex-col">
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
                    />
                  )}
                </div>
                <div className="border-t border-slate-100 bg-slate-50/60 px-5 py-4 sm:px-8">
                  <TaskInput
                    onSubmit={handleTaskSubmit}
                    disabled={isSubmitting}
                    loading={isSubmitting}
                    placeholder={
                      resolvedSessionId
                        ? t('console.input.placeholder.active')
                        : t('console.input.placeholder.idle')
                    }
                    prefill={prefillTask}
                    onPrefillApplied={() => setPrefillTask(null)}
                  />
                </div>
              </div>
            </section>
          </main>

        </div>
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
