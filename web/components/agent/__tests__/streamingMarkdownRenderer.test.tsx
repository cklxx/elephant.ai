import { describe, expect, it } from "vitest";

import { splitStreamingContent } from "@/components/agent/StreamingMarkdownRenderer";

describe("splitStreamingContent", () => {
  it("keeps a raw tail for long streamed content", () => {
    const longText = "a".repeat(200);
    const result = splitStreamingContent(longText, longText.length);

    expect(result.stable.length).toBeLessThan(longText.length);
    expect(result.tail.length).toBeGreaterThan(0);
    expect(result.stable + result.tail).toBe(longText);
  });
});
