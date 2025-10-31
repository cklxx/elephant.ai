'use client';

import { useMemo } from 'react';
import { Card } from '@/components/ui/card';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { CheckCircle2, Clock, Loader2, XCircle } from 'lucide-react';
import { usePlanProgress } from '@/hooks/usePlanProgress';
import { PlanProgressSummary } from './plan/PlanProgressSummary';

export type StepStatus = 'planned' | 'active' | 'done' | 'failed';

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
  result?: string;
  error?: string;
  anchorEventIndex?: number;
}

interface ResearchTimelineProps {
  steps: TimelineStep[];
  className?: string;
  focusedStepId?: string | null;
  onStepSelect?: (stepId: string) => void;
}

export function ResearchTimeline({ steps, className, focusedStepId, onStepSelect }: ResearchTimelineProps) {
  const t = useTranslation();

  const { planned, active, done, failed } = useMemo(() => {
    const grouped = {
      planned: [] as TimelineStep[],
      active: [] as TimelineStep[],
      done: [] as TimelineStep[],
      failed: [] as TimelineStep[],
    };

    steps.forEach((step) => {
      grouped[step.status].push(step);
    });
    return {
      planned: grouped.planned,
      active: grouped.active,
      done: grouped.done,
      failed: grouped.failed,
    };
  }, [steps]);

  const planProgress = usePlanProgress(steps);

  return (
    <Card className={cn('console-card px-6 py-6', className)}>
      <header className="mb-6 space-y-3">
        <h3 className="text-base font-semibold uppercase tracking-[0.22em] text-foreground">
          {t('timeline.card.title')}
        </h3>
        <p className="console-microcopy text-muted-foreground">{t('timeline.card.subtitle')}</p>
        {planProgress && (
          <PlanProgressSummary
            progress={planProgress}
            variant="compact"
            onNavigateToStep={onStepSelect}
            className="mt-4"
          />
        )}
      </header>

      <div className="grid gap-4 lg:grid-cols-3">
        <StepColumn
          title={t('timeline.column.plan')}
          steps={planned}
          tone="muted"
          emptyLabel={t('timeline.column.planEmpty')}
          focusedStepId={focusedStepId}
          onStepSelect={onStepSelect}
        />
        <StepColumn
          title={t('timeline.column.inProgress')}
          steps={active}
          tone="active"
          emptyLabel={t('timeline.column.inProgressEmpty')}
          focusedStepId={focusedStepId}
          onStepSelect={onStepSelect}
        />
        <StepColumn
          title={t('timeline.column.history')}
          steps={[...done, ...failed]}
          tone="history"
          emptyLabel={t('timeline.column.historyEmpty')}
          focusedStepId={focusedStepId}
          onStepSelect={onStepSelect}
        />
      </div>
    </Card>
  );
}

interface StepColumnProps {
  title: string;
  steps: TimelineStep[];
  tone: 'muted' | 'active' | 'history';
  emptyLabel: string;
  focusedStepId?: string | null;
  onStepSelect?: (stepId: string) => void;
}

function StepColumn({ title, steps, tone, emptyLabel, focusedStepId, onStepSelect }: StepColumnProps) {
  return (
    <section className="space-y-3 rounded-2xl border border-border bg-card/90 p-4 shadow-[8px_8px_0_rgba(0,0,0,0.5)]">
      <h4 className="text-xs font-semibold uppercase tracking-[0.3em] text-muted-foreground">{title}</h4>
      {steps.length === 0 ? (
        <p className="console-microcopy text-muted-foreground/70">{emptyLabel}</p>
      ) : (
        <ul className="space-y-3">
          {steps.map((step) => (
            <StepItem
              key={step.id}
              step={step}
              tone={tone}
              isFocused={focusedStepId === step.id}
              onSelect={onStepSelect}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

function StepItem({
  step,
  tone,
  isFocused,
  onSelect,
}: {
  step: TimelineStep;
  tone: 'muted' | 'active' | 'history';
  isFocused: boolean;
  onSelect?: (stepId: string) => void;
}) {
  const meta = STEP_META[step.status];
  const Icon = meta.icon;
  return (
    <li>
      <button
        type="button"
        onClick={() => onSelect?.(step.id)}
        className={cn(
          'w-full rounded-xl border-2 border-border bg-background/70 p-3 text-left transition hover:-translate-y-1 hover:-translate-x-1 hover:shadow-[6px_6px_0_rgba(0,0,0,0.55)] focus:outline-none focus-visible:ring-2 focus-visible:ring-foreground focus-visible:ring-offset-2 focus-visible:ring-offset-background',
          isFocused && 'ring-2 ring-foreground ring-offset-2 ring-offset-background',
          tone === 'active' && 'border-foreground/60 bg-accent/20',
          tone === 'history' && 'opacity-90 hover:opacity-100',
        )}
      >
        <div className="flex items-start gap-3">
          <span className={cn('mt-0.5 rounded-full border border-border p-1', meta.className)}>
            <Icon className="h-3.5 w-3.5" />
          </span>
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <p className="text-sm font-semibold uppercase tracking-[0.2em] text-foreground">{step.title}</p>
              <span className="text-[10px] font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                {meta.label}
              </span>
            </div>
            {step.description && <p className="console-microcopy text-muted-foreground/80">{step.description}</p>}
            {step.result && step.status === 'done' && (
              <p className="console-microcopy text-foreground/80">{step.result}</p>
            )}
            {step.error && step.status === 'failed' && (
              <p className="console-microcopy text-destructive">{step.error}</p>
            )}
          </div>
        </div>
      </button>
    </li>
  );
}

const STEP_META: Record<StepStatus, { icon: typeof Loader2; className: string; label: string }> = {
  planned: {
    icon: Clock,
    className: 'bg-background text-muted-foreground',
    label: 'Planned',
  },
  active: {
    icon: Loader2,
    className: 'bg-foreground text-background animate-spin',
    label: 'Active',
  },
  done: {
    icon: CheckCircle2,
    className: 'bg-foreground text-background',
    label: 'Done',
  },
  failed: {
    icon: XCircle,
    className: 'bg-destructive text-background',
    label: 'Failed',
  },
};
