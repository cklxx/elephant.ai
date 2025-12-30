import { afterEach, describe, expect, it, vi } from "vitest";

describe("buildApiUrl", () => {
  const originalEnv = process.env.NEXT_PUBLIC_API_URL;
  const originalLocation = window.location;
  const originalLocationDescriptor = Object.getOwnPropertyDescriptor(window, "location");

  afterEach(() => {
    process.env.NEXT_PUBLIC_API_URL = originalEnv;
    if (originalLocationDescriptor) {
      Object.defineProperty(window, "location", originalLocationDescriptor);
    } else {
      Object.defineProperty(window, "location", {
        value: originalLocation,
        writable: true,
        configurable: true,
      });
    }
    vi.resetModules();
  });

  it("rewrites internal hosts to the current browser host", async () => {
    process.env.NEXT_PUBLIC_API_URL = "http://alex-server:8080";
    Object.defineProperty(window, "location", {
      value: new URL("https://app.example.com") as unknown as Location,
      writable: true,
      configurable: true,
    });
    vi.resetModules();
    const { buildApiUrl } = await import("../api-base");

    expect(buildApiUrl("/api/data/test")).toBe("https://app.example.com:8080/api/data/test");
  });
});
