import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EventLine } from '../EventLine';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

function renderWithI18n(event: AnyAgentEvent, showSubagentContext = true) {
  return render(
    <LanguageProvider>
      <EventLine event={event} showSubagentContext={showSubagentContext} />
    </LanguageProvider>,
  );
}

describe('SubagentEventLine', () => {
  it('renders contextual header for subagent tool call start', () => {
    const event: AnyAgentEvent = {
      event_type: 'workflow.tool.started',
      agent_level: 'subagent',
      session_id: 'session-123',
      run_id: 'task-abc',
      parent_run_id: 'parent-1',
      timestamp: new Date().toISOString(),
      iteration: 1,
      call_id: 'call-1',
      tool_name: 'bash',
      arguments: { command: 'ls' },
      subtask_index: 0,
      total_subtasks: 3,
      subtask_preview: 'List project files for review',
      max_parallel: 2,
      is_subtask: true,
    } as AnyAgentEvent;

    renderWithI18n(event);

    expect(
      screen.getByText('List project files for review'),
    ).toBeInTheDocument();
    expect(screen.getByText('▸ bash(ls)')).toBeInTheDocument();
  });

  it('renders compact tool call for subagent completion', () => {
    const event: AnyAgentEvent = {
      event_type: 'workflow.tool.completed',
      agent_level: 'subagent',
      session_id: 'session-123',
      run_id: 'task-xyz',
      parent_run_id: 'parent-1',
      timestamp: new Date().toISOString(),
      call_id: 'call-42',
      tool_name: 'code_execute',
      result: 'Execution successful',
      duration: 850,
      subtask_index: 1,
      total_subtasks: 2,
      subtask_preview: 'Run project unit tests',
      is_subtask: true,
    } as AnyAgentEvent;

    renderWithI18n(event);

    expect(screen.getByText('Run project unit tests')).toBeInTheDocument();
    expect(
      screen.getByTestId('compact-tool-call-code_execute'),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId('tool-output-card-code_execute'),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/Execution successful/i)).toBeInTheDocument();
  });

  it('shows progress and stats badges for subagent summary events', () => {
    const progressEvent: AnyAgentEvent = {
      event_type: 'workflow.subflow.progress',
      agent_level: 'subagent',
      session_id: 'session-123',
      run_id: 'task-abc',
      parent_run_id: 'parent-1',
      timestamp: new Date().toISOString(),
      completed: 2,
      total: 4,
      tokens: 1200,
      tool_calls: 3,
    } as AnyAgentEvent;

    renderWithI18n(progressEvent);

    expect(screen.getAllByText(/progress 2\/4/i)).not.toHaveLength(0);
    expect(screen.getAllByText(/3 tool calls/i)).not.toHaveLength(0);
    expect(screen.getAllByText(/1200 tokens/i)).not.toHaveLength(0);
  });

  it('uses payload subtask preview when top-level preview is missing', () => {
    const event: AnyAgentEvent = {
      event_type: 'workflow.tool.started',
      agent_level: 'subagent',
      session_id: 'session-123',
      run_id: 'task-abc',
      parent_run_id: 'parent-1',
      timestamp: new Date().toISOString(),
      iteration: 1,
      call_id: 'call-1',
      tool_name: 'bash',
      arguments: { command: 'ls' },
      payload: {
        subtask_preview: 'Payload preview title',
      },
    } as unknown as AnyAgentEvent;

    renderWithI18n(event);

    expect(screen.getByText('Payload preview title')).toBeInTheDocument();
  });
});
