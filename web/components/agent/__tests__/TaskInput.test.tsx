import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TaskInput } from '../TaskInput';
import { LanguageProvider } from '@/lib/i18n';

describe('TaskInput', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('applies prefill suggestions and submits the task', async () => {
    const onSubmit = vi.fn();
    const onPrefillApplied = vi.fn();

    const Wrapper = ({ prefill }: { prefill: string | null }) => (
      <LanguageProvider>
        <TaskInput
          onSubmit={onSubmit}
          prefill={prefill}
          onPrefillApplied={onPrefillApplied}
        />
      </LanguageProvider>
    );

    const { rerender } = render(<Wrapper prefill="Summarize the latest repo changes" />);

    const textarea = await screen.findByTestId('task-input');

    await waitFor(() => {
      expect(textarea).toHaveValue('Summarize the latest repo changes');
    });
    expect(onPrefillApplied).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(textarea);

    await userEvent.click(screen.getByTestId('task-submit'));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith('Summarize the latest repo changes');
    });

    await waitFor(() => {
      expect(textarea).toHaveValue('');
    });

    rerender(<Wrapper prefill={null} />);
    rerender(<Wrapper prefill="Summarize the latest repo changes" />);

    await waitFor(() => {
      expect(textarea).toHaveValue('Summarize the latest repo changes');
    });
    expect(onPrefillApplied).toHaveBeenCalledTimes(2);
  });

  it('renders stop button while loading and triggers onStop', async () => {
    const onStop = vi.fn();
    const user = userEvent.setup();

    render(
      <LanguageProvider>
        <TaskInput
          onSubmit={vi.fn()}
          loading
          onStop={onStop}
        />
      </LanguageProvider>
    );

    const stopButton = await screen.findByTestId('task-stop');
    expect(stopButton).toBeInTheDocument();

    await user.click(stopButton);
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it('shows pending label when cancellation is in progress', async () => {
    render(
      <LanguageProvider>
        <TaskInput
          onSubmit={vi.fn()}
          loading
          onStop={vi.fn()}
          stopPending
        />
      </LanguageProvider>
    );

    const stopButton = await screen.findByTestId('task-stop');
    expect(stopButton).toHaveTextContent(/Stopping/i);
  });
});
