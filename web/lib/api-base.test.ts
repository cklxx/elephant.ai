import { afterAll, beforeEach, describe, expect, it, vi } from "vitest";

declare global {
  var window: Window & typeof globalThis;
}

const REAL_ENV = { ...process.env };

function cleanupWindow() {
  if (typeof window !== "undefined") {
    // @ts-expect-error - delete test window shim
    delete globalThis.window;
  }
}

describe("resolveApiBaseUrl", () => {
  beforeEach(() => {
    cleanupWindow();
    process.env = { ...REAL_ENV };
    vi.resetModules();
  });

  afterAll(() => {
    cleanupWindow();
    process.env = REAL_ENV;
  });

  it("rewrites localhost API targets to the active hostname", async () => {
    process.env.NEXT_PUBLIC_API_URL = "http://localhost:8080";
    globalThis.window = {
      location: {
        origin: "http://152.136.32.119:3000",
        hostname: "152.136.32.119",
        href: "",
      } as unknown as Location,
    } as Window & typeof globalThis;

    const { resolveApiBaseUrl } = await import("./api-base");

    expect(resolveApiBaseUrl()).toBe("http://152.136.32.119:8080");
  });

  it("does not rewrite when the browser is also on localhost", async () => {
    process.env.NEXT_PUBLIC_API_URL = "http://localhost:8080";
    globalThis.window = {
      location: {
        origin: "http://localhost:3000",
        hostname: "localhost",
        href: "",
      } as unknown as Location,
    } as Window & typeof globalThis;
  
    const { resolveApiBaseUrl } = await import("./api-base");

    expect(resolveApiBaseUrl()).toBe("http://localhost:8080");
  });
});
