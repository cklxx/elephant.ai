import { useMemo } from 'react';
import { TimelineStep, StepStatus } from '@/components/agent/ResearchTimeline';

export interface PlanProgressMetrics {
  totalSteps: number;
  completedSteps: number;
  completionRatio: number;
  activeStepId?: string | null;
  activeStepTitle?: string | null;
  latestCompletedStepId?: string | null;
  latestCompletedStepTitle?: string | null;
  totalDurationMs?: number;
  averageStepDurationMs?: number;
  totalTokensUsed?: number;
  uniqueToolsUsed: string[];
  hasErrors: boolean;
  stepStatuses: Record<string, StepStatus>;
}

export function usePlanProgress(steps: TimelineStep[]): PlanProgressMetrics | null {
  return useMemo(() => {
    if (!steps || steps.length === 0) {
      return null;
    }

    const totalSteps = steps.length;
    const completedSteps = steps.filter((step) => step.status === 'done').length;
    const completionRatio = totalSteps === 0 ? 0 : Math.min(1, Math.max(0, completedSteps / totalSteps));

    const activeStep = steps.find((step) => step.status === 'active') ?? null;
    const latestCompleted = [...steps]
      .filter((step) => step.status === 'done')
      .sort((a, b) => (b.endTime ?? 0) - (a.endTime ?? 0))[0] ?? null;

    const durationTotals = steps.reduce(
      (acc, step) => {
        if (typeof step.duration === 'number' && step.duration >= 0) {
          acc.totalDuration += step.duration;
          acc.durationCount += 1;
        }
        if (typeof step.tokensUsed === 'number' && step.tokensUsed > 0) {
          acc.totalTokens += step.tokensUsed;
        }
        if (Array.isArray(step.toolsUsed)) {
          step.toolsUsed.forEach((tool) => {
            if (tool) {
              acc.uniqueTools.add(tool);
            }
          });
        }
        if (step.status === 'failed') {
          acc.hasErrors = true;
        }
        return acc;
      },
      {
        totalDuration: 0,
        durationCount: 0,
        totalTokens: 0,
        uniqueTools: new Set<string>(),
        hasErrors: false,
      }
    );

    const averageStepDurationMs =
      durationTotals.durationCount > 0 ? Math.round(durationTotals.totalDuration / durationTotals.durationCount) : undefined;

    const uniqueToolsUsed = Array.from(durationTotals.uniqueTools).sort((a, b) => a.localeCompare(b));

    const stepStatuses = steps.reduce<Record<string, StepStatus>>((acc, step) => {
      acc[step.id] = step.status;
      return acc;
    }, {});

    return {
      totalSteps,
      completedSteps,
      completionRatio,
      activeStepId: activeStep?.id ?? null,
      activeStepTitle: activeStep?.title ?? null,
      latestCompletedStepId: latestCompleted?.id ?? null,
      latestCompletedStepTitle: latestCompleted?.title ?? null,
      totalDurationMs: durationTotals.totalDuration > 0 ? durationTotals.totalDuration : undefined,
      averageStepDurationMs,
      totalTokensUsed: durationTotals.totalTokens > 0 ? durationTotals.totalTokens : undefined,
      uniqueToolsUsed,
      hasErrors: durationTotals.hasErrors,
      stepStatuses,
    };
  }, [steps]);
}
