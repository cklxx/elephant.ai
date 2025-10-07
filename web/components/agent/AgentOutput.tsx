'use client';

import { AnyAgentEvent } from '@/lib/types';
import { ConnectionStatus } from './ConnectionStatus';
import { VirtualizedEventList } from './VirtualizedEventList';
import { useMemoryStats } from '@/hooks/useAgentStreamStore';

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
  const memoryStats = useMemoryStats() as {
    eventCount: number;
    estimatedBytes: number;
    toolCallCount: number;
    iterationCount: number;
    researchStepCount: number;
    browserSnapshotCount: number;
  };

  return (
    <div className="space-y-6">
      {/* Connection status */}
      <div className="glass-card p-4 rounded-xl shadow-soft flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h2 className="text-lg font-bold bg-gradient-to-r from-gray-900 to-gray-700 bg-clip-text text-transparent">
            Agent Output
          </h2>
          {/* Memory usage indicator */}
          <div className="text-xs text-gray-500 font-mono">
            {memoryStats.eventCount} events ({Math.round(memoryStats.estimatedBytes / 1024)}KB)
          </div>
        </div>
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
      </div>

      {/* Virtualized event stream */}
      <VirtualizedEventList events={events} autoScroll={true} />
    </div>
  );
}
