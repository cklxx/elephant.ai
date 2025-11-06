import { describe, expect, it, vi, beforeEach, beforeAll, afterAll } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TaskInput } from '../TaskInput';
import { LanguageProvider } from '@/lib/i18n';

class MockFileReader {
  public result: string | ArrayBuffer | null = 'data:image/png;base64,Zm9v';
  public onload: ((event: ProgressEvent<FileReader>) => void) | null = null;
  public onerror: ((event: ProgressEvent<FileReader>) => void) | null = null;

  readAsDataURL(_: Blob) {
    if (typeof this.onload === 'function') {
      this.onload(new ProgressEvent('load'));
    }
  }
}

beforeAll(() => {
  vi.stubGlobal('FileReader', MockFileReader as unknown as typeof FileReader);
});

afterAll(() => {
  vi.unstubAllGlobals();
});

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
      expect(onSubmit).toHaveBeenCalledWith('Summarize the latest repo changes', []);
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
      </LanguageProvider>,
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
      </LanguageProvider>,
    );

    const stopButton = await screen.findByTestId('task-stop');
    expect(stopButton).toHaveTextContent(/Stopping/i);
  });

  it('allows adding image attachments and includes them on submit', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(
      <LanguageProvider>
        <TaskInput onSubmit={onSubmit} />
      </LanguageProvider>,
    );

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['foo'], 'diagram.png', { type: 'image/png' });
    Object.defineProperty(fileInput, 'files', {
      value: [file],
      writable: false,
    });

    await user.click(screen.getByTestId('task-attach-image'));
    fireEvent.change(fileInput);

    await waitFor(() => {
      expect(screen.getByTestId('task-attachments')).toBeInTheDocument();
    });

    const textarea = await screen.findByTestId('task-input');
    expect(textarea).toHaveValue('[diagram.png]');

    await user.click(screen.getByTestId('task-submit'));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith('[diagram.png]', [
        expect.objectContaining({
          name: 'diagram.png',
          media_type: 'image/png',
          data: 'Zm9v',
          source: 'user_upload',
        }),
      ]);
    });
  });
});
