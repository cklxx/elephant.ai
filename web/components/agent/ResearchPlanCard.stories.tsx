import type { Meta, StoryObj } from '@storybook/react';
import { ResearchPlanCard } from './ResearchPlanCard';
import { fn } from '@storybook/test';

const meta = {
  title: 'Agent/ResearchPlanCard',
  component: ResearchPlanCard,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
  argTypes: {
    plan: { control: 'object' },
    loading: { control: 'boolean' },
    readonly: { control: 'boolean' },
  },
  args: {
    onApprove: fn(),
    onModify: fn(),
    onReject: fn(),
  },
} satisfies Meta<typeof ResearchPlanCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    plan: {
      goal: 'Build a React component library with TypeScript',
      steps: [
        'Analyze requirements and design component API',
        'Set up TypeScript project with build tools',
        'Implement core components (Button, Input, Card)',
        'Add unit tests and Storybook documentation',
        'Package and publish to npm',
      ],
      estimated_tools: ['file_write', 'bash', 'npm'],
      estimated_iterations: 8,
      estimated_duration_minutes: 180,
    },
    loading: false,
    readonly: false,
  },
};

export const Loading: Story = {
  args: {
    plan: null,
    loading: true,
  },
};

export const ReadonlyApproved: Story = {
  args: {
    plan: {
      goal: 'Refactor authentication system',
      steps: [
        'Review current auth implementation',
        'Design new JWT-based auth flow',
        'Implement new auth middleware',
        'Update all protected routes',
        'Add comprehensive tests',
      ],
      estimated_tools: ['file_read', 'file_edit', 'bash'],
      estimated_iterations: 6,
      estimated_duration_minutes: 240,
    },
    readonly: true,
    progress: {
      totalSteps: 5,
      completedSteps: 5,
      completionRatio: 1,
      activeStepTitle: null,
      latestCompletedStepTitle: 'Add comprehensive tests',
      totalDurationMs: 1000 * 60 * 18,
      averageStepDurationMs: 1000 * 60 * 4,
      totalTokensUsed: 4200,
      uniqueToolsUsed: ['auth_audit', 'file_edit', 'bash'],
      hasErrors: false,
    },
  },
};

export const SimplePlan: Story = {
  args: {
    plan: {
      goal: 'Fix bug in user profile page',
      steps: [
        'Identify the bug in profile rendering',
        'Apply fix to profile component',
        'Test the fix manually',
      ],
      estimated_tools: ['file_read', 'file_edit'],
      estimated_iterations: 3,
      estimated_duration_minutes: 90,
    },
  },
};

export const ComplexPlan: Story = {
  args: {
    plan: {
      goal: 'Migrate entire application from JavaScript to TypeScript',
      steps: [
        'Set up TypeScript configuration and build pipeline',
        'Install type definitions for all dependencies',
        'Convert utility functions and helpers',
        'Convert React components one by one',
        'Convert Redux store and actions',
        'Convert API client and services',
        'Convert test files to TypeScript',
        'Fix all type errors and strictness issues',
        'Update documentation and README',
        'Run full test suite and fix failures',
      ],
      estimated_tools: ['file_read', 'file_write', 'file_edit', 'bash', 'grep'],
      estimated_iterations: 25,
      estimated_duration_minutes: 720,
    },
  },
};

export const EmptyPlan: Story = {
  args: {
    plan: null,
    loading: false,
  },
};

export const WithEditActions: Story = {
  args: {
    plan: {
      goal: 'Optimize database queries',
      steps: [
        'Profile slow queries',
        'Add database indexes',
        'Refactor N+1 queries',
      ],
      estimated_tools: ['bash', 'file_edit'],
      estimated_iterations: 4,
      estimated_duration_minutes: 150,
    },
    readonly: false,
  },
  play: async ({ canvasElement }) => {
    // Could add interaction testing here
  },
};

export const Collapsed: Story = {
  args: {
    plan: {
      goal: 'Update dependencies to latest versions',
      steps: [
        'Run npm outdated to check versions',
        'Update package.json carefully',
        'Run tests to ensure compatibility',
      ],
      estimated_tools: ['bash', 'file_edit'],
      estimated_iterations: 3,
      estimated_duration_minutes: 120,
    },
  },
  play: async ({ canvasElement }) => {
    // Simulate collapse
    const collapseButton = canvasElement.querySelector('button[aria-label*="Collapse"]');
    if (collapseButton instanceof HTMLElement) {
      collapseButton.click();
    }
  },
};
