import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, beforeAll, afterAll, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactElement } from 'react';
import { LanguageProvider } from '@/lib/i18n';
import { ConversationPageContent } from '../conversation/ConversationPageContent';
import type { AnyAgentEvent } from '@/lib/types';

vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    prefetch: vi.fn(),
  }),
  usePathname: () => '/conversation',
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock('@/lib/api', () => ({
  apiClient: {
    createSession: vi.fn().mockResolvedValue({ session_id: 'prewarm-session' }),
  },
}));

const streamPropsRef: { current: { isRunning: boolean } | null } = {
  current: null,
};

vi.mock('@/components/agent/ConversationEventStream', () => ({
  ConversationEventStream: (props: { isRunning: boolean }) => {
    streamPropsRef.current = { isRunning: props.isRunning };
    return (
      <div data-testid="stream-running">{String(props.isRunning)}</div>
    );
  },
}));

vi.mock('@/hooks/useTaskExecution', () => ({
  useTaskExecution: () => ({
    isPending: false,
    mutate: (
      _payload: unknown,
      opts: { onSuccess?: (data: any) => void },
    ) => {
      opts.onSuccess?.({
        session_id: 'test-session',
        run_id: 'task-1',
        parent_run_id: null,
      });
    },
  }),
  useCancelTask: () => ({
    isPending: false,
    mutate: (_taskId: string, opts: { onSuccess?: () => void }) => {
      opts.onSuccess?.();
    },
  }),
}));

vi.mock('@/hooks/useAgentEventStream', () => ({
  useAgentEventStream: () => ({
    events: [
      {
        event_type: 'workflow.input.received',
        timestamp: new Date().toISOString(),
        agent_level: 'core',
        session_id: 'test-session',
        run_id: 'task-1',
        task: 'seed',
      } as AnyAgentEvent,
    ],
    isConnected: true,
    isReconnecting: false,
    isSlowRetry: false,
    activeRunId: null,
    error: null,
    reconnectAttempts: 0,
    clearEvents: vi.fn(),
    reconnect: vi.fn(),
    addEvent: vi.fn(),
  }),
}));

vi.mock('@/hooks/useSessionStore', () => ({
  useSessionStore: () => ({
    currentSessionId: null,
    setCurrentSession: vi.fn(),
    addToHistory: vi.fn(),
    clearCurrentSession: vi.fn(),
    removeSession: vi.fn(),
    renameSession: vi.fn(),
    sessionHistory: [],
    sessionLabels: {},
  }),
  useDeleteSession: () => ({
    mutateAsync: vi.fn(),
    mutate: vi.fn(),
    isPending: false,
  }),
}));

describe('ConversationPageContent - stop freezes running timeline', () => {
  const renderWithProviders = (ui: ReactElement) => {
    const queryClient = new QueryClient();
    return render(
      <LanguageProvider>
        <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
      </LanguageProvider>,
    );
  };

  beforeAll(() => {
    vi.stubGlobal('CSS', { escape: (value: string) => value });
    Object.defineProperty(window.HTMLElement.prototype, 'scrollIntoView', {
      value: vi.fn(),
      configurable: true,
    });
    Object.defineProperty(window.HTMLElement.prototype, 'focus', {
      value: vi.fn(),
      configurable: true,
    });
    Object.defineProperty(window.HTMLElement.prototype, 'scrollTop', {
      value: 0,
      writable: true,
      configurable: true,
    });
    Object.defineProperty(window.HTMLElement.prototype, 'scrollHeight', {
      value: 0,
      writable: true,
      configurable: true,
    });
  });

  afterAll(() => {
    vi.unstubAllGlobals();
  });

  it('stops isRunning as soon as user clicks stop', async () => {
    renderWithProviders(<ConversationPageContent />);

    // Submit a task to transition into running state.
    const input = await screen.findByTestId('task-input');
    fireEvent.change(input, { target: { value: 'hello' } });
    fireEvent.click(await screen.findByTestId('task-submit'));

    expect(await screen.findByTestId('task-stop')).toBeInTheDocument();
    expect(await screen.findByTestId('stream-running')).toHaveTextContent('true');
    expect(streamPropsRef.current?.isRunning).toBe(true);

    fireEvent.click(await screen.findByTestId('task-stop'));
    expect(await screen.findByTestId('stream-running')).toHaveTextContent('false');
    expect(streamPropsRef.current?.isRunning).toBe(false);
  });
});

