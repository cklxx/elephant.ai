import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { ConversationEventStream } from '../ConversationEventStream';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

const baseEvent: AnyAgentEvent = {
  event_type: 'workflow.input.received',
  session_id: 'session-1',
  run_id: 'task-1',
  agent_level: 'core',
  task: 'Summarize the latest output',
  timestamp: new Date().toISOString(),
};

describe('ConversationEventStream', () => {
  it('renders input event', () => {
    render(
      <LanguageProvider>
        <ConversationEventStream
          events={[baseEvent]}
          isConnected
          isReconnecting={false}
          error={null}
          reconnectAttempts={0}
          onReconnect={() => {}}
        />
      </LanguageProvider>,
    );

    expect(screen.getByTestId('event-workflow.input.received')).toBeInTheDocument();
  });

  it('renders tool completed event with arguments from paired started event', () => {
    const startedAt = new Date().toISOString();
    const completedAt = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'task-1',
        call_id: 'call-1',
        tool_name: 'web_search',
        arguments: { query: 'test query' },
        timestamp: startedAt,
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'task-1',
        call_id: 'call-1',
        tool_name: 'web_search',
        result: 'Search results',
        duration: 120,
        timestamp: completedAt,
      },
    ];

    render(
      <LanguageProvider>
        <ConversationEventStream
          events={events}
          isConnected
          isReconnecting={false}
          error={null}
          reconnectAttempts={0}
          onReconnect={() => {}}
        />
      </LanguageProvider>,
    );

    expect(screen.getByTestId('event-workflow.tool.completed')).toBeInTheDocument();
  });

  it('renders subagent tool call with aggregated card', () => {
    const toolTimestamp = new Date(Date.now() + 300).toISOString();
    const subagentTimestamp = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'parent-1',
        call_id: 'subagent-call-1',
        tool_name: 'subagent',
        result: 'Subtask started',
        duration: 100,
        timestamp: toolTimestamp,
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        session_id: 'session-1',
        call_id: 'tool-call-1',
        tool_name: 'web_search',
        result: 'Fetched references',
        duration: 120,
        timestamp: subagentTimestamp,
        run_id: 'task-1',
        parent_run_id: 'parent-1',
        subtask_index: 0,
        total_subtasks: 1,
        subtask_preview: 'Collect references',
      },
      {
        event_type: 'workflow.result.final',
        agent_level: 'subagent',
        session_id: 'session-1',
        run_id: 'task-1',
        parent_run_id: 'parent-1',
        subtask_index: 0,
        total_subtasks: 1,
        final_answer: 'Subtask completed',
        total_iterations: 1,
        total_tokens: 1200,
        stop_reason: 'complete',
        duration: 3200,
        timestamp: new Date(Date.now() + 800).toISOString(),
      },
    ];

    render(
      <LanguageProvider>
        <ConversationEventStream
          events={events}
          isConnected
          isReconnecting={false}
          error={null}
          reconnectAttempts={0}
          onReconnect={() => {}}
        />
      </LanguageProvider>,
    );

    // Subagent tool call should be visible
    expect(screen.getAllByTestId('event-workflow.tool.completed')).toHaveLength(1);

    // Subagent card should be visible
    const cards = screen.getAllByTestId('subagent-thread');
    expect(cards).toHaveLength(1);

    // Card should contain aggregated events
    expect(within(cards[0]).getAllByTestId(/event-workflow/).length).toBeGreaterThanOrEqual(1);
  });

  it('filters prepare node events', () => {
    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.node.started',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'task-1',
        node_id: 'prepare',
        timestamp: new Date().toISOString(),
      } as AnyAgentEvent,
    ];

    render(
      <LanguageProvider>
        <ConversationEventStream
          events={events}
          isConnected
          isReconnecting={false}
          error={null}
          reconnectAttempts={0}
          onReconnect={() => {}}
        />
      </LanguageProvider>,
    );

    expect(screen.getByTestId('event-workflow.input.received')).toBeInTheDocument();
    expect(screen.queryByTestId('event-workflow.node.started')).not.toBeInTheDocument();
  });

  it('renders connection banner when not connected', () => {
    render(
      <LanguageProvider>
        <ConversationEventStream
          events={[baseEvent]}
          isConnected={false}
          isReconnecting={false}
          error={null}
          reconnectAttempts={0}
          onReconnect={() => {}}
        />
      </LanguageProvider>,
    );

    expect(screen.getByText('Offline')).toBeInTheDocument();
  });
});
