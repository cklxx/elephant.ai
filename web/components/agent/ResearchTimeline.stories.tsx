import type { Meta, StoryObj } from '@storybook/react';
import { ResearchTimeline } from './ResearchTimeline';

const meta = {
  title: 'Agent/ResearchTimeline',
  component: ResearchTimeline,
  parameters: {
    layout: 'padded',
  },
  tags: ['autodocs'],
} satisfies Meta<typeof ResearchTimeline>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    steps: [],
  },
};

export const SingleStepActive: Story = {
  args: {
    steps: [
      {
        id: 'step-1',
        title: 'Step 1: Analyze Code',
        description: 'Reading and analyzing the codebase structure',
        status: 'active',
        startTime: Date.now() - 30000,
      },
    ],
  },
};

export const MultipleStepsProgress: Story = {
  args: {
    steps: [
      {
        id: 'step-1',
        title: 'Step 1: Analyze Code',
        description: 'Reading and analyzing the codebase structure',
        status: 'done',
        startTime: Date.now() - 120000,
        endTime: Date.now() - 90000,
        duration: 30000,
        toolsUsed: ['file_read', 'grep'],
      },
      {
        id: 'step-2',
        title: 'Step 2: Design Solution',
        description: 'Planning the implementation approach',
        status: 'done',
        startTime: Date.now() - 90000,
        endTime: Date.now() - 60000,
        duration: 30000,
      },
      {
        id: 'step-3',
        title: 'Step 3: Implement Changes',
        description: 'Writing code and making modifications',
        status: 'active',
        startTime: Date.now() - 60000,
        toolsUsed: ['file_write', 'file_edit'],
      },
      {
        id: 'step-4',
        title: 'Step 4: Test Solution',
        description: 'Running tests and verifying functionality',
        status: 'planned',
      },
    ],
  },
};

export const WithErrors: Story = {
  args: {
    steps: [
      {
        id: 'step-1',
        title: 'Step 1: Read File',
        status: 'done',
        startTime: Date.now() - 180000,
        endTime: Date.now() - 150000,
        duration: 30000,
      },
      {
        id: 'step-2',
        title: 'Step 2: Process Data',
        status: 'failed',
        error: 'Failed to parse JSON: Unexpected token < in JSON at position 0',
        startTime: Date.now() - 150000,
        endTime: Date.now() - 120000,
        duration: 30000,
      },
      {
        id: 'step-3',
        title: 'Step 3: Save Results',
        status: 'planned',
      },
    ],
  },
};

export const AllCompleted: Story = {
  args: {
    steps: [
      {
        id: 'step-1',
        title: 'Iteration 1/3',
        status: 'done',
        startTime: Date.now() - 180000,
        endTime: Date.now() - 120000,
        duration: 60000,
        toolsUsed: ['bash', 'file_read'],
        tokensUsed: 450,
      },
      {
        id: 'step-2',
        title: 'Iteration 2/3',
        status: 'done',
        startTime: Date.now() - 120000,
        endTime: Date.now() - 60000,
        duration: 60000,
        toolsUsed: ['file_write', 'bash'],
        tokensUsed: 520,
      },
      {
        id: 'step-3',
        title: 'Iteration 3/3',
        status: 'done',
        startTime: Date.now() - 60000,
        endTime: Date.now(),
        duration: 60000,
        toolsUsed: ['bash'],
        tokensUsed: 380,
      },
    ],
  },
};

export const LongRunningStep: Story = {
  args: {
    steps: [
      {
        id: 'step-1',
        title: 'Step 1: Large File Analysis',
        description: 'Analyzing 10,000+ files in the monorepo',
        status: 'active',
        startTime: Date.now() - 600000, // 10 minutes ago
        toolsUsed: ['grep', 'file_read', 'bash'],
      },
    ],
  },
};

export const ManySteps: Story = {
  args: {
    steps: Array.from({ length: 20 }, (_, i) => ({
      id: `iter-${i + 1}`,
      title: `Iteration ${i + 1}/20`,
      description: i < 15 ? `Completed iteration ${i + 1}` : i === 15 ? 'Currently processing...' : undefined,
      status: (i < 15 ? 'done' : i === 15 ? 'active' : 'planned') as 'done' | 'active' | 'planned',
      startTime: i < 16 ? Date.now() - (20 - i) * 30000 : undefined,
      endTime: i < 15 ? Date.now() - (20 - i - 1) * 30000 : undefined,
      duration: i < 15 ? 30000 : undefined,
      toolsUsed: i < 16 ? ['bash', 'file_read'] : undefined,
      tokensUsed: i < 15 ? 300 + Math.random() * 200 : undefined,
    })),
  },
};

export const WithDetailedTools: Story = {
  args: {
    steps: [
      {
        id: 'step-1',
        title: 'Step 1: Setup',
        status: 'done',
        startTime: Date.now() - 120000,
        endTime: Date.now() - 90000,
        duration: 30000,
        toolsUsed: ['bash', 'file_write', 'npm'],
        tokensUsed: 450,
      },
      {
        id: 'step-2',
        title: 'Step 2: Implementation',
        description: 'Writing core functionality',
        status: 'active',
        startTime: Date.now() - 90000,
        toolsUsed: ['file_write', 'file_edit', 'grep', 'bash'],
      },
    ],
  },
};
