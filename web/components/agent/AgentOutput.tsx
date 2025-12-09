'use client';

import { useEffect, useMemo, useState } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
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
    <div className="flex flex-col gap-6">
      <Card className="flex items-center justify-between gap-6 border border-border bg-card px-6 py-5">
        <div className="flex flex-col gap-2">
          <h2 className="text-lg font-semibold text-foreground">Agent Output</h2>
          <div className="flex flex-wrap items-center gap-2 text-xs font-mono text-muted-foreground">
            <Badge variant="outline" className="text-[11px] font-medium">
              {memoryStats.eventCount.toLocaleString()} events
            </Badge>
            <span>
              {Math.round(memoryStats.estimatedBytes / 1024)} KB · {memoryStats.toolCallCount} tool calls · {memoryStats.iterationCount}{' '}
              iterations
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
      </Card>

      {hasTimeline && (
        <Card className="rounded-2xl border bg-card p-6">
          <header className="mb-4 space-y-1">
            <h3 className="text-base font-semibold text-foreground">
              {t('timeline.card.title')}
            </h3>
            <p className="text-sm text-muted-foreground">{t('timeline.card.subtitle')}</p>
          </header>
          <TimelineStepList
            steps={timelineSteps}
            focusedStepId={focusedStepId}
            onStepSelect={(stepId) => {
              setFocusedStepId(stepId);
              setHasUserSelectedStep(true);
            }}
          />
        </Card>
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
