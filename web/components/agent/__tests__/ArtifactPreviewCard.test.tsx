import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { ArtifactPreviewCard } from '../ArtifactPreviewCard';
import { AttachmentPayload } from '@/lib/types';

describe('ArtifactPreviewCard', () => {
  const attachment: AttachmentPayload = {
    name: 'Example Title.md',
    description: 'Example Title',
    uri: 'https://example.com/doc.md',
    media_type: 'text/markdown',
    format: 'markdown',
    kind: 'artifact',
  };

  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      text: vi.fn().mockResolvedValue('# Example Title\n\nBody content.'),
    } as unknown as Response);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows the markdown heading when opening the preview', async () => {
    const user = userEvent.setup();

    render(<ArtifactPreviewCard attachment={attachment} />);

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(attachment.uri);
    });

    await user.click(screen.getByRole('button', { name: /preview example title/i }));

    const dialog = await screen.findByRole('dialog');
    const headings = within(dialog).getAllByRole('heading', { name: 'Example Title' });
    expect(headings.some((node) => !node.classList.contains('sr-only'))).toBe(true);
  });

  it('renders markdown tables for mkd attachments', async () => {
    const user = userEvent.setup();
    const mkdAttachment: AttachmentPayload = {
      name: 'table.mkd',
      description: 'Table Preview',
      uri: 'https://example.com/table.mkd',
      media_type: 'text/plain',
      format: 'mkd',
      kind: 'artifact',
    };

    const fetchMock = vi.mocked(globalThis.fetch);
    fetchMock.mockResolvedValueOnce({
      text: vi.fn().mockResolvedValue(
        [
          '# Table Preview',
          '',
          '| Name | Count |',
          '| --- | --- |',
          '| Alpha | 1 |',
          '| Beta | 2 |',
        ].join('\n'),
      ),
    } as unknown as Response);

    render(<ArtifactPreviewCard attachment={mkdAttachment} />);

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(mkdAttachment.uri);
    });

    await user.click(screen.getByRole('button', { name: /preview table preview/i }));

    const dialog = await screen.findByRole('dialog');
    const table = within(dialog).getByRole('table');
    expect(table).toBeTruthy();
    expect(within(table).getByText('Alpha')).toBeTruthy();
  });
});
