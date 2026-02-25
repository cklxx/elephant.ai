import { describe, expect, it } from "vitest";

import { parseA2UIMessagePayload } from "../a2ui";

describe("parseA2UIMessagePayload", () => {
  it("parses JSONL payloads", () => {
    const payload = [
      '{"surfaceUpdate":{"surfaceId":"main","components":[]}}',
      '{"beginRendering":{"surfaceId":"main","root":"root"}}',
    ].join("\n");
    const messages = parseA2UIMessagePayload(payload);
    expect(messages).toHaveLength(2);
    expect(messages[0].surfaceUpdate).toBeDefined();
    expect(messages[1].beginRendering).toBeDefined();
  });

  it("parses JSON array payloads", () => {
    const payload = JSON.stringify([
      { dataModelUpdate: { surfaceId: "main", contents: [] } },
    ]);
    const messages = parseA2UIMessagePayload(payload);
    expect(messages).toHaveLength(1);
    expect(messages[0].dataModelUpdate).toBeDefined();
  });
});
