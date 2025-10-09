import type { Meta, StoryObj } from '@storybook/react';
import { ConnectionStatus } from './ConnectionStatus';
import { fn } from '@storybook/test';

const meta: Meta<typeof ConnectionStatus> = {
  title: 'Agent/ConnectionStatus',
  component: ConnectionStatus,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  args: {
    onReconnect: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof ConnectionStatus>;

export const Connected: Story = {
  args: {
    connected: true,
    reconnecting: false,
  },
};

export const Disconnected: Story = {
  args: {
    connected: false,
    reconnecting: false,
    error: 'Connection lost',
  },
};

export const Reconnecting: Story = {
  args: {
    connected: false,
    reconnecting: true,
  },
};

export const ReconnectingWithError: Story = {
  args: {
    connected: false,
    reconnecting: true,
    error: 'Connection failed. Retrying...',
  },
};

export const MaxRetriesReached: Story = {
  args: {
    connected: false,
    reconnecting: false,
    error: 'Max reconnection attempts reached',
  },
};
