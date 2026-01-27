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

  it("renders table and kanban components", () => {
    const payload = JSON.stringify({
      root: {
        type: "column",
        children: [
          {
            type: "table",
            props: {
              headers: ["Service", "Latency"],
              rows: [
                ["api", 120],
                ["search", 180],
              ],
            },
          },
          {
            type: "kanban",
            props: {
              columns: [
                {
                  title: "Todo",
                  items: [{ title: "Collect requirements" }],
                },
              ],
            },
          },
          {
            type: "diagram",
            props: {
              nodes: [
                { id: "a", label: "A" },
                { id: "b", label: "B" },
              ],
              edges: [{ from: "a", to: "b", label: "calls" }],
            },
          },
        ],
      },
    });

    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Service");
    expect(html).toContain("Latency");
    expect(html).toContain("Collect requirements");
    expect(html).toContain("a -> b");
    expect(html).toContain("calls");
  });
});
