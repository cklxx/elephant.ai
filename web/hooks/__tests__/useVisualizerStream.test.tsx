import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useVisualizerStream } from "../useVisualizerStream";

class MockEventSource {
  static instances: MockEventSource[] = [];
  url: string;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  close = vi.fn();

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }

  emitMessage(payload: unknown): void {
    if (!this.onmessage) {
      return;
    }
    this.onmessage(
      new MessageEvent("message", {
        data: JSON.stringify(payload),
      }),
    );
  }
}

const realEventSource = globalThis.EventSource;

describe("useVisualizerStream", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockEventSource.instances = [];
    globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    globalThis.EventSource = realEventSource;
  });

  it("clears stale timers between events and on unmount", () => {
    const { unmount } = renderHook(() => useVisualizerStream());
    const eventSource = MockEventSource.instances[0]!;

    act(() => {
      eventSource.emitMessage({
        timestamp: "2026-02-23T00:00:00.000Z",
        event: "tool.start",
        tool: "shell",
        status: "started",
      });
    });

    expect(vi.getTimerCount()).toBe(1);

    act(() => {
      eventSource.emitMessage({
        timestamp: "2026-02-23T00:00:01.000Z",
        event: "tool.finish",
        tool: "shell",
        status: "completed",
      });
    });

    expect(vi.getTimerCount()).toBe(1);

    unmount();

    expect(eventSource.close).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });
});
