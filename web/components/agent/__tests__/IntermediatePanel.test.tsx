import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { IntermediatePanel } from '../IntermediatePanel';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

const renderPanel = (events: AnyAgentEvent[]) =>
  render(
    <LanguageProvider>
      <IntermediatePanel events={events} />
    </LanguageProvider>,
  );

describe('IntermediatePanel', () => {
  it('shows enriched summary for running tool calls', () => {
    const timestamp = new Date().toISOString();
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        call_id: 'call-1',
        tool_name: 'web_fetch',
        arguments: { url: 'https://example.com' },
        timestamp,
        session_id: 's1',
        task_id: 't1',
        parent_task_id: undefined,
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: 'call-1',
        tool_name: 'web_fetch',
        result: 'Fetched 200 OK',
        duration: 1200,
        timestamp: new Date(Date.now() + 50).toISOString(),
        session_id: 's1',
        task_id: 't1',
        parent_task_id: undefined,
      },
      {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        call_id: 'call-2',
        tool_name: 'bash',
        arguments: { command: 'npm test -- --watch=false' },
        timestamp: new Date(Date.now() + 100).toISOString(),
        session_id: 's1',
        task_id: 't1',
        parent_task_id: undefined,
      },
    ];

    renderPanel(events);

    expect(
      screen.getByText(/Run Shell · npm test -- --watch=false/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/1 running/i)).toBeInTheDocument();
    expect(screen.getByText(/1 done/i)).toBeInTheDocument();
  });

  it('falls back to completed tool previews when nothing is running', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        call_id: 'call-3',
        tool_name: 'web_fetch',
        arguments: { url: 'https://news.example.com' },
        timestamp: new Date().toISOString(),
        session_id: 's2',
        task_id: 't2',
        parent_task_id: undefined,
      },
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: 'call-3',
        tool_name: 'web_fetch',
        result: 'Headline: Example News',
        duration: 800,
        timestamp: new Date(Date.now() + 25).toISOString(),
        session_id: 's2',
        task_id: 't2',
        parent_task_id: undefined,
      },
    ];

    renderPanel(events);

    expect(
      screen.getByText(/Fetch Page · https:\/\/news\.example\.com/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/Headline: Example News/i)).toBeInTheDocument();
    expect(screen.queryByText(/running/i)).not.toBeInTheDocument();
  });
});
