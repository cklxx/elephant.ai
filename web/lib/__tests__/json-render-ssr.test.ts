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

  it("renders container/grid payloads from json-render protocol", () => {
    const payload = JSON.stringify({
      content: {
        type: "page",
        body: {
          type: "container",
          padding: 24,
          gap: 16,
          children: [
            {
              type: "grid",
              columns: 2,
              gap: 12,
              children: [
                {
                  type: "card",
                  children: [{ type: "text", value: "Card A" }],
                },
                {
                  type: "card",
                  children: [{ type: "text", value: "Card B" }],
                },
              ],
            },
          ],
        },
      },
    });

    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Card A");
    expect(html).toContain("Card B");
  });

  it("renders accordion component", () => {
    const payload = JSON.stringify({
      root: {
        type: "accordion",
        props: { title: "Details" },
        children: [{ type: "text", props: { text: "Hidden content" } }],
      },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Details");
    expect(html).toContain("Hidden content");
    expect(html).toContain("jr-accordion");
    expect(html).toContain("<details");
  });

  it("renders progress component", () => {
    const payload = JSON.stringify({
      root: {
        type: "progress",
        props: { value: 75, max: 100, label: "Upload" },
      },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Upload");
    expect(html).toContain("75%");
    expect(html).toContain("jr-progress-bar");
  });

  it("renders link component", () => {
    const payload = JSON.stringify({
      root: {
        type: "link",
        props: { href: "https://example.com", text: "Example" },
      },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Example");
    expect(html).toContain("https://example.com");
    expect(html).toContain('target="_blank"');
    expect(html).toContain("noopener noreferrer");
  });

  it("renders alert component with variants", () => {
    const payload = JSON.stringify({
      root: {
        type: "column",
        children: [
          { type: "alert", props: { variant: "warning", title: "Heads up", message: "Check this" } },
          { type: "alert", props: { variant: "error", message: "Something broke" } },
          { type: "alert", props: { variant: "success", title: "Done" } },
        ],
      },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Heads up");
    expect(html).toContain("Check this");
    expect(html).toContain("jr-alert-warning");
    expect(html).toContain("Something broke");
    expect(html).toContain("jr-alert-error");
    expect(html).toContain("jr-alert-success");
  });

  it("renders timeline component", () => {
    const payload = JSON.stringify({
      root: {
        type: "timeline",
        props: {
          items: [
            { title: "Step 1", description: "Init", status: "completed" },
            { title: "Step 2", status: "active" },
            { title: "Step 3" },
          ],
        },
      },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Step 1");
    expect(html).toContain("Init");
    expect(html).toContain("jr-dot-done");
    expect(html).toContain("jr-dot-active");
    expect(html).toContain("jr-dot-pending");
  });

  it("renders stat component", () => {
    const payload = JSON.stringify({
      root: {
        type: "stat",
        props: { label: "Revenue", value: "$12.4k", unit: "USD", change: "+14%", description: "vs last month" },
      },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Revenue");
    expect(html).toContain("$12.4k");
    expect(html).toContain("USD");
    expect(html).toContain("+14%");
    expect(html).toContain("vs last month");
    expect(html).toContain("jr-stat");
  });

  it("renders chart component", () => {
    const payload = JSON.stringify({
      type: "chart",
      title: "Population Trend",
      subtitle: "Projected decline",
      data: {
        values: [
          { year: 2020, population_m: 1412, projected: false },
          { year: 2021, population_m: 1413, projected: false },
          { year: 2022, population_m: 1411, projected: false },
          { year: 2023, population_m: 1409, projected: false },
          { year: 2024, population_m: 1406.5, projected: true },
          { year: 2025, population_m: 1404.0, projected: true },
        ],
      },
      encoding: {
        x: { field: "year", type: "temporal", title: "Year" },
        y: { field: "population_m", type: "quantitative", title: "Population (M)" },
        color: {
          field: "projected",
          type: "nominal",
          scale: { domain: [true, false], range: ["#f39c12", "#2c3e50"] },
        },
      },
      mark: { type: "line", point: true },
    });

    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Population Trend");
    expect(html).toContain("Projected decline");
    expect(html).toContain("Population (M)");
    expect(html).toContain("Year");
    expect(html).toContain("jr-chart");
    expect(html).toContain("<svg");
  });

  it("renders empty timeline as fallback", () => {
    const payload = JSON.stringify({
      root: { type: "timeline", props: { items: [] } },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("Timeline has no items");
  });

  it("clamps progress value between 0 and 100", () => {
    const payload = JSON.stringify({
      root: { type: "progress", props: { value: 150, max: 100 } },
    });
    const tree = parseJsonRenderPayload(payload);
    const html = renderJsonRenderHtml(tree);
    expect(html).toContain("width:100%");
  });
});
