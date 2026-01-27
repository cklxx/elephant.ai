import { parseJsonRenderPayload } from "../json-render-model";

describe("parseJsonRenderPayload", () => {
  it("parses a ui message bundle", () => {
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
    expect(tree.root?.type).toBe("Column");
    expect(Array.isArray(tree.root?.children)).toBe(true);
    expect(tree.root?.children?.length).toBe(2);
  });

  it("parses a root element", () => {
    const payload = JSON.stringify({
      type: "heading",
      props: { text: "Hello" },
    });

    const tree = parseJsonRenderPayload(payload);
    expect(tree.root?.type).toBe("heading");
  });

  it("applies json-render patches", () => {
    const payload = [
      JSON.stringify({
        op: "set",
        path: "/root",
        value: { type: "heading", props: { text: "Patch" } },
      }),
      JSON.stringify({
        op: "set",
        path: "/elements/root",
        value: { type: "heading", props: { text: "Patch" } },
      }),
    ].join("\n");

    const tree = parseJsonRenderPayload(payload);
    expect(tree.root?.type).toBe("heading");
  });
});
