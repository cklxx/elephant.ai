'use client';

import { useEffect, useRef } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { TaskAnalysisCard } from './TaskAnalysisCard';
import { ToolCallCard } from './ToolCallCard';
import { ThinkingIndicator } from './ThinkingIndicator';
import { TaskCompleteCard } from './TaskCompleteCard';
import { ErrorCard } from './ErrorCard';
import { ConnectionStatus } from './ConnectionStatus';

interface AgentOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error?: string | null;
  reconnectAttempts?: number;
  onReconnect?: () => void;
}

export function AgentOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
}: AgentOutputProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [events]);

  return (
    <div className="space-y-6">
      {/* Connection status */}
      <div className="glass-card p-4 rounded-xl shadow-soft flex items-center justify-between">
        <h2 className="text-lg font-bold bg-gradient-to-r from-gray-900 to-gray-700 bg-clip-text text-transparent">
          Agent Output
        </h2>
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
      </div>

      {/* Event stream */}
      <div className="space-y-4 min-h-[400px] max-h-[800px] overflow-y-auto pr-2 scroll-smooth">
        {events.length === 0 ? (
          <div className="glass-card flex flex-col items-center justify-center h-64 text-gray-500 rounded-xl shadow-soft animate-fadeIn">
            <div className="w-16 h-16 rounded-full bg-gradient-to-br from-gray-100 to-gray-200 flex items-center justify-center mb-4 animate-pulse-soft">
              <span className="text-2xl">💭</span>
            </div>
            <p className="font-medium">No events yet. Submit a task to get started.</p>
          </div>
        ) : (
          events.map((event, idx) => (
            <EventCard key={`${event.event_type}-${idx}`} event={event} />
          ))
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}

function EventCard({ event }: { event: AnyAgentEvent }) {
  switch (event.event_type) {
    case 'task_analysis':
      return <TaskAnalysisCard event={event} />;

    case 'thinking':
      return <ThinkingIndicator />;

    case 'tool_call_start':
      return <ToolCallCard event={event} status="running" />;

    case 'tool_call_complete':
      const hasError = 'error' in event && event.error;
      return (
        <ToolCallCard
          event={event}
          status={hasError ? 'error' : 'complete'}
        />
      );

    case 'task_complete':
      return <TaskCompleteCard event={event} />;

    case 'error':
      return <ErrorCard event={event} />;

    case 'iteration_start':
      return (
        <div className="flex items-center gap-2 text-sm text-gray-600 font-medium px-4 py-2 bg-gradient-to-r from-blue-50/50 to-transparent rounded-lg border-l-2 border-blue-400 animate-slideIn">
          <span className="w-2 h-2 bg-blue-500 rounded-full animate-pulse"></span>
          <span>Iteration {event.iteration} of {event.total_iters}</span>
        </div>
      );

    case 'iteration_complete':
      return (
        <div className="flex items-center justify-end gap-3 text-xs text-gray-500 font-medium px-4 py-2 bg-gradient-to-l from-gray-50/50 to-transparent rounded-lg border-r-2 border-gray-300 animate-fadeIn">
          <span className="flex items-center gap-1">
            <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
            Iteration {event.iteration} complete
          </span>
          <span>•</span>
          <span className="px-2 py-0.5 bg-blue-50 text-blue-700 rounded border border-blue-200">
            {event.tokens_used} tokens
          </span>
          <span className="px-2 py-0.5 bg-purple-50 text-purple-700 rounded border border-purple-200">
            {event.tools_run} tools
          </span>
        </div>
      );

    default:
      return null;
  }
}
