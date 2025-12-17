'use client';

import { useMemo, useState } from 'react';
import dynamic from 'next/dynamic';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { AnyAgentEvent } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Play, RefreshCw, Square } from 'lucide-react';

const ConsoleAgentOutput = dynamic(
  () => import('@/components/agent/ConsoleAgentOutput').then((mod) => mod.ConsoleAgentOutput),
  {
    loading: () => <p className="text-sm text-slate-500">Loading console…</p>,
    ssr: false,
  },
);

function createSessionId() {
  return `mock-session-${Date.now()}`;
}

function findLatestTaskId(events: AnyAgentEvent[]): string | null {
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index];
    if ('task_id' in event && event.task_id) {
      return event.task_id;
    }
  }
  return null;
}

export default function MockConsolePage() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    reconnect,
    clearEvents,
  } = useAgentEventStream(sessionId, { useMock: true, enabled: Boolean(sessionId) });

  const latestTaskId = useMemo(() => findLatestTaskId(events), [events]);

  return (
    <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
      <div className="mx-auto flex max-w-6xl flex-col gap-4 lg:gap-6">
        <header className="flex flex-col gap-3 rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
          <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-[11px] font-semibold text-slate-400">
                Mock Streaming · Agent Console
              </p>
              <h1 className="text-xl font-semibold text-slate-900 lg:text-2xl">Streaming final answer preview</h1>
              <p className="text-sm text-slate-600">
                This replay uses mocked agent events, including streaming workflow.result.final updates with attachments.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  clearEvents();
                  setSessionId(null);
                }}
              >
                <Square className="mr-2 h-4 w-4" /> Stop
              </Button>
              <Button
                size="sm"
                variant="default"
                onClick={() => {
                  clearEvents();
                  setSessionId(createSessionId());
                }}
              >
                <RefreshCw className="mr-2 h-4 w-4" /> Restart mock
              </Button>
              {!sessionId && (
                <Button
                  size="sm"
                  onClick={() => {
                    clearEvents();
                    setSessionId(createSessionId());
                  }}
                >
                  <Play className="mr-2 h-4 w-4" /> Start replay
                </Button>
              )}
              {sessionId && (
                <Button size="sm" onClick={reconnect} variant="default">
                  <Play className="mr-2 h-4 w-4" /> Replay stream
                </Button>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
            <span className="rounded-full bg-slate-100 px-3 py-1 font-semibold text-slate-700">
              Session: {sessionId ?? 'stopped'}
            </span>
            {error && (
              <span className="rounded-full bg-red-50 px-3 py-1 font-semibold text-red-700">
                {error}
              </span>
            )}
            {isReconnecting && (
              <span className="rounded-full bg-amber-50 px-3 py-1 font-semibold text-amber-700">
                Reconnecting · attempt {reconnectAttempts}
              </span>
            )}
            {isConnected && !isReconnecting && !error && (
              <span className="rounded-full bg-green-50 px-3 py-1 font-semibold text-green-700">Connected</span>
            )}
          </div>
        </header>

        {sessionId ? (
          <div className="rounded-2xl bg-white/90 p-4 ring-1 ring-slate-200/60 lg:p-6">
            <ConsoleAgentOutput
              events={events}
              isConnected={isConnected}
              isReconnecting={isReconnecting}
              error={error}
              reconnectAttempts={reconnectAttempts}
              onReconnect={reconnect}
              sessionId={sessionId}
              taskId={latestTaskId}
            />
          </div>
        ) : (
          <div className="rounded-2xl bg-white/90 p-6 text-sm text-slate-600 ring-1 ring-slate-200/60">
            Click “Start replay” to load the full console UI.
          </div>
        )}
      </div>
    </div>
  );
}
