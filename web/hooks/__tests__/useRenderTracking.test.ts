import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { performanceMonitor } from "@/lib/analytics/performance";
import { useRenderTracking } from "../useRenderTracking";

// The hook is dev-only (process.env.NODE_ENV === 'development').
// We test both paths by temporarily patching NODE_ENV at runtime.

describe("useRenderTracking", () => {
  let trackRenderSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    trackRenderSpy = vi.spyOn(performanceMonitor, "trackRenderTime");
    performanceMonitor.clear();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("calls trackRenderTime in development mode", () => {
    vi.stubEnv("NODE_ENV", "development");

    renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).toHaveBeenCalledWith(
      "TestComponent",
      expect.any(Number),
    );
  });

  it("reports non-negative duration", () => {
    vi.stubEnv("NODE_ENV", "development");

    renderHook(() => useRenderTracking("TestComponent"));

    const duration = trackRenderSpy.mock.calls[0]?.[1] as number;
    expect(duration).toBeGreaterThanOrEqual(0);
  });

  it("does not call trackRenderTime in production mode", () => {
    vi.stubEnv("NODE_ENV", "production");

    renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).not.toHaveBeenCalled();
  });

  it("does not call trackRenderTime in test mode", () => {
    vi.stubEnv("NODE_ENV", "test");

    renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).not.toHaveBeenCalled();
  });

  it("tracks subsequent re-renders", () => {
    vi.stubEnv("NODE_ENV", "development");

    const { rerender } = renderHook(() => useRenderTracking("TestComponent"));

    expect(trackRenderSpy).toHaveBeenCalledTimes(1);

    rerender();

    expect(trackRenderSpy).toHaveBeenCalledTimes(2);
  });
});
