import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import type { AttachmentPayload } from '@/lib/types';

describe('attachments uri cache', () => {
  const originalURL = globalThis.URL;
  const originalBlob = globalThis.Blob;
  const originalCreateObjectURL = originalURL?.createObjectURL;
  const originalRevokeObjectURL = originalURL?.revokeObjectURL;
  let createObjectURL: ReturnType<typeof vi.fn>;
  let revokeObjectURL: ReturnType<typeof vi.fn>;
  let counter = 0;

  beforeEach(() => {
    vi.resetModules();
    process.env.NEXT_PUBLIC_BLOB_URL_CACHE_LIMIT = '2';
    counter = 0;
    createObjectURL = vi.fn(() => `blob:${counter++}`);
    revokeObjectURL = vi.fn();
    if (originalURL) {
      originalURL.createObjectURL = createObjectURL;
      originalURL.revokeObjectURL = revokeObjectURL;
      globalThis.URL = originalURL;
    } else {
      globalThis.URL = {
        createObjectURL,
        revokeObjectURL,
      } as typeof URL;
    }
    globalThis.Blob = class {
      constructor(_parts: unknown[], _opts?: BlobPropertyBag) {}
    } as typeof Blob;
  });

  afterEach(() => {
    if (originalURL) {
      if (originalCreateObjectURL) {
        originalURL.createObjectURL = originalCreateObjectURL;
      } else {
        delete (originalURL as typeof URL & { createObjectURL?: typeof URL.createObjectURL })
          .createObjectURL;
      }
      if (originalRevokeObjectURL) {
        originalURL.revokeObjectURL = originalRevokeObjectURL;
      } else {
        delete (originalURL as typeof URL & { revokeObjectURL?: typeof URL.revokeObjectURL })
          .revokeObjectURL;
      }
      globalThis.URL = originalURL;
    }
    if (originalBlob) {
      globalThis.Blob = originalBlob;
    }
    delete process.env.NEXT_PUBLIC_BLOB_URL_CACHE_LIMIT;
  });

  it('reuses blob urls for identical base64 payloads', async () => {
    const { buildAttachmentUri } = await import('@/lib/attachments/uri');
    const attachment: AttachmentPayload = {
      name: 'note.txt',
      media_type: 'text/plain',
      data: Buffer.from('hello').toString('base64'),
    };

    const first = buildAttachmentUri(attachment);
    const second = buildAttachmentUri(attachment);

    expect(first).toBe(second);
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledTimes(0);
  });

  it('evicts oldest blob urls when cache limit is exceeded', async () => {
    const { buildAttachmentUri } = await import('@/lib/attachments/uri');
    const attachments: AttachmentPayload[] = [
      { name: 'a.txt', media_type: 'text/plain', data: Buffer.from('a').toString('base64') },
      { name: 'b.txt', media_type: 'text/plain', data: Buffer.from('b').toString('base64') },
      { name: 'c.txt', media_type: 'text/plain', data: Buffer.from('c').toString('base64') },
    ];

    const urls = attachments.map((att) => buildAttachmentUri(att));

    expect(urls.filter(Boolean)).toHaveLength(3);
    expect(createObjectURL).toHaveBeenCalledTimes(3);
    expect(revokeObjectURL).toHaveBeenCalledTimes(1);
  });
});
