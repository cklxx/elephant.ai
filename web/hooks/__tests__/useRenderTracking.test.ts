import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { performanceMonitor } from "@/lib/analytics/performance";
import { useRenderTracking } from "../useRenderTracking";

// The hook is dev-only (process.env.NODE_ENV === 'development').
// We test both paths by temporarily patching NODE_ENV at runtime.

let originalNodeEnv: string | undefined;

describe("useRenderTracking", () => {
  let trackRenderSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    originalNodeEnv = process.env.NODE_ENV;
    trackRenderSpy = vi.spyOn(performanceMonitor, "trackRenderTime");
    performanceMonitor.clear();
  });

  afterEach(() => {
    process.env.NODE_ENV = originalNodeEnv;
    vi.restoreAllMocks();
  });

  it("calls trackRenderTime in development mode", () => {
    process.env.NODE_ENV = "development";

    renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).toHaveBeenCalledWith(
      "TestComponent",
      expect.any(Number),
    );
  });

  it("reports non-negative duration", () => {
    process.env.NODE_ENV = "development";

    renderHook(() => useRenderTracking("TestComponent"));

    const duration = trackRenderSpy.mock.calls[0]?.[1] as number;
    expect(duration).toBeGreaterThanOrEqual(0);
  });

  it("does not call trackRenderTime in production mode", () => {
    process.env.NODE_ENV = "production";

    renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).not.toHaveBeenCalled();
  });

  it("does not call trackRenderTime in test mode", () => {
    process.env.NODE_ENV = "test";

    renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).not.toHaveBeenCalled();
  });

  it("tracks subsequent re-renders", () => {
    process.env.NODE_ENV = "development";

    const { rerender } = renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).toHaveBeenCalledTimes(1);

    rerender();

    expect(trackRenderSpy).toHaveBeenCalledTimes(2);
  });
});
