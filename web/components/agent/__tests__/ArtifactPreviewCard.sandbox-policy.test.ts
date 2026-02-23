import { describe, expect, it } from "vitest";

import { UNTRUSTED_HTML_PREVIEW_SANDBOX } from "../ArtifactPreviewCard";

describe("ArtifactPreviewCard sandbox policy", () => {
  it("does not permit allow-same-origin for untrusted HTML previews", () => {
    expect(UNTRUSTED_HTML_PREVIEW_SANDBOX.includes("allow-scripts")).toBe(true);
    expect(
      UNTRUSTED_HTML_PREVIEW_SANDBOX.includes("allow-same-origin"),
    ).toBe(false);
  });
});
