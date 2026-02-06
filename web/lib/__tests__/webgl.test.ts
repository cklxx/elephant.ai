import { describe, it, expect, vi, beforeEach } from "vitest";
import { detectWebGL, isMobileDevice, getGLCapabilities } from "../webgl";

describe("detectWebGL", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns true when webgl2 context is available", () => {
    const mockCanvas = {
      getContext: vi.fn().mockReturnValue({}),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);

    expect(detectWebGL()).toBe(true);
    expect(mockCanvas.getContext).toHaveBeenCalledWith("webgl2");
  });

  it("falls back to webgl when webgl2 is unavailable", () => {
    const mockCanvas = {
      getContext: vi.fn((ctx: string) => (ctx === "webgl" ? {} : null)),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);

    expect(detectWebGL()).toBe(true);
  });

  it("returns false when no WebGL is available", () => {
    const mockCanvas = {
      getContext: vi.fn().mockReturnValue(null),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);

    expect(detectWebGL()).toBe(false);
  });

  it("returns false when getContext throws", () => {
    const mockCanvas = {
      getContext: vi.fn().mockImplementation(() => {
        throw new Error("WebGL blocked");
      }),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);

    expect(detectWebGL()).toBe(false);
  });
});

describe("isMobileDevice", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns true for Android user agent", () => {
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue(
      "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36",
    );
    expect(isMobileDevice()).toBe(true);
  });

  it("returns true for iPhone user agent", () => {
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue(
      "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0)",
    );
    expect(isMobileDevice()).toBe(true);
  });

  it("returns false for desktop user agent", () => {
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue(
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
    );
    vi.spyOn(navigator, "maxTouchPoints", "get").mockReturnValue(0);
    expect(isMobileDevice()).toBe(false);
  });

  it("returns true for touch device with small viewport", () => {
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue(
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
    );
    vi.spyOn(navigator, "maxTouchPoints", "get").mockReturnValue(5);
    Object.defineProperty(window, "innerWidth", { value: 800, configurable: true });
    expect(isMobileDevice()).toBe(true);
  });
});

describe("getGLCapabilities", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns none tier when WebGL is unavailable", () => {
    const mockCanvas = {
      getContext: vi.fn().mockReturnValue(null),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);

    const caps = getGLCapabilities();
    expect(caps).toEqual({
      webgl: false,
      tier: "none",
      particleCount: 0,
      bloom: false,
      dpr: 1,
    });
  });

  it("returns low tier for mobile devices", () => {
    const mockCanvas = {
      getContext: vi.fn().mockReturnValue({}),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue(
      "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0)",
    );
    Object.defineProperty(window, "devicePixelRatio", { value: 3, configurable: true });

    const caps = getGLCapabilities();
    expect(caps.tier).toBe("low");
    expect(caps.particleCount).toBe(200);
    expect(caps.bloom).toBe(false);
    expect(caps.dpr).toBe(2); // capped at 2
  });

  it("returns high tier for desktop devices", () => {
    const mockCanvas = {
      getContext: vi.fn().mockReturnValue({}),
    };
    vi.spyOn(document, "createElement").mockReturnValue(mockCanvas as unknown as HTMLCanvasElement);
    vi.spyOn(navigator, "userAgent", "get").mockReturnValue(
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
    );
    vi.spyOn(navigator, "maxTouchPoints", "get").mockReturnValue(0);
    Object.defineProperty(window, "devicePixelRatio", { value: 2, configurable: true });

    const caps = getGLCapabilities();
    expect(caps.tier).toBe("high");
    expect(caps.particleCount).toBe(500);
    expect(caps.bloom).toBe(true);
    expect(caps.dpr).toBe(2);
  });
});
