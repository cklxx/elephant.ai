import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TerminalOutput } from '../TerminalOutput';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

const baseEvent: AnyAgentEvent = {
  event_type: 'user_task',
  timestamp: new Date().toISOString(),
  agent_level: 'core',
  session_id: 'session-1',
  task_id: 'task-1',
  task: 'Summarize the latest output',
};

describe('TerminalOutput', () => {
  it('filters assistant_message events from the output stream', () => {
    const firstTimestamp = new Date().toISOString();
    const thirdTimestamp = new Date(Date.now() + 2000).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'assistant_message',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        iteration: 1,
        delta: 'Here is the summary',
        final: false,
        timestamp: firstTimestamp,
        created_at: firstTimestamp,
      },
      {
        event_type: 'assistant_message',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        iteration: 1,
        delta: ' with additional context.',
        final: true,
        timestamp: thirdTimestamp,
        created_at: thirdTimestamp,
      },
    ];

    render(
      <LanguageProvider>
        <TerminalOutput
          events={events}
          isConnected
          isReconnecting={false}
          error={null}
          reconnectAttempts={0}
          onReconnect={() => {}}
        />
      </LanguageProvider>,
    );

    expect(screen.getByTestId('conversation-stream')).toBeInTheDocument();
    expect(
      screen.queryByText(/Here is the summary with additional context\./i),
    ).not.toBeInTheDocument();
  });
});
