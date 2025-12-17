'use client';

import { useMemo, useState } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { ConnectionStatus } from './ConnectionStatus';
import { VirtualizedEventList } from './VirtualizedEventList';
import { TimelineStepList } from './TimelineStepList';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
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
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null);

  const activeStep = useMemo(
    () => timelineSteps.find((step) => step.status === 'active') ?? null,
    [timelineSteps]
  );
  const latestStep = useMemo(
    () => (timelineSteps.length > 0 ? timelineSteps[timelineSteps.length - 1] : null),
    [timelineSteps]
  );
  const fallbackStepId = activeStep?.id ?? latestStep?.id ?? null;
  const resolvedSelectedStepId = useMemo(() => {
    if (!selectedStepId) {
      return null;
    }
    return timelineSteps.some((step) => step.id === selectedStepId)
      ? selectedStepId
      : null;
  }, [selectedStepId, timelineSteps]);
  const focusedStepId = resolvedSelectedStepId ?? fallbackStepId;
  const hasUserSelectedStep = resolvedSelectedStepId !== null;
  const focusedStep = useMemo(() => {
    if (!focusedStepId) {
      return null;
    }
    return timelineSteps.find((step) => step.id === focusedStepId) ?? null;
  }, [timelineSteps, focusedStepId]);
  const focusedEventIndex = focusedStep?.anchorEventIndex ?? null;

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
              setSelectedStepId(stepId);
            }}
          />
        </div>
      )}

      {/* Main Stream */}
      <div className="min-h-[500px] overflow-hidden rounded-2xl border border-border/40 bg-background/50">
        <VirtualizedEventList
          events={events}
          autoScroll={!hasUserSelectedStep}
          focusedEventIndex={focusedEventIndex}
          onJumpToLatest={() => {
            setSelectedStepId(null);
          }}
          className="bg-transparent"
        />
      </div>
    </div>
  );
}
