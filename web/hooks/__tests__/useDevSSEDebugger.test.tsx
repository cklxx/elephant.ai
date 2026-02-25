import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ChangeEvent } from "react";

import { useDevSSEDebugger } from "../useDevSSEDebugger";

class MockEventSource {
  onopen: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  readonly close = vi.fn();
  private readonly listeners = new Map<string, Set<(event: MessageEvent) => void>>();

  addEventListener(type: string, listener: (event: MessageEvent) => void): void {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type)?.add(listener);
  }

  emitOpen(): void {
    this.onopen?.(new Event("open"));
  }

  emitError(): void {
    this.onerror?.(new Event("error"));
  }

  emitEvent(type: string, payload: unknown, lastEventId?: string): void {
    const listeners = this.listeners.get(type);
    if (!listeners) {
      return;
    }
    const data = typeof payload === "string" ? payload : JSON.stringify(payload);
    const event = new MessageEvent(type, { data, lastEventId });
    listeners.forEach((listener) => listener(event));
  }

  emitMessage(payload: unknown, lastEventId?: string): void {
    const data = typeof payload === "string" ? payload : JSON.stringify(payload);
    this.onmessage?.(new MessageEvent("message", { data, lastEventId }));
  }
}

describe("useDevSSEDebugger", () => {
  it("requires a session ID before connecting", () => {
    const createConnection = vi.fn();
    const { result } = renderHook(() =>
      useDevSSEDebugger({
        eventTypes: ["workflow.result.final"],
        createConnection,
      }),
    );

    act(() => {
      result.current.connect();
    });

    expect(createConnection).not.toHaveBeenCalled();
    expect(result.current.error).toBe("Session ID is required.");
  });

  it("connects, collects events, and keeps only the last N events", () => {
    const sources: MockEventSource[] = [];
    const createConnection = vi.fn(() => {
      const source = new MockEventSource();
      sources.push(source);
      return source as unknown as EventSource;
    });

    const { result } = renderHook(() =>
      useDevSSEDebugger({
        eventTypes: ["workflow.result.final"],
        createConnection,
        defaultMaxEvents: 2,
      }),
    );

    act(() => {
      result.current.setSessionIdInput("  session-123  ");
    });
    act(() => {
      result.current.connect();
    });

    expect(createConnection).toHaveBeenCalledWith({
      sessionId: "session-123",
      replayMode: "session",
    });
    expect(result.current.isConnecting).toBe(true);

    const source = sources[0];
    act(() => {
      source.emitOpen();
      source.emitEvent("workflow.result.final", { event_type: "workflow.result.final", step: 1 }, "e-1");
      source.emitMessage({ message: "hello" }, "e-2");
      source.emitEvent("workflow.result.final", "not-json", "e-3");
    });

    expect(result.current.isConnected).toBe(true);
    expect(result.current.events).toHaveLength(2);
    expect(result.current.events[0].eventType).toBe("message");
    expect(result.current.events[0].lastEventId).toBe("e-2");
    expect(result.current.events[1].eventType).toBe("workflow.result.final");
    expect(result.current.events[1].parseError).toBeTruthy();
    expect(result.current.selectedId).toBe(result.current.events[1].id);
  });

  it("shrinks event buffer and updates selection when max events decreases", () => {
    const source = new MockEventSource();
    const { result } = renderHook(() =>
      useDevSSEDebugger({
        eventTypes: ["workflow.result.final"],
        createConnection: () => source as unknown as EventSource,
        defaultMaxEvents: 5,
      }),
    );

    act(() => {
      result.current.setSessionIdInput("session-1");
    });
    act(() => {
      result.current.connect();
    });
    act(() => {
      source.emitOpen();
      source.emitEvent("workflow.result.final", { event_type: "workflow.result.final", step: 1 });
      source.emitEvent("workflow.result.final", { event_type: "workflow.result.final", step: 2 });
      source.emitEvent("workflow.result.final", { event_type: "workflow.result.final", step: 3 });
    });

    const firstId = result.current.events[0]?.id;
    const lastId = result.current.events[result.current.events.length - 1]?.id;
    expect(firstId).toBeTruthy();
    expect(lastId).toBeTruthy();

    act(() => {
      result.current.setSelectedId(firstId ?? null);
      result.current.handleMaxEventsChange({
        target: { value: "1" },
      } as ChangeEvent<HTMLInputElement>);
    });

    expect(result.current.events).toHaveLength(1);
    expect(result.current.selectedId).toBe(lastId);
  });

  it("closes active connection on unmount", () => {
    const source = new MockEventSource();
    const { result, unmount } = renderHook(() =>
      useDevSSEDebugger({
        eventTypes: ["workflow.result.final"],
        createConnection: () => source as unknown as EventSource,
      }),
    );

    act(() => {
      result.current.setSessionIdInput("session-close");
    });
    act(() => {
      result.current.connect();
    });

    unmount();
    expect(source.close).toHaveBeenCalledTimes(1);
  });
});
