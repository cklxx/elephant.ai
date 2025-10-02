'use client';

import { useState } from 'react';
import { TaskInput } from '@/components/agent/TaskInput';
import { AgentOutput } from '@/components/agent/AgentOutput';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useSSE } from '@/hooks/useSSE';
import { useSessionStore } from '@/hooks/useSessionStore';
import { Card } from '@/components/ui/card';

export default function HomePage() {
  const [sessionId, setSessionId] = useState<string | null>(null);
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
  } = useSSE(sessionId || currentSessionId);

  const handleTaskSubmit = (task: string) => {
    executeTask(
      {
        task,
        session_id: sessionId || currentSessionId || undefined,
      },
      {
        onSuccess: (data) => {
          // Set session ID and connect to SSE
          setSessionId(data.session_id);
          setCurrentSession(data.session_id);
          addToHistory(data.session_id);
        },
        onError: (error) => {
          alert(`Failed to execute task: ${error.message}`);
        },
      }
    );
  };

  const handleNewSession = () => {
    if (confirm('Start a new session? Current session will be preserved.')) {
      setSessionId(null);
      clearEvents();
    }
  };

  return (
    <div className="space-y-8">
      {/* Hero section */}
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold text-gray-900">
          Welcome to ALEX
        </h1>
        <p className="text-lg text-gray-600 max-w-2xl mx-auto">
          An AI-powered programming agent built with hexagonal architecture and ReAct pattern.
          Submit tasks and watch ALEX solve them in real-time.
        </p>
      </div>

      {/* Task input */}
      <Card className="p-6">
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold text-gray-900">New Task</h2>
            {sessionId && (
              <button
                onClick={handleNewSession}
                className="text-sm text-blue-600 hover:text-blue-700"
              >
                Start New Session
              </button>
            )}
          </div>
          <TaskInput
            onSubmit={handleTaskSubmit}
            disabled={isPending}
            loading={isPending}
          />
          {sessionId && (
            <p className="text-xs text-gray-500">
              Session ID: {sessionId}
            </p>
          )}
        </div>
      </Card>

      {/* Agent output */}
      {(sessionId || currentSessionId) && (
        <AgentOutput
          events={events}
          isConnected={isConnected}
          isReconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={reconnect}
        />
      )}

      {/* Empty state */}
      {!sessionId && !currentSessionId && (
        <Card className="p-12">
          <div className="text-center space-y-4">
            <div className="text-6xl">🤖</div>
            <h3 className="text-xl font-semibold text-gray-900">
              Ready to assist
            </h3>
            <p className="text-gray-600">
              Enter a task above to start working with ALEX
            </p>
          </div>
        </Card>
      )}
    </div>
  );
}
