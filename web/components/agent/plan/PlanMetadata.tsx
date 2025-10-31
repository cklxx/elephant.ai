'use client';

import { useTranslation } from '@/lib/i18n';
import { ResearchPlan } from './types';
import { LucideIcon } from 'lucide-react';
import { Gauge, Clock, Wrench } from 'lucide-react';

export function PlanMetadata({ plan }: { plan: ResearchPlan }) {
  const t = useTranslation();
  const estimatedDuration = formatEstimatedDurationLabel(t, plan.estimated_duration_minutes);
  const hasEstimatedTools = plan.estimated_tools.length > 0;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <PlanEstimatePill
          icon={Gauge}
          label={t('plan.estimates.iterations', { count: plan.estimated_iterations })}
        />
        {estimatedDuration && <PlanEstimatePill icon={Clock} label={estimatedDuration} />}
        {hasEstimatedTools && (
          <PlanEstimatePill
            icon={Wrench}
            label={t('plan.estimates.tools', { count: plan.estimated_tools.length })}
          />
        )}
      </div>

      {hasEstimatedTools && (
        <div className="flex flex-wrap items-center gap-1.5">
          {plan.estimated_tools.slice(0, 5).map((tool, idx) => (
            <span
              key={`${tool}-${idx}`}
              className="text-xs px-2 py-1 rounded border border-border/60 bg-background/80 text-muted-foreground"
            >
              {tool}
            </span>
          ))}
          {plan.estimated_tools.length > 5 && (
            <span className="text-xs px-2 py-1 rounded border border-border/60 bg-background/60 text-muted-foreground/80">
              {t('plan.tools.more', { count: plan.estimated_tools.length - 5 })}
            </span>
          )}
        </div>
      )}
    </div>
  );
}

function PlanEstimatePill({ icon: Icon, label }: { icon: LucideIcon; label: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-border/60 bg-background/80 px-3 py-1 text-[11px] font-medium text-muted-foreground">
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{label}</span>
    </span>
  );
}

function formatEstimatedDurationLabel(
  t: ReturnType<typeof useTranslation>,
  minutes?: number,
) {
  if (minutes === undefined || minutes === null || Number.isNaN(minutes)) {
    return null;
  }

  if (minutes <= 0) {
    return null;
  }

  if (minutes < 60) {
    return t('plan.estimates.durationMinutes', { minutes: Math.round(minutes) });
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = Math.round(minutes % 60);

  if (remainingMinutes === 0) {
    return t('plan.estimates.durationHours', { hours });
  }

  if (hours > 0) {
    return t('plan.estimates.durationHoursMinutes', {
      hours,
      minutes: remainingMinutes,
    });
  }

  return t('plan.estimates.durationMinutes', { minutes: remainingMinutes });
}
