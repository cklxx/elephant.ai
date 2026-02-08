import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useDebouncedValue } from "../useDebouncedValue";

describe("useDebouncedValue", () => {
  it("delays updates until timeout", () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebouncedValue(value, delay),
      {
        initialProps: { value: "a", delay: 200 },
      },
    );

    expect(result.current).toBe("a");

    rerender({ value: "ab", delay: 200 });
    expect(result.current).toBe("a");

    act(() => {
      vi.advanceTimersByTime(199);
    });
    expect(result.current).toBe("a");

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe("ab");

    vi.useRealTimers();
  });

  it("updates immediately when delay <= 0", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebouncedValue(value, delay),
      {
        initialProps: { value: "a", delay: 0 },
      },
    );

    expect(result.current).toBe("a");
    rerender({ value: "b", delay: 0 });
    expect(result.current).toBe("b");
  });
});
