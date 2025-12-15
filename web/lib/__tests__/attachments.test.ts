import { describe, expect, it, vi } from "vitest";

import { buildAttachmentUri, parseContentSegments } from "../attachments";
import { AttachmentPayload } from "@/lib/types";

describe("parseContentSegments", () => {
  const mockAttachments: Record<string, AttachmentPayload> = {
    "image.png": {
      name: "image.png",
      media_type: "image/png",
      uri: "https://example.com/image.png",
    },
    "doc.pdf": {
      name: "doc.pdf",
      media_type: "application/pdf",
      uri: "https://example.com/doc.pdf",
    },
  };

  it("parses full placeholders correctly", () => {
    const content = "Here is [image.png]";
    const scopedAttachments = { "image.png": mockAttachments["image.png"] };
    const segments = parseContentSegments(content, scopedAttachments);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toEqual({ type: "text", text: "Here is " });
    expect(segments[1]).toMatchObject({
      type: "image",
      placeholder: "[image.png]",
      attachment: mockAttachments["image.png"],
    });
  });

  it("handles suffix matching at start of content (truncated start)", () => {
    const content = "mage.png] matching test";
    const segments = parseContentSegments(content, mockAttachments);

    // Should detect 'mage.png]' as suffix of 'image.png'
    expect(segments[0]).toMatchObject({
      type: "image",
      placeholder: "[image.png]",
      attachment: mockAttachments["image.png"],
    });
    expect(segments[1]).toEqual({ type: "text", text: " matching test" });
  });

  it("handles prefix matching at end of content (truncated end)", () => {
    const content = "Check this [doc.";
    const segments = parseContentSegments(content, mockAttachments);

    expect(segments[0]).toEqual({ type: "text", text: "Check this " });
    // Should detect '[doc.' as prefix of 'doc.pdf'
    expect(segments[1]).toMatchObject({
      type: "document", // Assuming getAttachmentSegmentType returns document for pdf
      placeholder: "[doc.pdf]",
      attachment: mockAttachments["doc.pdf"],
    });
  });

  it("returns text for unmatched placeholders", () => {
    const content = "Unknown [unknown.txt]";
    // Pass empty attachments so nothing is appended
    const segments = parseContentSegments(content, {});
    expect(segments).toHaveLength(2);
    expect(segments[1]).toEqual({ type: "text", text: "[unknown.txt]" });
  });

  it("marks appended unused attachments as implicit", () => {
    const content = "Some text";
    const segments = parseContentSegments(content, mockAttachments);
    // 1 text segment + 2 implicit attachments
    expect(segments).toHaveLength(3);

    expect(segments[0]).toEqual({ type: "text", text: "Some text" });

    // The appended attachments should have implicit: true
    expect(segments[1].implicit).toBe(true);
    expect(segments[1].attachment).toBeDefined();

    expect(segments[2].implicit).toBe(true);
    expect(segments[2].attachment).toBeDefined();
  });

  it("does not mark inline attachments as implicit", () => {
    const content = "Here is [image.png]";
    const segments = parseContentSegments(content, mockAttachments);

    const imageSegment = segments.find(s => s.attachment?.name === "image.png");
    expect(imageSegment).toBeDefined();
    expect(imageSegment?.implicit).toBeUndefined();
  });

  it("handles mixed content with multiple placeholders", () => {
    const content = "A [image.png] and B [doc.pdf]";
    const segments = parseContentSegments(content, mockAttachments);
    expect(segments).toHaveLength(4);
    expect(segments[1].attachment).toBeDefined();
    expect(segments[3].attachment).toBeDefined();
  });
});


describe("buildAttachmentUri", () => {
  it("returns direct uri when provided", () => {
    const uri = "https://example.com/image.png";
    expect(
      buildAttachmentUri({
        name: "image.png",
        media_type: "image/png",
        uri,
      }),
    ).toBe(uri);
  });

  it("preserves data URIs without double prefixing", () => {
    const dataUri = "data:image/png;base64,aGVsbG8=";
    expect(
      buildAttachmentUri({
        name: "inline.png",
        media_type: "image/png",
        data: dataUri,
      }),
    ).toBe(dataUri);
  });

  it("wraps bare base64 payloads in a data URI", () => {
    expect(
      buildAttachmentUri({
        name: "inline.png",
        media_type: "image/png",
        data: "aGVsbG8=",
      }),
    ).toBe("data:image/png;base64,aGVsbG8=");
  });

  it("rewrites relative API paths using the configured API base", async () => {
    const previous = process.env.NEXT_PUBLIC_API_URL;
    process.env.NEXT_PUBLIC_API_URL = "http://localhost:8080";
    vi.resetModules();
    const { buildAttachmentUri: buildAttachmentUriWithEnv } = await import("../attachments");
    expect(
      buildAttachmentUriWithEnv({
        name: "cached.png",
        media_type: "image/png",
        uri: "/api/data/example-id",
      }),
    ).toBe("http://localhost:8080/api/data/example-id");
    process.env.NEXT_PUBLIC_API_URL = previous;
    vi.resetModules();
  });
});
