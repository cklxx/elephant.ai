'use client';

import { ComponentType } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/lib/i18n';
import { cn, formatDuration } from '@/lib/utils';
import { PlanProgressMetrics } from '@/hooks/usePlanProgress';
import { Clock, Gauge, Cpu, Wrench, ArrowUpRight } from 'lucide-react';

interface PlanProgressSummaryProps {
  progress: PlanProgressMetrics;
  variant?: 'detailed' | 'compact';
  onNavigateToStep?: (stepId: string) => void;
  className?: string;
}

export function PlanProgressSummary({
  progress,
  variant = 'detailed',
  onNavigateToStep,
  className,
}: PlanProgressSummaryProps) {
  const t = useTranslation();
  const completionPercent = Math.round(progress.completionRatio * 100);

  const statusKey = progress.hasErrors
    ? 'plan.progress.status.error'
    : progress.completedSteps >= progress.totalSteps && progress.totalSteps > 0
    ? 'plan.progress.status.complete'
    : progress.completedSteps > 0
    ? 'plan.progress.status.inProgress'
    : 'plan.progress.status.pending';

  const statusVariant: 'error' | 'success' | 'info' | 'warning' = progress.hasErrors
    ? 'error'
    : progress.completedSteps >= progress.totalSteps && progress.totalSteps > 0
    ? 'success'
    : progress.completedSteps > 0
    ? 'info'
    : 'warning';

  const navigateTargetId = progress.activeStepId ?? progress.latestCompletedStepId ?? null;
  const navigateLabel = progress.activeStepTitle
    ? t('plan.progress.activeStep', { title: progress.activeStepTitle })
    : progress.latestCompletedStepTitle
    ? t('plan.progress.latestCompleted', { title: progress.latestCompletedStepTitle })
    : null;

  const summaryContent = (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.3em] text-muted-foreground">
            {t('plan.progress.heading')}
          </p>
          <p className="text-sm font-semibold text-foreground">
            {t('plan.progress.completed', {
              completed: progress.completedSteps,
              total: progress.totalSteps,
            })}
          </p>
        </div>
        <Badge variant={statusVariant}>{t(statusKey)}</Badge>
      </div>

      <div className="h-2 rounded-full bg-border/60">
        <div
          className={cn('h-2 rounded-full transition-all',
            statusVariant === 'error'
              ? 'bg-destructive'
              : statusVariant === 'success'
              ? 'bg-primary'
              : 'bg-foreground'
          )}
          style={{ width: `${completionPercent}%` }}
          aria-hidden="true"
        />
      </div>

      {navigateLabel && (
        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
          <span>{navigateLabel}</span>
          {onNavigateToStep && navigateTargetId && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-[11px] uppercase tracking-[0.2em] text-foreground"
              onClick={() => onNavigateToStep(navigateTargetId)}
            >
              {t('plan.progress.viewInTimeline')}
              <ArrowUpRight className="ml-1 h-3 w-3" />
            </Button>
          )}
        </div>
      )}
    </div>
  );

  if (variant === 'compact') {
    return <div className={cn('rounded-xl border border-border/60 bg-muted/30 p-4', className)}>{summaryContent}</div>;
  }

  return (
    <div className={cn('rounded-xl border border-border/60 bg-muted/30 p-4 space-y-3', className)}>
      {summaryContent}

      <div className="grid gap-3 sm:grid-cols-2">
        {progress.totalDurationMs !== undefined && (
          <ProgressMetric
            icon={Clock}
            label={t('plan.progress.totalDuration')}
            value={formatDuration(progress.totalDurationMs)}
          />
        )}
        {progress.averageStepDurationMs !== undefined && (
          <ProgressMetric
            icon={Gauge}
            label={t('plan.progress.averageDuration')}
            value={formatDuration(progress.averageStepDurationMs)}
          />
        )}
        {progress.totalTokensUsed !== undefined && (
          <ProgressMetric
            icon={Cpu}
            label={t('plan.progress.tokens')}
            value={progress.totalTokensUsed.toLocaleString()}
          />
        )}
        {progress.uniqueToolsUsed.length > 0 && (
          <div className="rounded-lg border border-border/50 bg-background/60 p-3 space-y-2">
            <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              <Wrench className="h-3.5 w-3.5" />
              <span>{t('plan.progress.uniqueTools')}</span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {progress.uniqueToolsUsed.map((tool) => (
                <Badge key={tool} variant="info" className="text-[11px] px-2 py-0.5">
                  {tool}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function ProgressMetric({
  icon: Icon,
  label,
  value,
}: {
  icon: ComponentType<{ className?: string }>;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-lg border border-border/50 bg-background/60 p-3 space-y-1">
      <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
        <Icon className="h-3.5 w-3.5" />
        <span>{label}</span>
      </div>
      <p className="text-sm font-semibold text-foreground">{value}</p>
    </div>
  );
}
