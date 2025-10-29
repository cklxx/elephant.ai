'use client';

import { useEffect, useMemo, useState } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { ConnectionStatus } from './ConnectionStatus';
import { VirtualizedEventList } from './VirtualizedEventList';
import { ResearchTimeline } from './ResearchTimeline';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
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
    browserDiagnosticsCount: number;
  };
  const timelineSteps = useTimelineSteps(events);
  const hasTimeline = timelineSteps.length > 0;
  const [focusedStepId, setFocusedStepId] = useState<string | null>(null);
  const [hasUserSelectedStep, setHasUserSelectedStep] = useState(false);

  const activeStep = useMemo(
    () => timelineSteps.find((step) => step.status === 'active') ?? null,
    [timelineSteps]
  );
  const latestStep = useMemo(
    () => (timelineSteps.length > 0 ? timelineSteps[timelineSteps.length - 1] : null),
    [timelineSteps]
  );
  const fallbackStepId = activeStep?.id ?? latestStep?.id ?? null;
  const focusedStep = useMemo(
    () => (focusedStepId ? timelineSteps.find((step) => step.id === focusedStepId) ?? null : null),
    [timelineSteps, focusedStepId]
  );
  const focusedEventIndex = focusedStep?.anchorEventIndex ?? null;

  useEffect(() => {
    if (!hasTimeline) {
      if (focusedStepId !== null) {
        setFocusedStepId(null);
      }
      if (hasUserSelectedStep) {
        setHasUserSelectedStep(false);
      }
      return;
    }

    if (!hasUserSelectedStep) {
      if (fallbackStepId !== focusedStepId) {
        setFocusedStepId(fallbackStepId);
      }
      return;
    }

    const exists = timelineSteps.some((step) => step.id === focusedStepId);
    if (!exists) {
      setFocusedStepId(fallbackStepId);
      setHasUserSelectedStep(false);
    }
  }, [
    hasTimeline,
    timelineSteps,
    focusedStepId,
    hasUserSelectedStep,
    fallbackStepId,
  ]);

  return (
    <div className="space-y-6">
      {/* Connection status */}
      <section className="console-card flex items-center justify-between gap-6 px-6 py-5">
        <div className="space-y-2">
          <h2 className="console-pane-title">Agent Output</h2>
          <div className="flex flex-wrap items-center gap-2 text-xs font-mono uppercase tracking-[0.18em] text-muted-foreground">
            <span className="console-quiet-chip text-[11px] uppercase">
              {memoryStats.eventCount.toLocaleString()} EVENTS
            </span>
            <span>
              {Math.round(memoryStats.estimatedBytes / 1024)} KB · {memoryStats.toolCallCount} TOOL CALLS · {memoryStats.iterationCount}{' '}
              ITERATIONS
            </span>
          </div>
        </div>
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
      </section>

      {hasTimeline && (
        <ResearchTimeline
          steps={timelineSteps}
          focusedStepId={focusedStepId}
          onStepSelect={(stepId) => {
            setFocusedStepId(stepId);
            setHasUserSelectedStep(true);
          }}
        />
      )}

      {/* Virtualized event stream */}
      <VirtualizedEventList
        events={events}
        autoScroll={!hasUserSelectedStep}
        focusedEventIndex={focusedEventIndex}
        onJumpToLatest={() => {
          const targetStepId = activeStep?.id ?? latestStep?.id ?? null;
          setFocusedStepId(targetStepId);
          setHasUserSelectedStep(false);
        }}
        className="bg-card/90"
      />
    </div>
  );
}
