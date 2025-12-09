'use client';

import { useEffect, useMemo, useState } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { ConnectionStatus } from './ConnectionStatus';
import { VirtualizedEventList } from './VirtualizedEventList';
import { TimelineStepList } from './TimelineStepList';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import { useMemoryStats } from '@/hooks/useAgentStreamStore';
import { useTranslation } from '@/lib/i18n';

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
  const t = useTranslation();
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
      if (focusedStepId !== null) setFocusedStepId(null);
      if (hasUserSelectedStep) setHasUserSelectedStep(false);
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
    <div className="flex flex-col gap-8 max-w-4xl mx-auto w-full">
      {/* Header Status - Minimal */}
      <div className="flex items-center justify-between px-2">
        {/* Could put a title here if needed, but keeping it clean */}
        <div />
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
      </div>

      {hasTimeline && (
        <div className="rounded-xl border border-border/40 bg-card/50 p-1">
          <TimelineStepList
            steps={timelineSteps}
            focusedStepId={focusedStepId}
            onStepSelect={(stepId) => {
              setFocusedStepId(stepId);
              setHasUserSelectedStep(true);
            }}
          />
        </div>
      )}

      {/* Main Stream */}
      <div className="rounded-2xl border border-border/40 bg-background/50 overflow-hidden shadow-sm min-h-[500px]">
        <VirtualizedEventList
          events={events}
          autoScroll={!hasUserSelectedStep}
          focusedEventIndex={focusedEventIndex}
          onJumpToLatest={() => {
            const targetStepId = activeStep?.id ?? latestStep?.id ?? null;
            setFocusedStepId(targetStepId);
            setHasUserSelectedStep(false);
          }}
          className="bg-transparent"
        />
      </div>
    </div>
  );
}
