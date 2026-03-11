import { describe, expect, it, vi } from "vitest";

import { loadAttachmentText } from "../attachment-text";

describe("loadAttachmentText", () => {
  it("throws when the remote attachment request fails", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
    }) as any;

    await expect(
      loadAttachmentText({
        name: "missing.txt",
        media_type: "text/plain",
        uri: "https://example.com/missing.txt",
      }),
    ).rejects.toThrow("Failed to load attachment text (404)");
  });
});
