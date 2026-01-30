import { normalizeTitle, stripRedundantHeading } from "../artifact-preview-markdown";

describe("normalizeTitle", () => {
  it("removes BOM and extension", () => {
    expect(normalizeTitle("\uFEFFMy Document.md")).toBe("my document");
  });

  it("returns null for null input", () => {
    expect(normalizeTitle(null)).toBeNull();
  });

  it("returns null for empty string", () => {
    expect(normalizeTitle("")).toBeNull();
  });

  it("normalizes special characters", () => {
    expect(normalizeTitle("hello-world_test.tsx")).toBe("hello world test");
  });
});

describe("stripRedundantHeading", () => {
  it("removes heading matching the title", () => {
    const md = "# My Document\n\nSome content here.";
    const result = stripRedundantHeading(md, "my document");
    expect(result).toBe("Some content here.");
  });

  it("keeps heading that does not match title", () => {
    const md = "# Different Title\n\nSome content.";
    const result = stripRedundantHeading(md, "my document");
    expect(result).toBe("# Different Title\n\nSome content.");
  });

  it("handles empty markdown", () => {
    expect(stripRedundantHeading("", "title")).toBe("");
  });

  it("handles whitespace-only markdown", () => {
    expect(stripRedundantHeading("   ", "title")).toBe("   ");
  });

  it("skips leading blank lines before heading", () => {
    const md = "\n\n# My Document\n\nContent after.";
    const result = stripRedundantHeading(md, "my document");
    expect(result).toBe("Content after.");
  });

  it("handles heading with trailing hashes", () => {
    const md = "## Report ###\n\nData follows.";
    const result = stripRedundantHeading(md, "report");
    expect(result).toBe("Data follows.");
  });

  it("returns trimmed start when no heading match", () => {
    const md = "\n\nNot a heading\nMore text";
    const result = stripRedundantHeading(md, "something");
    expect(result).toBe("Not a heading\nMore text");
  });
});
