import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
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

  it('renders the latest task_complete event inline without duplication', () => {
    const firstTimestamp = new Date().toISOString();
    const completionTimestamp = new Date(Date.now() + 1500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'task_complete',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        final_answer: 'All done',
        total_iterations: 2,
        total_tokens: 1234,
        stop_reason: 'complete',
        duration: 3200,
        attachments: {
          'image-1': {
            name: 'diagram.png',
            uri: '/attachments/diagram.png',
            description: 'A diagram',
          },
        },
        timestamp: completionTimestamp,
      },
      {
        event_type: 'assistant_message',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        iteration: 1,
        delta: 'Trailing message that should not appear',
        final: true,
        timestamp: firstTimestamp,
        created_at: firstTimestamp,
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

    expect(screen.getByText(/all done/i)).toBeInTheDocument();
    expect(screen.queryByTestId('terminal-final-answer')).not.toBeInTheDocument();

    // Ensure the completion is shown once in the event stream
    expect(screen.getAllByTestId('task-complete-event')).toHaveLength(1);
  });

  it('aggregates subagent events under a single panel', () => {
    const subagentTimestamp = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'tool_call_complete',
        agent_level: 'subagent',
        call_id: 'subagent:1',
        tool_name: 'web_search',
        result: 'Fetched references',
        duration: 120,
        timestamp: subagentTimestamp,
        task_id: 'task-1',
        parent_task_id: 'parent-1',
        subtask_index: 0,
        total_subtasks: 2,
        subtask_preview: 'Collect references',
      },
      {
        event_type: 'task_complete',
        agent_level: 'subagent',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: 'parent-1',
        subtask_index: 0,
        total_subtasks: 2,
        final_answer: 'Ready to merge findings',
        total_iterations: 1,
        total_tokens: 1200,
        stop_reason: 'complete',
        duration: 3200,
        timestamp: new Date(Date.now() + 800).toISOString(),
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

    const aggregatePanel = screen.getByTestId('subagent-aggregate-panel');
    expect(aggregatePanel).toBeInTheDocument();
    expect(within(aggregatePanel).getAllByTestId(/event-subagent/)).toHaveLength(2);
    expect(aggregatePanel).toHaveTextContent(/Subagent Task 1\/2/i);

    const conversationEvents = screen.getByTestId('conversation-events');
    expect(within(conversationEvents).getAllByTestId(/event-user_task/)).toHaveLength(1);
  });
});
