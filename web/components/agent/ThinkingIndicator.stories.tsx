import type { Meta, StoryObj } from '@storybook/react';
import { ThinkingIndicator } from './ThinkingIndicator';

const meta: Meta<typeof ThinkingIndicator> = {
  title: 'Agent/ThinkingIndicator',
  component: ThinkingIndicator,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof ThinkingIndicator>;

export const Default: Story = {
  args: {},
};

export const InCard: Story = {
  args: {},
  decorators: [
    (Story) => (
      <div className="p-4 border rounded-lg bg-white">
        <Story />
      </div>
    ),
  ],
};

export const OnDarkBackground: Story = {
  args: {},
  decorators: [
    (Story) => (
      <div className="p-8 bg-gray-900 rounded-lg">
        <Story />
      </div>
    ),
  ],
};
