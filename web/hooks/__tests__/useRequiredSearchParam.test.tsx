import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";

import { useRequiredSearchParam } from "../useRequiredSearchParam";

const useSearchParamsMock = vi.fn<() => URLSearchParams>();

vi.mock("next/navigation", () => ({
  useSearchParams: () => useSearchParamsMock(),
}));

describe("useRequiredSearchParam", () => {
  beforeEach(() => {
    useSearchParamsMock.mockReset();
  });

  it("returns value when required key exists", () => {
    useSearchParamsMock.mockReturnValue(new URLSearchParams("session_id=session-123"));

    const { result } = renderHook(() => useRequiredSearchParam("session_id"));

    expect(result.current).toEqual({ value: "session-123", missing: false });
  });

  it("marks missing when key is absent", () => {
    useSearchParamsMock.mockReturnValue(new URLSearchParams(""));

    const { result } = renderHook(() => useRequiredSearchParam("session_id"));

    expect(result.current).toEqual({ value: "", missing: true });
  });

  it("marks missing when key is present but empty", () => {
    useSearchParamsMock.mockReturnValue(new URLSearchParams("session_id="));

    const { result } = renderHook(() => useRequiredSearchParam("session_id"));

    expect(result.current).toEqual({ value: "", missing: true });
  });

  it("supports optional trimming", () => {
    useSearchParamsMock.mockReturnValue(new URLSearchParams("session_id=%20%20abc%20%20"));

    const { result } = renderHook(() =>
      useRequiredSearchParam("session_id", { trim: true }),
    );

    expect(result.current).toEqual({ value: "abc", missing: false });
  });

  it("treats trimmed empty values as missing", () => {
    useSearchParamsMock.mockReturnValue(new URLSearchParams("session_id=%20%20"));

    const { result } = renderHook(() =>
      useRequiredSearchParam("session_id", { trim: true }),
    );

    expect(result.current).toEqual({ value: "", missing: true });
  });
});
