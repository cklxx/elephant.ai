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
  it('aggregates assistant_message deltas into a single markdown bubble', () => {
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

    expect(screen.getByText(/Here is the summary with additional context\./i)).toBeInTheDocument();
  });

  it('hides auto review events in the live conversation view', () => {
    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'auto_review',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        timestamp: new Date().toISOString(),
        summary: {
          assessment: {
            grade: 'C',
            needs_rework: true,
          },
        },
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

    expect(screen.queryByTestId('event-auto_review')).toBeNull();
  });
});
