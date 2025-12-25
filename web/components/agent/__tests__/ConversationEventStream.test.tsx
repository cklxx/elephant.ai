import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { ConversationEventStream } from '../ConversationEventStream';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

const baseEvent: AnyAgentEvent = {
  event_type: 'workflow.input.received',
  timestamp: new Date().toISOString(),
  agent_level: 'core',
  session_id: 'session-1',
  task_id: 'task-1',
  task: 'Summarize the latest output',
};

describe('ConversationEventStream', () => {
  it('enriches tool output titles with start arguments', () => {
    const startedAt = new Date().toISOString();
    const completedAt = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        call_id: 'call-1',
        tool_name: 'web_search',
        arguments: {
          query: 'PM iteration pain points',
        },
        timestamp: startedAt,
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        call_id: 'call-1',
        tool_name: 'web_search',
        result: 'Search results',
        duration: 120,
        timestamp: completedAt,
      },
      {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        call_id: 'call-2',
        tool_name: 'artifacts_list',
        arguments: {
          name: 'pm_iteration_article.md',
        },
        timestamp: startedAt,
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        call_id: 'call-2',
        tool_name: 'artifacts_list',
        result: 'Attachments on record',
        duration: 50,
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

    expect(screen.getByText('正在查找：PM iteration pain points')).toBeInTheDocument();
    expect(screen.getByText('查看文件：pm_iteration_article.md')).toBeInTheDocument();
  });

  it('filters workflow.node.output.delta events from the output stream', () => {
    const firstTimestamp = new Date().toISOString();
    const thirdTimestamp = new Date(Date.now() + 2000).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.node.output.delta',
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
        event_type: 'workflow.node.output.delta',
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

    expect(screen.getByTestId('conversation-stream')).toBeInTheDocument();
    expect(
      screen.queryByText(/Here is the summary with additional context\./i),
    ).not.toBeInTheDocument();
  });

  it('renders the latest workflow.result.final event inline without duplication', () => {
    const firstTimestamp = new Date().toISOString();
    const completionTimestamp = new Date(Date.now() + 1500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.result.final',
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
        event_type: 'workflow.node.output.delta',
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

    expect(screen.getByText(/all done/i)).toBeInTheDocument();
    expect(screen.queryByTestId('terminal-final-answer')).not.toBeInTheDocument();

    // Ensure the completion is shown once in the event stream
    expect(screen.getAllByTestId('task-complete-event')).toHaveLength(1);
  });

  it('dedupes workflow.node.output.summary when it repeats the final answer', () => {
    const summaryTimestamp = new Date(Date.now() + 500).toISOString();
    const completionTimestamp = new Date(Date.now() + 1500).toISOString();
    const content = '## Final Summary\n\n- ✅ Done\n';

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.node.output.summary',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        iteration: 1,
        content,
        timestamp: summaryTimestamp,
      } as AnyAgentEvent,
      {
        event_type: 'workflow.result.final',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: undefined,
        final_answer: content,
        total_iterations: 2,
        total_tokens: 1234,
        stop_reason: 'complete',
        duration: 3200,
        timestamp: completionTimestamp,
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

    expect(screen.getAllByTestId('task-complete-event')).toHaveLength(1);
    expect(screen.queryByTestId('event-workflow.node.output.summary')).not.toBeInTheDocument();
  });

  it('aggregates subagent events under a single panel', () => {
    const subagentTimestamp = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.completed',
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
        event_type: 'workflow.result.final',
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

    const threads = screen.getAllByTestId('subagent-thread');
    expect(threads).toHaveLength(1);
    expect(within(threads[0]).getAllByTestId(/event-subagent/)).toHaveLength(2);
    expect(threads[0]).toHaveTextContent(/Subagent Task 1\/2/i);
    expect(within(threads[0]).getAllByText(/Subagent Task 1\/2/i)).toHaveLength(1);

    const conversationEvents = screen.getByTestId('conversation-events');
    expect(within(conversationEvents).getAllByTestId(/event-workflow.input.received/)).toHaveLength(1);
  });

  it('orders subagent threads by their first seen timestamp even without displayable events', () => {
    const baseTimestamp = new Date().toISOString();
    const progressTimestamp = new Date(Date.now() + 1000).toISOString();

    const events: AnyAgentEvent[] = [
      { ...baseEvent, timestamp: baseTimestamp },
      {
        event_type: 'workflow.tool.progress',
        agent_level: 'subagent',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: 'parent-1',
        subtask_index: 0,
        total_subtasks: 1,
        tool_name: 'web_search',
        progress: 0.3,
        timestamp: progressTimestamp,
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

    const conversationEvents = screen.getByTestId('conversation-events');
    const children = Array.from(conversationEvents.children);
    const baseIndex = children.findIndex((node) =>
      node.querySelector('[data-testid="event-workflow.input.received"]'),
    );
    const subagentIndex = children.findIndex(
      (node) => node.getAttribute('data-testid') === 'subagent-thread',
    );

    expect(baseIndex).toBeGreaterThanOrEqual(0);
    expect(subagentIndex).toBeGreaterThanOrEqual(0);
    expect(subagentIndex).toBeGreaterThan(baseIndex);
  });

  it('ignores delegation-only subagent tool events', () => {
    const subagentTimestamp = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        session_id: 'session-1',
        task_id: 'task-1',
        parent_task_id: 'parent-1',
        subtask_index: 0,
        total_subtasks: 1,
        tool_name: 'subagent',
        result: 'delegation summary',
        duration: 200,
        timestamp: subagentTimestamp,
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

    expect(screen.queryByTestId('subagent-thread')).not.toBeInTheDocument();
    expect(screen.getByTestId('conversation-stream')).toBeInTheDocument();
  });

  it('routes core-level events with parent_task_id into the subagent aggregate', () => {
    const subagentTimestamp = new Date(Date.now() + 500).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: 'session-1',
        task_id: 'subtask-123',
        parent_task_id: 'parent-1',
        tool_name: 'text_to_image',
        result: 'image generated',
        duration: 200,
        timestamp: subagentTimestamp,
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

    const threads = screen.getAllByTestId('subagent-thread');
    expect(threads).toHaveLength(1);
    expect(within(threads[0]).getAllByTestId(/event-subagent/)).toHaveLength(1);

    expect(within(threads[0]).getByTestId('event-workflow.tool.completed')).toBeInTheDocument();
    expect(screen.getAllByTestId('event-workflow.tool.completed')).toHaveLength(1);
  });

  it('renders clearify events when present', () => {
    const planTimestamp = new Date(Date.now() + 500).toISOString();
    const clearifyTimestamp = new Date(Date.now() + 1000).toISOString();

    const events: AnyAgentEvent[] = [
      baseEvent,
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: baseEvent.session_id,
        task_id: baseEvent.task_id,
        tool_name: 'plan',
        result: 'Handle two tasks',
        duration: 1,
        timestamp: planTimestamp,
        metadata: {
          internal_plan: {
            steps: [
              {
                task_goal: 'Single task',
                success_criteria: [],
              },
            ],
          },
        },
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: baseEvent.session_id,
        task_id: baseEvent.task_id,
        tool_name: 'clearify',
        result: 'Single task detail should render',
        duration: 1,
        timestamp: clearifyTimestamp,
        metadata: {
          task_goal_ui: 'Single task detail should render',
          success_criteria: ['displayed'],
        },
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

    expect(screen.getByText(/handle two tasks/i)).toBeInTheDocument();
    expect(screen.getByText(/single task detail should render/i)).toBeInTheDocument();
  });
});
