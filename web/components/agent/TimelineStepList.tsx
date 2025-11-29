'use client';

import { ComponentType, useMemo } from 'react';
import { Badge } from '@/components/ui/badge';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { CheckCircle2, Clock, Loader2, XCircle } from 'lucide-react';
import { StepStatus, TimelineStep } from '@/lib/planTypes';

interface TimelineStepListProps {
  steps: TimelineStep[];
  focusedStepId?: string | null;
  onStepSelect?: (stepId: string) => void;
  className?: string;
}

type StatusMeta = {
  label: string;
  tone: 'muted' | 'info' | 'success' | 'danger';
  icon: ComponentType<{ className?: string }>;
};

export function TimelineStepList({
  steps,
  focusedStepId = null,
  onStepSelect,
  className,
}: TimelineStepListProps) {
  const t = useTranslation();

  const statusMeta = useMemo((): Record<StepStatus, StatusMeta> => ({
    planned: { label: t('plan.steps.status.planned'), tone: 'muted', icon: Clock },
    active: { label: t('plan.steps.status.active'), tone: 'info', icon: Loader2 },
    done: { label: t('plan.steps.status.done'), tone: 'success', icon: CheckCircle2 },
    failed: { label: t('plan.steps.status.failed'), tone: 'danger', icon: XCircle },
  }), [t]);

  if (!steps || steps.length === 0) {
    return null;
  }

  return (
    <div className={cn('flex flex-col gap-2', className)}>
      {steps.map((step, index) => {
        const meta = statusMeta[step.status];
        const Icon = meta.icon;
        const isFocused = focusedStepId === step.id;

        const content = (
          <div className="flex items-start gap-3">
            <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground text-sm font-semibold">
              {index + 1}
            </div>
            <div className="flex flex-1 flex-col gap-1">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm font-semibold text-foreground">{step.title}</p>
                <Badge variant={toneToVariant(meta.tone)} className="text-[10px] uppercase">
                  <Icon className={cn('mr-1 h-3 w-3', step.status === 'active' && 'animate-spin')} />
                  {meta.label}
                </Badge>
              </div>
              {step.description && <p className="text-sm text-muted-foreground">{step.description}</p>}
              {step.result && step.status === 'done' && (
                <p className="text-sm text-foreground/80">{step.result}</p>
              )}
              {step.error && step.status === 'failed' && (
                <p className="text-sm text-destructive">{step.error}</p>
              )}
            </div>
          </div>
        );

        const baseClasses = cn(
          'w-full rounded-xl border border-border/80 bg-background/80 p-4 text-left transition',
          isFocused && 'border-primary shadow-sm'
        );

        return (
          <div key={step.id}>
            {onStepSelect ? (
              <button
                type="button"
                onClick={() => onStepSelect(step.id)}
                className={cn(
                  baseClasses,
                  'hover:-translate-y-[2px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background'
                )}
              >
                {content}
              </button>
            ) : (
              <div className={cn(baseClasses, 'cursor-default')}>
                {content}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function toneToVariant(tone: StatusMeta['tone']): 'default' | 'info' | 'success' | 'error' {
  switch (tone) {
    case 'info':
      return 'info';
    case 'success':
      return 'success';
    case 'danger':
      return 'error';
    default:
      return 'default';
  }
}
