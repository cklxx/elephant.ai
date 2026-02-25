import { describe, expect, it } from "vitest";

import { renderA2UIHtml } from "../a2ui-ssr";

describe("renderA2UIHtml", () => {
  it("renders basic text content", () => {
    const payload = JSON.stringify([
      {
        surfaceUpdate: {
          surfaceId: "main",
          components: [
            { id: "root", component: { Column: { children: ["text"] } } },
            { id: "text", component: { Text: { text: { literalString: "Hello SSR" } } } },
          ],
        },
      },
      { beginRendering: { surfaceId: "main", root: "root" } },
    ]);

    const html = renderA2UIHtml(payload);
    expect(html).toContain("Hello SSR");
    expect(html).toContain("a2ui-surface");
  });

  it("accepts JSONL payloads", () => {
    const lines = [
      JSON.stringify({
        surfaceUpdate: {
          surfaceId: "main",
          components: [
            { id: "root", component: { Column: { children: ["text"] } } },
            { id: "text", component: { Text: { text: { literalString: "Line" } } } },
          ],
        },
      }),
      JSON.stringify({ beginRendering: { surfaceId: "main", root: "root" } }),
    ];

    const html = renderA2UIHtml(lines.join("\n"));
    expect(html).toContain("Line");
  });
});
