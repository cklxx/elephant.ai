import type { Meta, StoryObj } from '@storybook/react';
import { ResearchPlanCard } from './ResearchPlanCard';
import { PlanProgressMetrics } from '@/hooks/usePlanProgress';

const basePlan = {
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

const baseProgress: PlanProgressMetrics = {
  totalSteps: 4,
  completedSteps: 2,
  completionRatio: 0.5,
  activeStepId: 'step-2',
  activeStepTitle: 'Prototype timeline drill-down interactions',
  latestCompletedStepId: 'step-1',
  latestCompletedStepTitle: 'Design metrics schema for operator-facing dashboard',
  totalDurationMs: 1000 * 60 * 95,
  averageStepDurationMs: 1000 * 60 * 32,
  totalTokensUsed: 18400,
  uniqueToolsUsed: ['browser', 'file_read'],
  hasErrors: false,
  stepStatuses: {
    'step-0': 'done',
    'step-1': 'done',
    'step-2': 'active',
    'step-3': 'planned',
  },
};

const meta = {
  title: 'Agent/ResearchPlanCard',
  component: ResearchPlanCard,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
} satisfies Meta<typeof ResearchPlanCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    plan: basePlan,
    progress: baseProgress,
  },
};

export const Loading: Story = {
  args: {
    plan: null,
    loading: true,
  },
};

export const WithoutProgress: Story = {
  args: {
    plan: basePlan,
    progress: null,
  },
};

export const FocusedStep: Story = {
  args: {
    plan: basePlan,
    progress: baseProgress,
    focusedStepId: 'step-2',
  },
};
