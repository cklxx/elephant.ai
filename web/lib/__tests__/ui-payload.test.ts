import { parseUIPayload } from "../ui-payload";

describe("parseUIPayload", () => {
  it("rejects A2UI messages", () => {
    const payload = JSON.stringify({
      beginRendering: { surfaceId: "main", root: "root" },
    });
    const result = parseUIPayload(payload);
    expect(result.kind).toBe("unknown");
  });

  it("detects json-render payload", () => {
    const payload = JSON.stringify({
      type: "ui",
      version: "1.0",
      messages: [{ type: "heading", text: "Release Flow" }],
    });
    const result = parseUIPayload(payload);
    expect(result.kind).toBe("json-render");
  });
});
