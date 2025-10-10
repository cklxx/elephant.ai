'use client';

import { ResearchPlanCard } from '@/components/agent/ResearchPlanCard';
import { PlanProgressMetrics } from '@/hooks/usePlanProgress';

const demoPlan = {
  goal: 'Launch an insights dashboard for research runs',
  steps: [
    'Audit existing agent telemetry coverage',
    'Design metrics schema for operator-facing dashboard',
    'Prototype timeline drill-down interactions',
    'Polish UI and prepare handoff notes',
  ],
  estimated_tools: ['file_read', 'bash', 'browser'],
  estimated_iterations: 6,
  estimated_duration_minutes: 210,
};

const demoProgress: PlanProgressMetrics = {
  totalSteps: 4,
  completedSteps: 2,
  completionRatio: 0.5,
  activeStepTitle: 'Prototype timeline drill-down interactions',
  latestCompletedStepTitle: 'Design metrics schema for operator-facing dashboard',
  totalDurationMs: 1000 * 60 * 95,
  averageStepDurationMs: 1000 * 60 * 32,
  totalTokensUsed: 18400,
  uniqueToolsUsed: ['browser', 'file_read'],
  hasErrors: false,
};

export default function PlanPreviewPage() {
  return (
    <div className="min-h-screen bg-muted/40 p-6">
      <div className="mx-auto flex max-w-4xl flex-col gap-6">
        <ResearchPlanCard plan={demoPlan} progress={demoProgress} />
        <ResearchPlanCard plan={demoPlan} readonly progress={{ ...demoProgress, completedSteps: 4, completionRatio: 1, activeStepTitle: null, latestCompletedStepTitle: 'Polish UI and prepare handoff notes', totalDurationMs: 1000 * 60 * 205, hasErrors: false }} />
      </div>
    </div>
  );
}
