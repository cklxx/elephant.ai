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

function HomePageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const outputRef = useRef<HTMLDivElement>(null);
  const searchParams = useSearchParams();

  const useMockStream = useMemo(() => searchParams.get('mockSSE') === '1', [searchParams]);

  const { mutate: executeTask, isPending } = useTaskExecution();
  const {
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
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
          toast.error('Task execution failed', error.message);
        },
      }
    );
  };

  const handleClear = () => {
    setSessionId(null);
    setTaskId(null);
    clearEvents();
    clearCurrentSession();
  };

  const isSubmitting = useMockStream ? false : isPending;
  const sessionBadge = resolvedSessionId?.slice(0, 8);

  return (
    <div className="relative min-h-[calc(100vh-6rem)] overflow-hidden">
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_top,theme(colors.sky.500/12),transparent_55%)]" />
      <div className="pointer-events-none absolute inset-x-0 top-1/3 -z-10 h-1/2 bg-gradient-to-b from-transparent via-background/60 to-background" />

      <div className="relative flex h-full flex-col gap-6">
        <div className="manus-section">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
            <div className="space-y-2">
              <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-background/80 px-3 py-1 text-[11px] uppercase tracking-wide text-muted-foreground">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                Manus Research Console
              </div>
              <div>
                <h1 className="manus-heading text-2xl tracking-tight">Agent Operations</h1>
                <p className="manus-caption text-muted-foreground/90">
                  {resolvedSessionId
                    ? `Active session • ${sessionBadge}`
                    : 'Describe your objective to launch a Manus session'}
                </p>
              </div>
            </div>

            <div className="flex flex-col items-start gap-3 sm:flex-row sm:items-center">
              {useMockStream && (
                <span
                  className="manus-badge manus-badge-outline text-xs uppercase tracking-wide"
                  data-testid="mock-stream-indicator"
                >
                  Mock Stream
                </span>
              )}

              <div className="rounded-lg border border-border/60 bg-background/80 px-3 py-2 shadow-sm" data-testid="connection-status">
                <ConnectionStatus
                  connected={isConnected}
                  reconnecting={isReconnecting}
                  reconnectAttempts={reconnectAttempts}
                  error={error}
                  onReconnect={reconnect}
                />
              </div>

              {resolvedSessionId && (
                <button
                  onClick={handleClear}
                  className="manus-button-ghost text-xs"
                >
                  Clear Session
                </button>
              )}
            </div>
          </div>
        </div>

        <div className="manus-card relative flex flex-1 flex-col overflow-hidden rounded-xl border border-border/70 bg-card/80 shadow-lg backdrop-blur">
          <div className="pointer-events-none absolute inset-x-0 top-0 h-12 bg-gradient-to-b from-background/60 via-background/10 to-transparent" />
          <div
            ref={outputRef}
            className="relative flex-1 overflow-y-auto scroll-smooth px-4 py-6"
          >
            {events.length === 0 ? (
              <div className="flex h-full flex-col items-center justify-center gap-4 text-center text-sm text-muted-foreground/90">
                <div className="inline-flex h-12 w-12 items-center justify-center rounded-full border border-dashed border-border/70 text-muted-foreground">
                  <span className="text-xl">⌘</span>
                </div>
                <div className="space-y-3 max-w-sm">
                  <p className="text-base font-semibold text-foreground">No events yet</p>
                  <p className="text-xs text-muted-foreground/80">
                    Submit a task to begin streaming Manus events. Tool output, planning updates, and final responses will appear here.
                  </p>
                  <div className="text-xs text-muted-foreground/60 space-y-1 font-mono">
                    <div>• Code generation &amp; debugging</div>
                    <div>• Testing &amp; refactoring</div>
                    <div>• Architecture research</div>
                  </div>
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

          <div className="border-t border-border/60 bg-background/80 px-4 py-4">
            <TaskInput
              onSubmit={handleTaskSubmit}
              disabled={isSubmitting}
              loading={isSubmitting}
              placeholder={resolvedSessionId ? 'Continue conversation...' : 'Describe your task...'}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
          Loading Manus console…
        </div>
      }
    >
      <HomePageContent />
    </Suspense>
  );
}
