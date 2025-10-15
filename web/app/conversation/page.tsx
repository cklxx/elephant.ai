'use client';

import { Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { useI18n } from '@/lib/i18n';
import { Sidebar, Header, ContentArea, InputBar } from '@/components/layout';

function ConversationPageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [prefillTask, setPrefillTask] = useState<string | null>(null);
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

  const handleShare = () => {
    // TODO: Implement share functionality
    console.log('Share clicked');
  };

  const handleExport = () => {
    // TODO: Implement export functionality
    console.log('Export clicked');
  };

  const handleDelete = () => {
    // TODO: Implement delete functionality
    console.log('Delete clicked');
  };

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
          onShare={handleShare}
          onExport={handleExport}
          onDelete={handleDelete}
        />

        {/* Content Area */}
        <ContentArea
          ref={contentRef}
          isEmpty={events.length === 0}
          emptyState={emptyState}
        >
          <TerminalOutput
            events={events}
            isConnected={isConnected}
            isReconnecting={isReconnecting}
            error={error}
            reconnectAttempts={reconnectAttempts}
            onReconnect={reconnect}
          />
        </ContentArea>

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
