import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EventLine } from '../EventLine';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

describe('EventLine tool start rendering', () => {
  it('renders tool output card for core tool start events', () => {
    const event: AnyAgentEvent = {
      event_type: 'workflow.tool.started',
      agent_level: 'core',
      session_id: 'session-123',
      task_id: 'task-123',
      timestamp: new Date().toISOString(),
      iteration: 1,
      call_id: 'call-1',
      tool_name: 'text_to_image',
      arguments: { prompt: 'Generate storyboard frame' },
    } as AnyAgentEvent;

    render(
      <LanguageProvider>
        <EventLine event={event} />
      </LanguageProvider>,
    );

    expect(
      screen.getByTestId('tool-output-card-text_to_image'),
    ).toBeInTheDocument();
  });
});
