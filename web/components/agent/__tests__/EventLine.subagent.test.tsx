import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EventLine } from '../EventLine';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

function renderWithI18n(event: AnyAgentEvent) {
  return render(
    <LanguageProvider>
      <EventLine event={event} />
    </LanguageProvider>,
  );
}

describe('SubagentEventLine', () => {
  it('renders contextual header for subagent tool call start', () => {
    const event: AnyAgentEvent = {
      event_type: 'tool_call_start',
      agent_level: 'subagent',
      session_id: 'session-123',
      task_id: 'task-abc',
      parent_task_id: 'parent-1',
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

    expect(screen.getByText('Subagent Task 1/3')).toBeInTheDocument();
    expect(
      screen.getByText('List project files for review'),
    ).toBeInTheDocument();
    expect(screen.getByText('Parallel ×2')).toBeInTheDocument();
    expect(screen.getByText('▸ bash(ls)')).toBeInTheDocument();
  });

  it('renders tool output card for subagent completion', () => {
    const event: AnyAgentEvent = {
      event_type: 'tool_call_complete',
      agent_level: 'subagent',
      session_id: 'session-123',
      task_id: 'task-xyz',
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

    expect(screen.getByText('Subagent Task 2/2')).toBeInTheDocument();
    expect(screen.getByText('Run project unit tests')).toBeInTheDocument();
    expect(screen.getByText(/Execution successful/i)).toBeInTheDocument();
  });
});
