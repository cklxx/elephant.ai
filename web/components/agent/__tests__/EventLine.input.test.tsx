import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { EventLine } from '../EventLine';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

function renderInputEvent(event: AnyAgentEvent) {
  return render(
    <LanguageProvider>
      <EventLine event={event} />
    </LanguageProvider>,
  );
}

describe('EventLine (workflow.input.received)', () => {
  it('uses a wrapping grid for multiple media attachments', () => {
    const inputEvent: AnyAgentEvent = {
      event_type: 'workflow.input.received',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-input-1',
      task_id: 'task-input-1',
      task: 'Review these clips',
      attachments: {
        'clip-1': {
          name: 'clip-1.mp4',
          media_type: 'video/mp4',
          uri: 'https://example.com/clip-1.mp4',
        },
        'clip-2': {
          name: 'clip-2.mp4',
          media_type: 'video/mp4',
          uri: 'https://example.com/clip-2.mp4',
        },
      },
    };

    renderInputEvent(inputEvent);

    const mediaGrid = screen.getByTestId('event-input-media');
    expect(mediaGrid).toHaveClass(
      'grid-cols-[repeat(auto-fit,minmax(220px,1fr))]',
    );
  });
});
