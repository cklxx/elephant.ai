import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { DocumentCanvas } from '../DocumentCanvas';
import { LanguageProvider } from '@/lib/i18n';
import { AttachmentPayload } from '@/lib/types';

describe('DocumentCanvas attachments', () => {
  it('uses an auto-fit grid for image previews', () => {
    const attachments: Record<string, AttachmentPayload> = {
      'image-1': {
        name: 'image-1.png',
        media_type: 'image/png',
        uri: 'https://example.com/image-1.png',
      },
      'image-2': {
        name: 'image-2.png',
        media_type: 'image/png',
        uri: 'https://example.com/image-2.png',
      },
    };

    render(
      <LanguageProvider>
        <DocumentCanvas
          document={{
            id: 'doc-1',
            title: 'Attachment Test',
            content: 'Attachment preview',
            type: 'markdown',
            attachments,
          }}
        />
      </LanguageProvider>,
    );

    const grid = screen.getByTestId('document-attachment-images');
    expect(grid).toHaveClass(
      'grid-cols-[repeat(auto-fit,minmax(220px,1fr))]',
    );
  });

  it('does not render unsafe javascript links from markdown', () => {
    render(
      <LanguageProvider>
        <DocumentCanvas
          document={{
            id: 'doc-unsafe-link',
            title: 'Unsafe link',
            content: '[open](javascript:alert(1))',
            type: 'markdown',
          }}
        />
      </LanguageProvider>,
    );

    expect(screen.queryByRole('link', { name: /open/i })).toBeNull();
    expect(screen.getByText(/open/i)).toHaveTextContent('open');
    expect(screen.getByText(/open/i)).toHaveTextContent('[blocked]');
  });
});
