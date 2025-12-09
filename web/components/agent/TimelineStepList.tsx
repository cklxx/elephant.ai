'use client';

import { ComponentType, useMemo } from 'react';
import { Badge, type BadgeProps } from '@/components/ui/badge';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { Check, Clock, Loader2, X, Circle } from 'lucide-react';
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
    planned: { label: t('plan.steps.status.planned'), tone: 'muted', icon: Circle },
    active: { label: t('plan.steps.status.active'), tone: 'info', icon: Loader2 },
    done: { label: t('plan.steps.status.done'), tone: 'success', icon: Check },
    failed: { label: t('plan.steps.status.failed'), tone: 'danger', icon: X },
  }), [t]);

  if (!steps || steps.length === 0) {
    return null;
  }

  return (
    <div className={cn('flex flex-col', className)}>
      {steps.map((step, index) => {
        const meta = statusMeta[step.status];
        const Icon = meta.icon;
        const isFocused = focusedStepId === step.id;

        const baseClasses = cn(
          'group relative flex gap-4 p-3 rounded-lg transition-all duration-200 border border-transparent',
          isFocused ? 'bg-background border-border/50 shadow-sm' : 'hover:bg-muted/30'
        );

        const content = (
          <>
            {/* Connector Line (visual only, simplified) */}
            <div className="flex flex-col items-center pt-1">
              <div className={cn(
                "flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold border",
                step.status === 'active' ? "border-primary bg-primary text-primary-foreground" :
                  step.status === 'done' ? "border-primary/50 bg-primary/10 text-primary" :
                    "border-muted-foreground/30 text-muted-foreground"
              )}>
                {step.status === 'done' ? <Check className="w-3 h-3" /> : (index + 1)}
              </div>
            </div>

            <div className="flex-1 min-w-0 space-y-1">
              <div className="flex items-center justify-between gap-2">
                <p className={cn(
                  "text-sm font-medium leading-none",
                  step.status === 'planned' && "text-muted-foreground",
                  step.status === 'active' && "text-foreground",
                  step.status === 'done' && "text-foreground/80 line-through decoration-muted-foreground/40"
                )}>
                  {step.title}
                </p>
                {step.status === 'active' && (
                  <span className="flex items-center gap-1.5 text-[10px] uppercase font-bold tracking-wider text-primary animate-pulse">
                    <Loader2 className="w-3 h-3 animate-spin" /> {meta.label}
                  </span>
                )}
              </div>

              {step.description && !step.result && step.status !== 'done' && (
                <p className="text-xs text-muted-foreground/80 line-clamp-2">{step.description}</p>
              )}

              {step.error && step.status === 'failed' && (
                <p className="text-xs text-destructive font-medium bg-destructive/10 p-1.5 rounded mt-1">
                  {step.error}
                </p>
              )}
            </div>
          </>
        );

        return (
          <div key={step.id}>
            {onStepSelect ? (
              <div
                onClick={() => onStepSelect(step.id)}
                className={cn(baseClasses, "cursor-pointer")}
              >
                {content}
              </div>
            ) : (
              <div className={baseClasses}>{content}</div>
            )}
          </div>
        );
      })}
    </div>
  );
}
