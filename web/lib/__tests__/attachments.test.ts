import { describe, expect, it } from 'vitest';

import {
  buildAttachmentUri,
  getAttachmentSegmentType,
} from '../attachments';

describe('attachments helpers', () => {
  it('prefers preview asset URLs when no direct uri or data is provided', () => {
    const attachment = {
      name: 'clip',
      media_type: 'application/octet-stream',
      preview_assets: [
        {
          label: 'lowres',
          mime_type: 'image/png',
          cdn_url: 'https://cdn.example.com/thumb.png',
        },
        {
          label: 'video',
          mime_type: 'video/mp4',
          cdn_url: 'https://cdn.example.com/clip.mp4',
        },
      ],
    };

    const uri = buildAttachmentUri(attachment as any);

    expect(uri).toBe('https://cdn.example.com/clip.mp4');
  });

  it('detects video attachments from preview assets even without media_type', () => {
    const attachment = {
      name: 'rendered-clip',
      preview_assets: [
        {
          label: 'video',
          preview_type: 'video',
          cdn_url: 'https://cdn.example.com/rendered.mp4',
        },
      ],
    };

    const type = getAttachmentSegmentType(attachment as any);

    expect(type).toBe('video');
  });

  it('treats markdown attachments as documents even without explicit format', () => {
    const attachment = {
      name: 'web_example_20240101.md',
      media_type: 'text/markdown',
      data: 'IyBIZWxsbw==',
      uri: 'data:text/markdown;base64,IyBIZWxsbw==',
    };

    const type = getAttachmentSegmentType(attachment as any);

    expect(type).toBe('document');
  });
});
