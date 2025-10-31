'use client';

import { ComponentType, useMemo, useState } from 'react';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { CheckCircle2, Clock, Loader2, XCircle, ChevronUp, ChevronDown, ArrowUpRight } from 'lucide-react';
import { PlanProgressMetrics } from '@/hooks/usePlanProgress';
import { PlanProgressSummary } from './plan/PlanProgressSummary';
import { PlanMetadata } from './plan/PlanMetadata';
import { ResearchPlan } from './plan/types';

interface ResearchPlanCardProps {
  plan: ResearchPlan | null;
  loading?: boolean;
  progress?: PlanProgressMetrics | null;
  onStepFocus?: (stepId: string) => void;
  focusedStepId?: string | null;
  className?: string;
}

type StepStatus = 'planned' | 'active' | 'done' | 'failed';

type StatusMeta = {
  label: string;
  tone: 'muted' | 'info' | 'success' | 'danger';
  icon: ComponentType<{ className?: string }>;
};

export function ResearchPlanCard({
  plan,
  loading = false,
  progress = null,
  onStepFocus,
  focusedStepId = null,
  className,
}: ResearchPlanCardProps) {
  const t = useTranslation();
  const [isExpanded, setIsExpanded] = useState(true);

  const statusMeta = useMemo((): Record<StepStatus, StatusMeta> => ({
    planned: { label: t('plan.steps.status.planned'), tone: 'muted', icon: Clock },
    active: { label: t('plan.steps.status.active'), tone: 'info', icon: Loader2 },
    done: { label: t('plan.steps.status.done'), tone: 'success', icon: CheckCircle2 },
    failed: { label: t('plan.steps.status.failed'), tone: 'danger', icon: XCircle },
  }), [t]);

  if (loading) {
    return <ResearchPlanSkeleton className={className} />;
  }

  if (!plan) {
    return null;
  }

  const stepStatuses = progress?.stepStatuses ?? {};

  return (
    <Card className={cn('console-card border-l-4 border-primary animate-fadeIn overflow-hidden', className)}>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h3 className="console-heading text-lg">{t('plan.title')}</h3>
            <p className="console-caption">{t('plan.caption.readonly')}</p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-muted-foreground"
            onClick={() => setIsExpanded((prev) => !prev)}
            aria-label={isExpanded ? t('plan.collapse') : t('plan.expand')}
          >
            {isExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          </Button>
        </div>
      </CardHeader>

      {isExpanded && (
        <CardContent className="space-y-5">
          {progress && (
            <PlanProgressSummary
              progress={progress}
              onNavigateToStep={(stepId) => onStepFocus?.(stepId)}
            />
          )}

          <section className="space-y-2">
            <h4 className="console-subheading text-sm">{t('plan.goal.label')}</h4>
            <div className="console-card p-4">
              <p className="console-body text-sm leading-relaxed">{plan.goal}</p>
            </div>
          </section>

          <section className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="console-subheading text-sm">
                {t('plan.steps.label', { count: plan.steps.length })}
              </h4>
              {onStepFocus && progress?.activeStepId && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-[11px] uppercase tracking-[0.2em]"
                  onClick={() => onStepFocus(progress.activeStepId!)}
                >
                  {t('plan.steps.focusActive')}
                  <ArrowUpRight className="ml-1 h-3 w-3" />
                </Button>
              )}
            </div>
            <div className="space-y-2">
              {plan.steps.map((step, index) => {
                const stepId = `step-${index}`;
                const status = stepStatuses[stepId] ?? 'planned';
                const meta = statusMeta[status];
                const Icon = meta.icon;
                const isFocused = focusedStepId === stepId;

                return (
                  <div
                    key={stepId}
                    className={cn(
                      'flex items-start gap-3 rounded-xl border border-border/60 bg-background/80 p-3 transition-colors',
                      isFocused && 'border-primary bg-primary/5 shadow-sm'
                    )}
                  >
                    <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground text-xs font-semibold">
                      {index + 1}
                    </div>
                    <div className="flex-1 space-y-1">
                      <div className="flex items-center justify-between gap-2">
                        <p className="text-sm font-semibold text-foreground">{step}</p>
                        <Badge variant={toneToVariant(meta.tone)} className="text-[10px] uppercase">
                          <Icon className={cn('mr-1 h-3 w-3', status === 'active' && 'animate-spin')} />
                          {meta.label}
                        </Badge>
                      </div>
                      {onStepFocus && (
                        <button
                          type="button"
                          className="text-[11px] uppercase tracking-[0.2em] text-muted-foreground hover:text-foreground"
                          onClick={() => onStepFocus(stepId)}
                          aria-label={t('plan.steps.focus', { index: index + 1 })}
                        >
                          {t('plan.steps.focusLabel')}
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </section>

          <PlanMetadata plan={plan} />
        </CardContent>
      )}
    </Card>
  );
}

function toneToVariant(tone: StatusMeta['tone']): 'secondary' | 'info' | 'success' | 'destructive' {
  switch (tone) {
    case 'info':
      return 'info';
    case 'success':
      return 'success';
    case 'danger':
      return 'destructive';
    default:
      return 'secondary';
  }
}

function ResearchPlanSkeleton({ className }: { className?: string }) {
  return (
    <Card className={cn('console-card border-l-4 border-primary animate-fadeIn overflow-hidden', className)}>
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 w-10 rounded-md" />
          <div className="space-y-2">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-40" />
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <Skeleton className="h-4 w-16" />
        <Skeleton className="h-20 w-full" />
        <div className="space-y-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
        <Skeleton className="h-6 w-48" />
      </CardContent>
    </Card>
  );
}
