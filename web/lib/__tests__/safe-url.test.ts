import { describe, expect, it } from "vitest";

import { sanitizeLinkHref, sanitizeMediaUrl } from "../safe-url";

describe("sanitizeLinkHref", () => {
  it("allows safe relative and external links", () => {
    expect(sanitizeLinkHref("/sessions/123")).toBe("/sessions/123");
    expect(sanitizeLinkHref("https://example.com")).toBe("https://example.com");
    expect(sanitizeLinkHref("mailto:test@example.com")).toBe(
      "mailto:test@example.com",
    );
  });

  it("rejects javascript and protocol-relative links", () => {
    expect(sanitizeLinkHref("javascript:alert(1)")).toBeNull();
    expect(sanitizeLinkHref("java\nscript:alert(1)")).toBeNull();
    expect(sanitizeLinkHref("//evil.example.com")).toBeNull();
  });
});

describe("sanitizeMediaUrl", () => {
  it("allows blob and data media URLs", () => {
    expect(sanitizeMediaUrl("blob:https://example.com/file")).toBe(
      "blob:https://example.com/file",
    );
    expect(sanitizeMediaUrl("data:image/png;base64,aGVsbG8=")).toBe(
      "data:image/png;base64,aGVsbG8=",
    );
  });

  it("rejects executable javascript media URLs", () => {
    expect(sanitizeMediaUrl("javascript:alert(1)")).toBeNull();
  });
});
