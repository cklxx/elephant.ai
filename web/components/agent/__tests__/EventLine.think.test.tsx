import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';

import { EventLine } from '../EventLine';
import { LanguageProvider } from '@/lib/i18n';
import { ThinkCompleteEvent } from '@/lib/types';

function renderThinkEvent(event: ThinkCompleteEvent) {
  return render(
    <LanguageProvider>
      <EventLine event={event} />
    </LanguageProvider>,
  );
}

describe('EventLine (think_complete)', () => {
  it('renders attachments using the task complete card', () => {
    const thinkEvent: ThinkCompleteEvent = {
      event_type: 'think_complete',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-123',
      iteration: 1,
      content: 'Please review the generated clip: [demo-video]',
      tool_call_count: 1,
      attachments: {
        'demo-video': {
          name: 'demo-video.mp4',
          media_type: 'video/mp4',
          uri: 'https://example.com/demo-video.mp4',
          description: 'Demo video clip',
        },
      },
    };

    const { container, getByTestId } = renderThinkEvent(thinkEvent);

    expect(getByTestId('task-complete-event')).toBeInTheDocument();

    const video = container.querySelector('video');
    expect(video).toBeInTheDocument();
    expect(video?.querySelector('source')?.getAttribute('src')).toBe(
      'https://example.com/demo-video.mp4',
    );
  });

  it('renders preview asset videos when the attachment lacks a direct uri', () => {
    const thinkEvent: ThinkCompleteEvent = {
      event_type: 'think_complete',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-456',
      iteration: 2,
      content: 'Here is the final render: [rendered-clip]',
      tool_call_count: 2,
      attachments: {
        'rendered-clip': {
          name: 'rendered-clip',
          media_type: 'application/octet-stream',
          preview_assets: [
            {
              label: 'video-preview',
              mime_type: 'video/mp4',
              cdn_url: 'https://cdn.example.com/rendered.mp4',
            },
          ],
        },
      },
    };

    const { container } = renderThinkEvent(thinkEvent);

    const video = container.querySelector('video');
    expect(video).toBeInTheDocument();
    expect(video?.querySelector('source')?.getAttribute('src')).toBe(
      'https://cdn.example.com/rendered.mp4',
    );
  });
});
