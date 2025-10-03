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
    <div className="space-y-8 gradient-mesh min-h-[calc(100vh-10rem)]">
      {/* Hero section */}
      <div className="text-center space-y-6 py-8 animate-fadeIn">
        <div className="inline-block">
          <h1 className="text-5xl md:text-6xl font-bold gradient-text mb-2">
            Welcome to ALEX
          </h1>
          <div className="h-1 w-32 mx-auto bg-gradient-to-r from-blue-600 to-purple-600 rounded-full"></div>
        </div>
        <p className="text-lg md:text-xl text-gray-600 max-w-2xl mx-auto leading-relaxed">
          An AI-powered programming agent built with hexagonal architecture and ReAct pattern.
          Submit tasks and watch ALEX solve them in real-time.
        </p>
      </div>

      {/* Task input */}
      <div className="animate-scaleIn">
        <Card className="glass-card p-6 shadow-strong hover-lift">
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold bg-gradient-to-r from-gray-900 to-gray-700 bg-clip-text text-transparent">
                New Task
              </h2>
              {sessionId && (
                <button
                  onClick={handleNewSession}
                  className="text-sm font-medium text-blue-600 hover:text-blue-700 transition-all duration-200 hover:scale-105"
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
              <div className="flex items-center gap-2">
                <div className="status-dot-animated bg-green-500"></div>
                <p className="text-xs text-gray-500 font-mono">
                  Session: {sessionId.slice(0, 8)}...
                </p>
              </div>
            )}
          </div>
        </Card>
      </div>

      {/* Agent output */}
      {(sessionId || currentSessionId) && (
        <div className="animate-fadeIn">
          <AgentOutput
            events={events}
            isConnected={isConnected}
            isReconnecting={isReconnecting}
            error={error}
            reconnectAttempts={reconnectAttempts}
            onReconnect={reconnect}
          />
        </div>
      )}

      {/* Empty state */}
      {!sessionId && !currentSessionId && (
        <Card className="glass-card p-12 shadow-medium hover-lift animate-scaleIn">
          <div className="text-center space-y-6">
            <div className="inline-flex items-center justify-center w-24 h-24 rounded-full bg-gradient-to-br from-blue-100 to-purple-100 animate-pulse-soft">
              <span className="text-5xl">🤖</span>
            </div>
            <div>
              <h3 className="text-2xl font-semibold bg-gradient-to-r from-gray-900 to-gray-700 bg-clip-text text-transparent mb-2">
                Ready to assist
              </h3>
              <p className="text-gray-600">
                Enter a task above to start working with ALEX
              </p>
            </div>
            <div className="flex flex-wrap justify-center gap-2 pt-4">
              <span className="px-3 py-1 text-xs font-medium bg-blue-50 text-blue-700 rounded-full border border-blue-200">
                Code Generation
              </span>
              <span className="px-3 py-1 text-xs font-medium bg-purple-50 text-purple-700 rounded-full border border-purple-200">
                Debugging
              </span>
              <span className="px-3 py-1 text-xs font-medium bg-green-50 text-green-700 rounded-full border border-green-200">
                Testing
              </span>
              <span className="px-3 py-1 text-xs font-medium bg-orange-50 text-orange-700 rounded-full border border-orange-200">
                Refactoring
              </span>
            </div>
          </div>
        </Card>
      )}
    </div>
  );
}
