'use client';

import { useState, useRef, useEffect } from 'react';
import { TaskInput } from '@/components/agent/TaskInput';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useSSE } from '@/hooks/useSSE';
import { useSessionStore } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';

export default function HomePage() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const outputRef = useRef<HTMLDivElement>(null);

  const { mutate: executeTask, isPending } = useTaskExecution();
  const { currentSessionId, setCurrentSession, addToHistory } = useSessionStore();

  const {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents,
    reconnect,
    addEvent,
  } = useSSE(sessionId || currentSessionId);

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

    executeTask(
      {
        task,
        session_id: sessionId || currentSessionId || undefined,
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
  };

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]">
      {/* Minimal header - Manus style */}
      <div className="flex-shrink-0 pb-3 mb-3 border-b border-border/50">
        <div className="flex items-center justify-between">
          <div className="flex items-baseline gap-3">
            <h1 className="text-lg font-semibold tracking-tight">ALEX</h1>
            {sessionId && (
              <div className="flex items-center gap-2 text-xs font-mono text-muted-foreground">
                <div className="flex items-center gap-1">
                  <div className={`w-1.5 h-1.5 rounded-full ${isConnected ? 'bg-green-500' : 'bg-gray-400'}`} />
                  <span>{sessionId.slice(0, 8)}</span>
                </div>
              </div>
            )}
          </div>

          {sessionId && (
            <button
              onClick={handleClear}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Clear
            </button>
          )}
        </div>
      </div>

      {/* Terminal output area - scrollable */}
      <div
        ref={outputRef}
        className="flex-1 overflow-y-auto mb-4 scroll-smooth"
      >
        {events.length === 0 ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center space-y-2">
              <p className="text-sm text-muted-foreground">
                Enter a task to get started
              </p>
              <div className="text-xs text-muted-foreground/70 space-y-1">
                <div>• Code generation & debugging</div>
                <div>• Testing & refactoring</div>
                <div>• Architecture planning</div>
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
            sessionId={sessionId}
            taskId={taskId}
          />
        )}
      </div>

      {/* Fixed input at bottom - always visible */}
      <div className="flex-shrink-0 border-t border-border/50 pt-3">
        <TaskInput
          onSubmit={handleTaskSubmit}
          disabled={isPending}
          loading={isPending}
          placeholder={sessionId ? "Continue conversation..." : "Describe your task..."}
        />
      </div>
    </div>
  );
}
