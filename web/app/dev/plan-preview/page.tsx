'use client';

import { TimelineStepList } from '@/components/agent/TimelineStepList';
import { TimelineStep } from '@/lib/planTypes';

const demoTimelineSteps: TimelineStep[] = [
  {
    id: 'step-0',
    title: 'Audit existing agent telemetry coverage',
    description: 'Catalog current instrumentation and identify gaps.',
    status: 'done',
  },
  {
    id: 'step-1',
    title: 'Design metrics schema',
    description: 'Define operator-facing dashboard metrics and sources.',
    status: 'done',
  },
  {
    id: 'step-2',
    title: 'Prototype drill-down interactions',
    description: 'Implement exploratory UI interactions for runs.',
    status: 'active',
  },
  {
    id: 'step-3',
    title: 'Polish UI and prepare handoff',
    description: 'Finalize visuals and write handoff notes.',
    status: 'planned',
  },
];

export default function PlanPreviewPage() {
  return (
    <div className="min-h-screen bg-muted/40 p-6">
      <div className="mx-auto flex max-w-4xl flex-col gap-6">
        <div className="rounded-2xl border bg-card/80 p-6 space-y-4">
          <h2 className="text-lg font-semibold text-foreground">Timeline preview</h2>
          <p className="text-sm text-muted-foreground">
            Quick view of research steps without the plan approval UI.
          </p>
          <TimelineStepList steps={demoTimelineSteps} />
        </div>
      </div>
    </div>
  );
}
