import { parseJsonRenderPayload } from "../json-render-model";
import { renderJsonRenderHtml } from "../json-render-ssr";

describe("renderJsonRenderHtml", () => {
  it("renders a flow message bundle", () => {
    const payload = JSON.stringify({
      type: "ui",
      version: "1.0",
      messages: [
        { type: "heading", text: "Release Flow" },
        {
          type: "flow",
          direction: "horizontal",
          nodes: [
            { id: "design", label: "Design" },
            { id: "build", label: "Build" },
          ],
          edges: [{ from: "design", to: "build", label: "" }],
        },
      ],
    });

    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Release Flow");
    expect(html).toContain("Design");
    expect(html).toContain("Build");
  });
});
