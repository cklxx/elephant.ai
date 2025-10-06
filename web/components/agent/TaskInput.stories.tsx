import type { Meta, StoryObj } from '@storybook/react';
import { TaskInput } from './TaskInput';
import { fn } from '@storybook/test';

const meta: Meta<typeof TaskInput> = {
  title: 'Agent/TaskInput',
  component: TaskInput,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    onSubmit: { action: 'submitted' },
  },
  args: {
    onSubmit: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof TaskInput>;

export const Default: Story = {
  args: {},
};

export const Loading: Story = {
  args: {
    loading: true,
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};

export const WithPlaceholder: Story = {
  args: {
    placeholder: 'Enter a custom task description...',
  },
};

export const LoadingWithText: Story = {
  args: {
    loading: true,
  },
  play: async ({ canvasElement }) => {
    const textarea = canvasElement.querySelector('textarea');
    if (textarea) {
      textarea.value = 'This task is being submitted...';
    }
  },
};
