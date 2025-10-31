import type { Meta, StoryObj } from '@storybook/react';
import { ResearchPlanManager } from './ResearchPlanManager';
import { PlanProgressMetrics } from '@/hooks/usePlanProgress';

const demoPlan = {
  goal: 'Evaluate codebase for modularization opportunities',
  steps: [
    'List feature areas with high coupling',
    'Propose module boundaries based on product flows',
    'Draft migration checklist for each target module',
  ],
  estimated_tools: ['file_read', 'bash', 'analysis'],
  estimated_iterations: 5,
  estimated_duration_minutes: 180,
};

const demoProgress: PlanProgressMetrics = {
  totalSteps: 3,
  completedSteps: 1,
  completionRatio: 1 / 3,
  activeStepId: 'step-1',
  activeStepTitle: 'Propose module boundaries based on product flows',
  latestCompletedStepId: 'step-0',
  latestCompletedStepTitle: 'List feature areas with high coupling',
  totalDurationMs: 1000 * 60 * 54,
  averageStepDurationMs: 1000 * 60 * 27,
  totalTokensUsed: 8200,
  uniqueToolsUsed: ['analysis', 'browser'],
  hasErrors: false,
  stepStatuses: {
    'step-0': 'done',
    'step-1': 'active',
    'step-2': 'planned',
  },
};

const meta = {
  title: 'Agent/ResearchPlanManager',
  component: ResearchPlanManager,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
} satisfies Meta<typeof ResearchPlanManager>;

export default meta;

type Story = StoryObj<typeof meta>;

export const AwaitingApproval: Story = {
  args: {
    plan: demoPlan,
    progress: demoProgress,
    loading: false,
  },
};

export const Loading: Story = {
  args: {
    plan: demoPlan,
    loading: true,
  },
};

export const ReadonlySummary: Story = {
  args: {
    plan: demoPlan,
    progress: demoProgress,
    readonly: true,
  },
};
