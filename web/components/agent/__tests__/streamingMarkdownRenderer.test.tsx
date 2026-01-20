import { describe, expect, it } from "vitest";

import { sliceStreamingContent } from "@/components/agent/StreamingMarkdownRenderer";

describe("sliceStreamingContent", () => {
  it("clamps visible length to bounds", () => {
    const content = "hello";
    expect(sliceStreamingContent(content, -1)).toBe("");
    expect(sliceStreamingContent(content, 0)).toBe("");
    expect(sliceStreamingContent(content, 2)).toBe("he");
    expect(sliceStreamingContent(content, 99)).toBe("hello");
  });
});
