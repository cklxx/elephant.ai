'use client';

import { useEffect, useRef } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
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

  // Auto-scroll to active step
  useEffect(() => {
    if (activeStepRef.current) {
      activeStepRef.current.scrollIntoView({
        behavior: 'smooth',
        block: 'center',
      });
    }
  }, [steps]);

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
    <Card className={cn('glass-card p-6 shadow-medium', className)}>
      <div className="mb-4">
        <h3 className="text-lg font-semibold text-gray-900">{t('timeline.card.title')}</h3>
        <p className="console-microcopy mt-1">{t('timeline.card.subtitle')}</p>
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
                    'absolute left-4 top-10 w-0.5 h-full transition-colors duration-300',
                    step.status === 'complete'
                      ? 'bg-emerald-300'
                      : step.status === 'error'
                      ? 'bg-destructive/60'
                      : 'bg-primary/30'
                  )}
                />
              )}

              <div
                ref={isActive ? activeStepRef : null}
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
                  'relative flex items-start gap-3 rounded-xl p-4 text-left transition-all duration-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background',
                  isActive && 'bg-primary/10 border-2 border-primary/40 shadow-md animate-pulse-soft',
                  isComplete && !isExpanded && 'opacity-70 hover:opacity-100',
                  'hover:bg-muted',
                  isFocused && 'ring-2 ring-primary ring-offset-2 ring-offset-background'
                )}
              >
                {/* Status icon */}
                <div className="flex-shrink-0 z-10">
                  <StepIcon status={step.status} />
                </div>

                {/* Content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <h4 className="font-semibold text-gray-900">{step.title}</h4>
                        <StepStatusBadge status={step.status} />
                      </div>
                      {step.description && (
                        <p className="console-microcopy">
                          {step.description}
                        </p>
                      )}
                    </div>

                    {/* Expand button */}
                    {hasDetails && (
                      <button
                        onClick={() => toggleExpand(step.id)}
                        className="flex-shrink-0 p-1 rounded-lg hover:bg-muted transition-colors"
                        aria-label={
                          isExpanded
                            ? t('timeline.card.collapse')
                            : t('timeline.card.expand')
                        }
                        onClickCapture={(event) => event.stopPropagation()}
                        onKeyDownCapture={(event) => event.stopPropagation()}
                      >
                        {isExpanded ? (
                          <ChevronUp className="h-4 w-4 text-gray-500" />
                        ) : (
                          <ChevronDown className="h-4 w-4 text-gray-500" />
                        )}
                      </button>
                    )}
                  </div>

                  {/* Duration/timestamp */}
                  {step.duration !== undefined && (
                    <div className="flex items-center gap-1 mt-2 text-[11px] text-gray-400">
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
                          <p className="text-[11px] font-semibold uppercase tracking-wide text-gray-500 mb-1">
                            {t('timeline.card.toolsUsed')}
                          </p>
                          <div className="flex flex-wrap gap-1">
                            {step.toolsUsed.map((tool, toolIdx) => (
                              <span
                                key={toolIdx}
                                className="text-[11px] px-2 py-1 rounded border border-primary/20 bg-primary/5 text-primary"
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
                          <p className="text-[11px] font-semibold uppercase tracking-wide text-gray-500 mb-1">
                            {t('timeline.card.tokensUsed')}
                          </p>
                          <Badge variant="info" className="text-[11px] px-2 py-0.5">
                            {step.tokensUsed.toLocaleString()}
                          </Badge>
                        </div>
                      )}

                      {/* Error details */}
                      {step.error && (
                        <div>
                          <p className="text-[11px] font-semibold uppercase tracking-wide text-destructive mb-1">
                            {t('timeline.card.error')}
                          </p>
                          <div className="bg-destructive/5 border border-destructive/20 rounded-lg p-2">
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
  const iconClasses = 'h-8 w-8';

  switch (status) {
    case 'pending':
      return (
        <div className="p-1 rounded-full bg-muted">
          <Circle className={cn(iconClasses, 'text-muted-foreground')} />
        </div>
      );
    case 'active':
      return (
        <div className="p-1 rounded-full bg-primary/10 animate-pulse">
          <Loader2 className={cn(iconClasses, 'text-primary animate-spin')} />
        </div>
      );
    case 'complete':
      return (
        <div className="p-1 rounded-full bg-emerald-50">
          <CheckCircle2 className={cn(iconClasses, 'text-emerald-600')} />
        </div>
      );
    case 'error':
      return (
        <div className="p-1 rounded-full bg-destructive/10">
          <XCircle className={cn(iconClasses, 'text-destructive')} />
        </div>
      );
  }
}

function StepStatusBadge({ status }: { status: StepStatus }) {
  const t = useTranslation();
  switch (status) {
    case 'pending':
      return (
        <Badge variant="default" className="text-xs">
          {t('timeline.card.badge.pending')}
        </Badge>
      );
    case 'active':
      return (
        <Badge variant="info" className="text-xs animate-pulse-soft">
          {t('timeline.card.badge.active')}
        </Badge>
      );
    case 'complete':
      return (
        <Badge variant="success" className="text-xs">
          {t('timeline.card.badge.complete')}
        </Badge>
      );
    case 'error':
      return (
        <Badge variant="error" className="text-xs">
          {t('timeline.card.badge.error')}
        </Badge>
      );
  }
}
