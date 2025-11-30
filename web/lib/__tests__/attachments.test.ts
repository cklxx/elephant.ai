import { describe, expect, it, vi } from "vitest";

import { buildAttachmentUri } from "../attachments";

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
