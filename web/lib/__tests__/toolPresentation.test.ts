import { describe, expect, it } from "vitest";

import { userFacingToolTitle } from "../toolPresentation";
import { humanizeToolName } from "../utils";

describe("userFacingToolTitle", () => {
  const maxLen = 56;

  it("keeps short titles intact", () => {
    const base = humanizeToolName("web_fetch");
    const title = userFacingToolTitle({
      toolName: "web_fetch",
      arguments: { url: "https://example.com" },
    });

    expect(title).toBe(`${base}：https://example.com`);
  });

  it("truncates long hints to keep the title compact", () => {
    const base = humanizeToolName("web_search");
    const longQuery = "a".repeat(200);
    const title = userFacingToolTitle({
      toolName: "web_search",
      arguments: { query: longQuery },
    });

    expect(title.startsWith(`${base}：`)).toBe(true);
    expect(title.endsWith("…")).toBe(true);
    expect(title.length).toBeLessThanOrEqual(maxLen);
  });

  it("truncates long base tool names when no hint is present", () => {
    const longToolName = Array.from({ length: 20 }, () => "tool").join("_");
    const base = humanizeToolName(longToolName);
    const title = userFacingToolTitle({ toolName: longToolName });

    expect(base.length).toBeGreaterThan(maxLen);
    expect(title.length).toBeLessThanOrEqual(maxLen);
    expect(title.endsWith("…")).toBe(true);
  });
});
