import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TaskCompleteCard } from '../TaskCompleteCard';
import { LanguageProvider } from '@/lib/i18n';
import { WorkflowResultFinalEvent } from '@/lib/types';

const baseEvent: WorkflowResultFinalEvent = {
  event_type: 'workflow.result.final',
  timestamp: new Date().toISOString(),
  agent_level: 'core',
  session_id: 'session-123',
  task_id: 'task-123',
  parent_task_id: undefined,
  final_answer: '',
  total_iterations: 0,
  total_tokens: 0,
  stop_reason: 'cancelled',
  duration: 0,
};

function renderWithProvider(event: WorkflowResultFinalEvent) {
  return render(
    <LanguageProvider>
      <TaskCompleteCard event={event} />
    </LanguageProvider>,
  );
}

describe('TaskCompleteCard', () => {
  it('renders cancellation fallback copy when final answer is empty', () => {
    renderWithProvider({
      ...baseEvent,
      final_answer: '',
      stop_reason: 'cancelled',
      total_iterations: 2,
      total_tokens: 150,
      duration: 1200,
    });

    expect(screen.getByTestId('task-complete-fallback')).toBeInTheDocument();
    expect(screen.getByText(/Task cancelled/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Submit another prompt to continue working/i),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('task-complete-metrics')).not.toBeInTheDocument();
  });

  it('renders markdown answer when final answer is present', () => {
    renderWithProvider({
      ...baseEvent,
      final_answer: 'This is the answer.',
      stop_reason: 'final_answer',
      total_iterations: 3,
      total_tokens: 220,
      duration: 5000,
    });

    expect(screen.queryByTestId('task-complete-fallback')).not.toBeInTheDocument();
    expect(screen.getByText(/This is the answer\./i)).toBeInTheDocument();
    expect(screen.queryByTestId('task-complete-metrics')).not.toBeInTheDocument();
  });

  it('renders inline images from attachment placeholders', () => {
    const imageAnswer = 'Here is the preview: [draft.png] with caption.';
    renderWithProvider({
      ...baseEvent,
      final_answer: imageAnswer,
      stop_reason: 'final_answer',
      attachments: {
        'draft.png': {
          name: 'draft.png',
          description: 'Draft image',
          media_type: 'image/png',
          data: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAOunS9QAAAAASUVORK5CYII=',
        },
      },
    });

    const img = screen.getByRole('img', { name: /Draft image/i });
    expect(img).toBeInTheDocument();
    expect(screen.queryByTestId('task-complete-fallback')).not.toBeInTheDocument();
  });

  it('renders inline video placeholders with VideoPreview', () => {
    const videoAnswer = 'Inline demo: [demo.mp4] finished here.';
    const { container } = renderWithProvider({
      ...baseEvent,
      final_answer: videoAnswer,
      stop_reason: 'final_answer',
      attachments: {
        'demo.mp4': {
          name: 'demo.mp4',
          description: 'Walkthrough video',
          media_type: 'video/mp4',
          uri: 'https://example.com/demo.mp4',
        },
      },
    });

    const video = container.querySelector('video');
    expect(video).toBeInTheDocument();
    expect(video?.querySelector('source')?.getAttribute('src')).toBe(
      'https://example.com/demo.mp4',
    );
  });

  it('renders inline document placeholders with artifact preview', () => {
    const docAnswer = 'See the spec: [plan.pdf]';
    renderWithProvider({
      ...baseEvent,
      final_answer: docAnswer,
      stop_reason: 'final_answer',
      attachments: {
        'plan.pdf': {
          name: 'plan.pdf',
          description: 'Project plan',
          media_type: 'application/pdf',
          uri: 'https://example.com/plan.pdf',
          format: 'pdf',
        },
      },
    });

    expect(screen.getByText(/Project plan/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /View/i })).toHaveAttribute(
      'href',
      'https://example.com/plan.pdf',
    );
  });

  it('shows streaming indicator while final answer is streaming', () => {
    renderWithProvider({
      ...baseEvent,
      final_answer: 'Partial stream chunk',
      is_streaming: true,
      stream_finished: false,
      stop_reason: 'final_answer',
    });

    expect(screen.getByTestId('markdown-streaming-indicator')).toBeInTheDocument();
    expect(screen.queryByTestId('task-complete-fallback')).not.toBeInTheDocument();
    expect(screen.getByText(/Partial stream chunk/i)).toBeInTheDocument();
  });

  it('renders streaming state before any content arrives', () => {
    renderWithProvider({
      ...baseEvent,
      final_answer: '',
      is_streaming: true,
      stream_finished: false,
      stop_reason: 'final_answer',
    });

    expect(screen.getByTestId('markdown-streaming-indicator')).toBeInTheDocument();
    expect(screen.queryByTestId('task-complete-fallback')).not.toBeInTheDocument();
  });

  it('hides streaming indicator once stream is finished', () => {
    renderWithProvider({
      ...baseEvent,
      final_answer: 'All set.',
      is_streaming: false,
      stream_finished: true,
      stop_reason: 'final_answer',
    });

    expect(screen.queryByTestId('markdown-streaming-indicator')).not.toBeInTheDocument();
    expect(screen.getByText(/All set\./i)).toBeInTheDocument();
  });
});
