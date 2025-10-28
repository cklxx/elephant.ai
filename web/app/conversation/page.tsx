'use client';

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { useI18n } from '@/lib/i18n';
import { Sidebar, Header, ContentArea, InputBar } from '@/components/layout';
import { buildToolCallSummaries } from '@/lib/eventAggregation';
import {
  buildEnvironmentPlan,
  formatEnvironmentPlanShareText,
  serializeEnvironmentPlan,
} from '@/lib/environmentPlan';
import { EnvironmentSummaryCard } from '@/components/environment/EnvironmentSummaryCard';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';

function ConversationPageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
  const [showTimelineDialog, setShowTimelineDialog] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const searchParams = useSearchParams();
  const { t } = useI18n();

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
    environmentPlans = {},
    saveEnvironmentPlan,
    toggleEnvironmentTodo,
    clearEnvironmentPlan,
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

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, [events]);

  const handleTaskSubmit = (task: string) => {
    console.log('[ConversationPage] Task submitted:', task);

    // Add user task message to events
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

  const handleNewSession = () => {
    setSessionId(null);
    setTaskId(null);
    clearEvents();
    clearCurrentSession();
  };

  const handleSessionSelect = (id: string) => {
    if (!id) return;
    clearEvents();
    setSessionId(id);
    setTaskId(null);
    setCurrentSession(id);
    addToHistory(id);
  };

  const toolSummaries = useMemo(() => buildToolCallSummaries(events), [events]);
  const existingPlan = resolvedSessionId ? environmentPlans[resolvedSessionId] : undefined;
  const environmentPlan = useMemo(() => {
    if (!resolvedSessionId) {
      return null;
    }
    return existingPlan ?? buildEnvironmentPlan(resolvedSessionId, toolSummaries);
  }, [resolvedSessionId, existingPlan, toolSummaries]);

  const handleSharePlan = useCallback(async () => {
    if (!environmentPlan) {
      toast.error(t('conversation.environment.actions.noPlan'));
      return;
    }

    const shareText = formatEnvironmentPlanShareText(environmentPlan);
    const title = t('conversation.environment.actions.shareTitle', {
      session: environmentPlan.sessionId,
    });

    try {
      if (navigator.share) {
        await navigator.share({ title, text: shareText });
        toast.success(t('conversation.environment.actions.shareSuccess'));
        return;
      }
    } catch (error) {
      console.error('ConversationPage share via Web Share API failed', error);
    }

    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(shareText);
        toast.success(t('conversation.environment.actions.shareCopied'));
        return;
      }
    } catch (error) {
      console.error('ConversationPage share copy failed', error);
    }

    try {
      const textarea = document.createElement('textarea');
      textarea.value = shareText;
      textarea.setAttribute('readonly', 'true');
      textarea.style.position = 'absolute';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      toast.success(t('conversation.environment.actions.shareCopied'));
    } catch (error) {
      console.error('ConversationPage share fallback failed', error);
      toast.error(t('conversation.environment.actions.shareFailure'));
    }
  }, [environmentPlan, t]);

  const handleExportPlan = useCallback(() => {
    if (!environmentPlan) {
      toast.error(t('conversation.environment.actions.noPlan'));
      return;
    }

    try {
      const serialized = serializeEnvironmentPlan(environmentPlan);
      const json = JSON.stringify(serialized, null, 2);
      const blob = new Blob([json], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `sandbox-plan-${environmentPlan.sessionId}.json`;
      anchor.click();
      URL.revokeObjectURL(url);
      toast.success(t('conversation.environment.actions.exportSuccess'));
    } catch (error) {
      console.error('ConversationPage export failed', error);
      toast.error(t('conversation.environment.actions.exportFailure'));
    }
  }, [environmentPlan, t]);

  const handleDeletePlan = useCallback(() => {
    if (!resolvedSessionId) {
      toast.error(t('conversation.environment.actions.noPlan'));
      return;
    }

    clearEnvironmentPlan(resolvedSessionId);
    setSessionId(null);
    setTaskId(null);
    clearEvents();
    clearCurrentSession();
    toast.success(t('conversation.environment.actions.deleteSuccess'));
  }, [
    clearCurrentSession,
    clearEnvironmentPlan,
    clearEvents,
    resolvedSessionId,
    t,
  ]);

  const isSubmitting = useMockStream ? false : isPending;

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
    </div>
  );

  const timelineSteps = useTimelineSteps(events);

  useEffect(() => {
    if (timelineSteps.length === 0 && showTimelineDialog) {
      setShowTimelineDialog(false);
    }
  }, [timelineSteps, showTimelineDialog]);

  const handleTodoToggle = useCallback(
    (todoId: string) => {
      if (!resolvedSessionId) {
        return;
      }
      toggleEnvironmentTodo(resolvedSessionId, todoId);
    },
    [resolvedSessionId, toggleEnvironmentTodo]
  );

  useEffect(() => {
    if (!resolvedSessionId) {
      return;
    }
    const nextPlan = buildEnvironmentPlan(resolvedSessionId, toolSummaries, existingPlan);
    saveEnvironmentPlan(resolvedSessionId, nextPlan);
  }, [resolvedSessionId, toolSummaries, existingPlan, saveEnvironmentPlan]);

  return (
    <div className="flex h-screen bg-white">
      {/* Left Sidebar */}
      <Sidebar
        sessionHistory={sessionHistory}
        pinnedSessions={pinnedSessions}
        sessionLabels={sessionLabels}
        currentSessionId={resolvedSessionId}
        onSessionSelect={handleSessionSelect}
        onSessionRename={renameSession}
        onSessionPin={togglePinSession}
        onNewSession={handleNewSession}
      />

      {/* Main Content Area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Header */}
        <Header
          title={sessionBadge || t('conversation.header.idle')}
          subtitle={resolvedSessionId ? t('conversation.header.subtitle') : undefined}
          onShare={environmentPlan ? handleSharePlan : undefined}
          onExport={environmentPlan ? handleExportPlan : undefined}
          onDelete={resolvedSessionId ? handleDeletePlan : undefined}
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
          {environmentPlan && (
            <EnvironmentSummaryCard plan={environmentPlan} onToggleTodo={handleTodoToggle} />
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
        <InputBar
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
