import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

import { ConversationPageContent } from '../conversation/ConversationPageContent';
import { useSessionStore } from '@/hooks/useSessionStore';
import { APIError } from '@/lib/api';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock('@/hooks/useTaskExecution', () => ({
  useTaskExecution: vi.fn(),
}));

vi.mock('@/hooks/useAgentEventStream', () => ({
  useAgentEventStream: vi.fn(),
}));

vi.mock('@/hooks/useTimelineSteps', () => ({
  useTimelineSteps: vi.fn(() => []),
}));

vi.mock('@/components/agent/TerminalOutput', () => ({
  TerminalOutput: () => <div data-testid="terminal-output" />,
}));

vi.mock('@/components/ui/toast', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/components/ui/dialog', () => ({
  useConfirmDialog: () => ({
    confirm: vi.fn().mockResolvedValue(true),
    ConfirmDialog: () => <div data-testid="confirm-dialog" />,
  }),
}));

vi.mock('@/lib/eventAggregation', () => ({
  buildToolCallSummaries: () => [],
}));

const translationMock: Record<string, string> = {
  'console.input.placeholder.idle': 'Type your task…',
  'console.input.placeholder.active': 'Type your task…',
  'console.input.hotkeyHint': 'Enter to send, Shift+Enter for new line',
  'task.input.ariaLabel': 'Task input',
  'task.submit.title.default': 'Send task',
  'task.submit.title.running': 'Stop task',
  'task.submit.running': 'Stop',
  'task.submit.label': 'Send',
  'inputBar.actions.send': 'Send message',
  'sidebar.toggle.open': 'Open session list',
  'sidebar.toggle.close': 'Close session list',
};

vi.mock('@/lib/i18n', () => ({
  useI18n: () => ({
    t: (key: string) => translationMock[key] ?? key,
  }),
  useTranslation: () => (key: string) => translationMock[key] ?? key,
}));

vi.mock('@/components/layout', async () => {
  const ReactModule = await vi.importActual<typeof import('react')>('react');
  const actual = await vi.importActual<typeof import('@/components/layout')>(
    '@/components/layout'
  );

  return {
    ...actual,
    Sidebar: () => <div data-testid="sidebar" />,
    Header: ({ leadingSlot }: { leadingSlot?: React.ReactNode }) => (
      <div data-testid="header">{leadingSlot}</div>
    ),
    ContentArea: ReactModule.forwardRef<HTMLDivElement, { children: React.ReactNode }>(
      ({ children }, ref) => (
        <div data-testid="content-area" ref={ref}>
          {children}
        </div>
      )
    ),
  };
});

vi.mock('@/hooks/useSessionStore', async () => {
  const actual = await vi.importActual<typeof import('@/hooks/useSessionStore')>(
    '@/hooks/useSessionStore'
  );

  return {
    ...actual,
    useDeleteSession: () => ({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
    }),
  };
});

const useTaskExecutionMock = vi.mocked(useTaskExecution);
const useAgentEventStreamMock = vi.mocked(useAgentEventStream);

describe('ConversationPageContent - stale session handling', () => {
  const defaultStoreState = useSessionStore.getState();

  beforeEach(() => {
    vi.clearAllMocks();

    useAgentEventStreamMock.mockReturnValue({
      events: [],
      isConnected: true,
      isReconnecting: false,
      error: null,
      reconnectAttempts: 0,
      clearEvents: vi.fn(),
      reconnect: vi.fn(),
      addEvent: vi.fn(),
    });

    useSessionStore.setState({
      currentSessionId: 'stale-session',
      sessionHistory: ['stale-session'],
      pinnedSessions: [],
      sessionLabels: {},
    });

    useSessionStore.persist?.clearStorage?.();
  });

  afterEach(() => {
    useSessionStore.setState(defaultStoreState, true);
  });

  it('retries task creation without the stale session after a 404', async () => {
    const mutate = vi.fn();
    const addEvent = vi.fn();

    useAgentEventStreamMock.mockReturnValue({
      events: [],
      isConnected: true,
      isReconnecting: false,
      error: null,
      reconnectAttempts: 0,
      clearEvents: vi.fn(),
      reconnect: vi.fn(),
      addEvent,
    });

    let callCount = 0;
    mutate.mockImplementation(
      (
        variables: { task: string; session_id?: string },
        options?: {
          onSuccess?: (response: {
            session_id: string;
            task_id: string;
            status: 'pending';
          }) => void;
          onError?: (error: unknown) => void;
        }
      ) => {
        if (callCount === 0) {
          callCount += 1;
          expect(variables.session_id).toBe('stale-session');
          options?.onError?.(new APIError(404, 'Not Found', 'session not found'));
          return;
        }

        callCount += 1;
        expect(variables.session_id).toBeUndefined();
        options?.onSuccess?.({
          session_id: 'new-session',
          task_id: 'new-task',
          status: 'pending',
        });
      }
    );

    useTaskExecutionMock.mockReturnValue({
      mutate,
      isPending: false,
    } as unknown as ReturnType<typeof useTaskExecution>);

    const store = useSessionStore.getState();
    const removeSessionSpy = vi.spyOn(store, 'removeSession');
    const clearCurrentSessionSpy = vi.spyOn(store, 'clearCurrentSession');
    const setCurrentSessionSpy = vi.spyOn(store, 'setCurrentSession');
    const addToHistorySpy = vi.spyOn(store, 'addToHistory');

    render(<ConversationPageContent />);

    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: 'Fix the stale session bug' } });

    const submitButton = screen.getByRole('button', {
      name: /send/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(2));

    expect(removeSessionSpy).toHaveBeenCalledWith('stale-session');
    expect(clearCurrentSessionSpy).toHaveBeenCalledTimes(1);
    expect(setCurrentSessionSpy).toHaveBeenCalledWith('new-session');
    expect(addToHistorySpy).toHaveBeenCalledWith('new-session');

    const state = useSessionStore.getState();
    expect(state.currentSessionId).toBe('new-session');
    expect(state.sessionHistory[0]).toBe('new-session');
  });
});

