import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import { ToolResultPanel } from '../tooling/ToolPanels';
import { AttachmentPayload } from '@/lib/types';

describe('ToolResultPanel', () => {
  it('uses an auto-fit grid for media attachments', () => {
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
      <ToolResultPanel
        toolName="text_to_image"
        result=""
        error={null}
        resultTitle="Output"
        errorTitle="Error"
        copyLabel="Copy"
        copyErrorLabel="Copy error"
        copiedLabel="Copied"
        attachments={attachments}
        metadata={null}
      />,
    );

    const grid = screen.getByTestId('tool-result-media');
    expect(grid).toHaveClass(
      'grid-cols-[repeat(auto-fit,minmax(220px,1fr))]',
    );
  });
});
