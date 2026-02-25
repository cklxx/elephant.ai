import {
  decodeHtmlFromDataUri,
  loadHtmlSource,
  ensureViewportMeta,
  ensureBaseHref,
  buildHtmlPreviewUrl,
  validateHtmlSource,
} from "../artifact-preview-html";

describe("decodeHtmlFromDataUri", () => {
  it("returns null for non-data URIs", () => {
    expect(decodeHtmlFromDataUri("https://example.com")).toBeNull();
  });

  it("decodes URL-encoded data URIs", () => {
    const html = "<h1>Hello</h1>";
    const uri = `data:text/html,${encodeURIComponent(html)}`;
    expect(decodeHtmlFromDataUri(uri)).toBe(html);
  });

  it("decodes base64 data URIs", () => {
    const html = "<p>Test</p>";
    const base64 = btoa(html);
    const uri = `data:text/html;base64,${base64}`;
    expect(decodeHtmlFromDataUri(uri)).toBe(html);
  });

  it("returns null for malformed data URIs without comma", () => {
    expect(decodeHtmlFromDataUri("data:text/html")).toBeNull();
  });
});

describe("loadHtmlSource", () => {
  it("decodes data URIs without fetch", async () => {
    const html = "<div>content</div>";
    const uri = `data:text/html,${encodeURIComponent(html)}`;
    const result = await loadHtmlSource(uri);
    expect(result).toBe(html);
  });

  it("fetches remote URLs", async () => {
    const html = "<p>remote</p>";
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(html),
    });
    const result = await loadHtmlSource("https://example.com/page.html");
    expect(result).toBe(html);
  });

  it("throws on failed fetch", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
    });
    await expect(loadHtmlSource("https://example.com/missing.html")).rejects.toThrow(
      "Failed to load HTML (404)",
    );
  });
});

describe("ensureViewportMeta", () => {
  it("returns unchanged if viewport already exists", () => {
    const html = '<head><meta name="viewport" content="width=device-width"></head>';
    expect(ensureViewportMeta(html)).toBe(html);
  });

  it("injects after <head> tag", () => {
    const result = ensureViewportMeta("<head></head><body>hi</body>");
    expect(result).toContain('name="viewport"');
  });

  it("wraps bare HTML with full structure", () => {
    const result = ensureViewportMeta("<p>bare</p>");
    expect(result).toContain("<!doctype html>");
    expect(result).toContain('name="viewport"');
    expect(result).toContain("<p>bare</p>");
  });

  it("returns empty html unchanged", () => {
    expect(ensureViewportMeta("")).toBe("");
    expect(ensureViewportMeta("  ")).toBe("  ");
  });
});

describe("ensureBaseHref", () => {
  it("returns unchanged when baseHref is null", () => {
    const html = "<head></head>";
    expect(ensureBaseHref(html, null)).toBe(html);
  });

  it("injects base tag after <head>", () => {
    const result = ensureBaseHref("<head></head>", "https://example.com/");
    expect(result).toContain('<base href="https://example.com/">');
  });

  it("skips if <base> already exists", () => {
    const html = '<head><base href="https://other.com/"></head>';
    expect(ensureBaseHref(html, "https://example.com/")).toBe(html);
  });
});

describe("buildHtmlPreviewUrl", () => {
  it("returns a blob URL when URL.createObjectURL is available", () => {
    const blobUrl = "blob:mock-url";
    globalThis.URL.createObjectURL = vi.fn().mockReturnValue(blobUrl);
    const result = buildHtmlPreviewUrl("<p>test</p>", null);
    expect(result.url).toBe(blobUrl);
    expect(result.shouldRevoke).toBe(true);
  });
});

describe("validateHtmlSource", () => {
  it("returns error for empty HTML", () => {
    const issues = validateHtmlSource("");
    expect(issues).toEqual([{ level: "error", message: "HTML is empty." }]);
  });

  it("returns warnings for minimal HTML", () => {
    const issues = validateHtmlSource("<p>Hello</p>");
    expect(issues.length).toBeGreaterThan(0);
    expect(issues.some((i) => i.message.includes("DOCTYPE"))).toBe(true);
  });

  it("returns no errors for well-formed HTML", () => {
    const html = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width">
  <title>Test</title>
</head>
<body><p>Content</p></body>
</html>`;
    const issues = validateHtmlSource(html);
    const errors = issues.filter((i) => i.level === "error");
    expect(errors).toHaveLength(0);
  });

  it("detects mismatched script tags", () => {
    const html = '<!DOCTYPE html><html><head><title>T</title><meta charset="utf-8"><meta name="viewport" content="w"></head><body><script>x</body></html>';
    const issues = validateHtmlSource(html);
    expect(issues.some((i) => i.message.includes("script"))).toBe(true);
  });
});
