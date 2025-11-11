import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TaskCompleteCard } from '../TaskCompleteCard';
import { LanguageProvider } from '@/lib/i18n';
import { TaskCompleteEvent } from '@/lib/types';

const baseEvent: TaskCompleteEvent = {
  event_type: 'task_complete',
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

function renderWithProvider(event: TaskCompleteEvent) {
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
    expect(screen.getByTestId('task-complete-metrics')).toHaveTextContent(
      /2 iterations/i,
    );
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
    expect(screen.getByTestId('task-complete-metrics')).toHaveTextContent(
      /3 iterations/i,
    );
  });
});
