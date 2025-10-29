'use client';

import { useEffect, useMemo, useRef } from 'react';
import { Card } from '@/components/ui/card';
import {
  Circle,
  CheckCircle2,
  XCircle,
  Loader2,
  Clock,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { cn, formatDuration } from '@/lib/utils';
import { useState } from 'react';
import { useTranslation } from '@/lib/i18n';

export type StepStatus = 'pending' | 'active' | 'complete' | 'error';

export interface TimelineStep {
  id: string;
  title: string;
  description?: string;
  status: StepStatus;
  startTime?: number;
  endTime?: number;
  duration?: number;
  toolsUsed?: string[];
  tokensUsed?: number;
  error?: string;
  anchorEventIndex?: number;
}

interface ResearchTimelineProps {
  steps: TimelineStep[];
  className?: string;
  focusedStepId?: string | null;
  onStepSelect?: (stepId: string) => void;
}

export function ResearchTimeline({
  steps,
  className,
  focusedStepId,
  onStepSelect,
}: ResearchTimelineProps) {
  const activeStepRef = useRef<HTMLDivElement>(null);
  const [expandedSteps, setExpandedSteps] = useState<Set<string>>(new Set());
  const t = useTranslation();

  const { completedCount, totalCount, progressPercent, overallDurationLabel } = useMemo(() => {
    const total = steps.length;
    const completed = steps.filter((step) => step.status === 'complete').length;
    const progress = total === 0 ? 0 : Math.min(100, Math.max(0, Math.round((completed / total) * 100)));

    const startTimes = steps
      .map((step) => step.startTime)
      .filter((value): value is number => typeof value === 'number');
    const endTimes = steps
      .map((step) => step.endTime)
      .filter((value): value is number => typeof value === 'number');

    const earliestStart = startTimes.length > 0 ? Math.min(...startTimes) : null;
    const latestEnd = endTimes.length > 0 ? Math.max(...endTimes) : null;

    const durationLabel =
      earliestStart !== null && latestEnd !== null && latestEnd >= earliestStart
        ? formatDuration(latestEnd - earliestStart)
        : null;

    return {
      completedCount: completed,
      totalCount: total,
      progressPercent: progress,
      overallDurationLabel: durationLabel,
    };
  }, [steps]);

  // Auto-scroll to active step
  useEffect(() => {
    if (activeStepRef.current) {
      activeStepRef.current.scrollIntoView({
        behavior: 'smooth',
        block: 'center',
      });
    }
  }, [steps, focusedStepId]);

  const toggleExpand = (stepId: string) => {
    setExpandedSteps((prev) => {
      const next = new Set(prev);
      if (next.has(stepId)) {
        next.delete(stepId);
      } else {
        next.add(stepId);
      }
      return next;
    });
  };

  return (
    <Card className={cn('console-card px-6 py-6', className)}>
      <div className="mb-4 space-y-4">
        <h3 className="text-base font-semibold uppercase tracking-[0.22em] text-foreground">
          {t('timeline.card.title')}
        </h3>
        <p className="console-microcopy mt-1 text-muted-foreground">{t('timeline.card.subtitle')}</p>
        {totalCount > 0 && (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[0.28em] text-muted-foreground">
              <span>{t('timeline.card.progressLabel')}</span>
              <span className="text-foreground/70 tracking-[0.1em]">
                {t('timeline.card.progressSummary', {
                  completed: completedCount,
                  total: totalCount,
                })}
              </span>
            </div>
            <div className="relative h-2 rounded-full border border-border bg-background">
              <div
                className="absolute inset-y-0 left-0 rounded-full bg-foreground"
                style={{ width: `${progressPercent}%` }}
                aria-hidden="true"
              />
            </div>
            {overallDurationLabel && (
              <div className="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
                {t('timeline.card.totalDuration', { duration: overallDurationLabel })}
              </div>
            )}
          </div>
        )}
      </div>

      <div className="space-y-3">
        {steps.map((step, idx) => {
          const isActive = step.status === 'active';
          const isFocused = focusedStepId === step.id;
          const isExpanded = expandedSteps.has(step.id);
          const isComplete = step.status === 'complete';
          const hasDetails = step.toolsUsed || step.tokensUsed || step.error;

          return (
            <div
              key={step.id}
              className="relative"
            >
              {/* Connector line */}
              {idx < steps.length - 1 && (
                <div
                  className={cn(
                    'absolute left-4 top-10 h-full w-px transition-colors duration-300',
                    step.status === 'complete'
                      ? 'bg-foreground'
                      : step.status === 'error'
                        ? 'bg-destructive/70'
                        : 'bg-border'
                  )}
                />
              )}

              <div
                ref={isActive || isFocused ? activeStepRef : null}
                role="button"
                tabIndex={0}
                aria-pressed={isFocused || isActive}
                data-step-id={step.id}
                onClick={() => onStepSelect?.(step.id)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    onStepSelect?.(step.id);
                  }
                }}
                className={cn(
                  'relative flex items-start gap-3 rounded-2xl border-2 border-border bg-card/92 p-4 text-left shadow-[10px_10px_0_rgba(0,0,0,0.55)] transition-all duration-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-foreground focus-visible:ring-offset-2 focus-visible:ring-offset-background',
                  isActive && 'bg-accent/30',
                  isComplete && !isExpanded && 'opacity-80 hover:opacity-100',
                  isFocused && 'ring-2 ring-foreground ring-offset-2 ring-offset-background'
                )}
                aria-current={isFocused ? 'step' : undefined}
              >
                {/* Status icon */}
                <div className="flex-shrink-0 z-10">
                  <StepIcon status={step.status} />
                </div>

                {/* Content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1">
                      <div className="mb-2 flex items-center gap-2">
                        <h4 className="text-sm font-semibold uppercase tracking-[0.2em] text-foreground">{step.title}</h4>
                        <StepStatusBadge status={step.status} />
                      </div>
                      {step.description && (
                        <p className="console-microcopy text-foreground/75">
                          {step.description}
                        </p>
                      )}
                    </div>

                    {/* Expand button */}
                    {hasDetails && (
                      <button
                        onClick={() => toggleExpand(step.id)}
                        className="console-button console-button-ghost text-[11px] uppercase"
                        aria-label={
                          isExpanded
                            ? t('timeline.card.collapse')
                            : t('timeline.card.expand')
                        }
                        onClickCapture={(event) => event.stopPropagation()}
                        onKeyDownCapture={(event) => event.stopPropagation()}
                      >
                        {isExpanded ? (
                          <ChevronUp className="h-4 w-4" />
                        ) : (
                          <ChevronDown className="h-4 w-4" />
                        )}
                      </button>
                    )}
                  </div>

                  {/* Duration/timestamp */}
                  {step.duration !== undefined && (
                    <div className="mt-2 flex items-center gap-1 text-[11px] uppercase tracking-[0.24em] text-muted-foreground">
                      <Clock className="h-3 w-3" />
                      <span>{formatDuration(step.duration)}</span>
                    </div>
                  )}

                  {/* Expanded details */}
                  {isExpanded && hasDetails && (
                    <div className="mt-3 space-y-2 animate-fadeIn">
                      {/* Tools used */}
                      {step.toolsUsed && step.toolsUsed.length > 0 && (
                        <div>
                          <p className="console-microcopy font-semibold uppercase tracking-[0.28em] text-muted-foreground">
                            {t('timeline.card.toolsUsed')}
                          </p>
                          <div className="flex flex-wrap gap-2">
                            {step.toolsUsed.map((tool, toolIdx) => (
                              <span
                                key={toolIdx}
                                className="console-quiet-chip text-[10px] uppercase"
                              >
                                {tool}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Tokens used */}
                      {step.tokensUsed !== undefined && (
                        <div>
                          <p className="console-microcopy font-semibold uppercase tracking-[0.28em] text-muted-foreground">
                            {t('timeline.card.tokensUsed')}
                          </p>
                          <span className="console-quiet-chip text-[10px] uppercase">
                            {step.tokensUsed.toLocaleString()}
                          </span>
                        </div>
                      )}

                      {/* Error details */}
                      {step.error && (
                        <div>
                          <p className="console-microcopy font-semibold uppercase tracking-[0.28em] text-destructive">
                            {t('timeline.card.error')}
                          </p>
                          <div className="console-card bg-destructive/10 border-destructive/30 p-3 text-left shadow-none">
                            <pre className="console-microcopy whitespace-pre-wrap font-mono text-destructive">
                              {step.error}
                            </pre>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
}

function StepIcon({ status }: { status: StepStatus }) {
  const base = 'inline-flex h-10 w-10 items-center justify-center rounded-full border-2 border-border bg-card shadow-[4px_4px_0_rgba(0,0,0,0.45)]';

  switch (status) {
    case 'pending':
      return (
        <div className={base}>
          <Circle className={cn('h-5 w-5 text-muted-foreground')} />
        </div>
      );
    case 'active':
      return (
        <div className={cn(base, 'bg-accent/30 animate-pulse')}>
          <Loader2 className="h-5 w-5 animate-spin text-foreground" />
        </div>
      );
    case 'complete':
      return (
        <div className={cn(base, 'bg-foreground text-background')}>
          <CheckCircle2 className="h-5 w-5 text-background" />
        </div>
      );
    case 'error':
      return (
        <div className={cn(base, 'border-destructive bg-destructive/10 text-destructive')}>
          <XCircle className="h-5 w-5" />
        </div>
      );
  }
}

function StepStatusBadge({ status }: { status: StepStatus }) {
  const t = useTranslation();
  switch (status) {
    case 'pending':
      return <span className="console-quiet-chip text-[10px] uppercase">{t('timeline.card.badge.pending')}</span>;
    case 'active':
      return <span className="console-quiet-chip text-[10px] uppercase animate-pulse">{t('timeline.card.badge.active')}</span>;
    case 'complete':
      return <span className="console-quiet-chip text-[10px] uppercase">{t('timeline.card.badge.complete')}</span>;
    case 'error':
      return (
        <span className="console-quiet-chip border-destructive bg-destructive/10 text-[10px] uppercase text-destructive">
          {t('timeline.card.badge.error')}
        </span>
      );
  }
}
