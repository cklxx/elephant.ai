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
    <div className="space-y-4">
      {/* Connection status */}
      <div className="flex items-center justify-between border-b pb-3">
        <h2 className="text-lg font-semibold text-gray-900">Agent Output</h2>
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
      </div>

      {/* Event stream */}
      <div className="space-y-4 min-h-[400px] max-h-[800px] overflow-y-auto pr-2">
        {events.length === 0 ? (
          <div className="flex items-center justify-center h-64 text-gray-500">
            <p>No events yet. Submit a task to get started.</p>
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
        <div className="text-sm text-gray-500 italic">
          Iteration {event.iteration} of {event.total_iters}
        </div>
      );

    case 'iteration_complete':
      return (
        <div className="text-xs text-gray-400 text-right">
          Iteration {event.iteration} complete - {event.tokens_used} tokens, {event.tools_run} tools
        </div>
      );

    default:
      return null;
  }
}
