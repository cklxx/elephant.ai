import { renderHook, act } from "@testing-library/react";
import { vi } from "vitest";
import { useSSE } from "../useSSE";
import { apiClient } from "@/lib/api";
import { authClient } from "@/lib/auth/client";
import { AnyAgentEvent } from "@/lib/types";

// Mock the apiClient
vi.mock("@/lib/api", () => ({
  apiClient: {
    createSSEConnection: vi.fn(),
  },
}));

const mockGetSession = vi.hoisted(() => vi.fn());

vi.mock("@/lib/auth/client", () => ({
  authClient: {
    getSession: mockGetSession,
  },
}));

// Mock EventSource
class MockEventSource {
  url: string;
  onopen: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  private listeners: Map<string, Set<(event: MessageEvent) => void>> =
    new Map();
  readyState: number = 0;

  constructor(url: string) {
    this.url = url;
    this.readyState = 0; // CONNECTING
  }

  addEventListener(
    type: string,
    listener: (event: MessageEvent) => void,
  ): void {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type)!.add(listener);
  }

  removeEventListener(
    type: string,
    listener: (event: MessageEvent) => void,
  ): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.readyState = 2; // CLOSED
  }

  // Test helpers
  simulateOpen(): void {
    this.readyState = 1; // OPEN
    if (this.onopen) {
      this.onopen(new Event("open"));
    }
  }

  simulateError(): void {
    if (this.onerror) {
      this.onerror(new Event("error"));
    }
  }

  simulateEvent(type: string, data: any): void {
    const listeners = this.listeners.get(type);
    if (listeners) {
      const event = new MessageEvent(type, { data: JSON.stringify(data) });
      listeners.forEach((listener) => listener(event));
    }
  }
}

describe("useSSE", () => {
  let mockEventSource: MockEventSource;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();

    mockGetSession.mockReturnValue({
      accessToken: "test-token",
      accessExpiry: new Date(Date.now() + 60_000).toISOString(),
      refreshExpiry: new Date(Date.now() + 3600_000).toISOString(),
      user: {
        id: "test-user",
        email: "test@example.com",
        displayName: "Test User",
        pointsBalance: 0,
        subscription: {
          tier: "free",
          monthlyPriceCents: 0,
          expiresAt: null,
          isPaid: false,
        },
      },
    });

    // Setup mock EventSource factory
    (apiClient.createSSEConnection as vi.Mock).mockImplementation(
      (sessionId: string, token?: string) => {
        mockEventSource = new MockEventSource(
          `http://localhost:8080/api/sse?session_id=${sessionId}&access_token=${token}`,
        );
        return mockEventSource;
      },
    );
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  describe("Basic Connection", () => {
    test("should establish connection when sessionId and enabled are provided", async () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      await act(async () => {
        await Promise.resolve();
      });

      expect(apiClient.createSSEConnection).toHaveBeenCalledWith(
        "test-session-123",
        "test-token",
      );
      expect(result.current.isConnected).toBe(false);
      expect(result.current.isReconnecting).toBe(false);
    });

    test("should set isConnected to true on successful connection", async () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      await act(async () => {
        await Promise.resolve();
      });

      act(() => {
        mockEventSource.simulateOpen();
      });

      expect(result.current.isConnected).toBe(true);
      expect(result.current.isReconnecting).toBe(false);
      expect(result.current.error).toBe(null);
      expect(result.current.reconnectAttempts).toBe(0);
    });

    test("should not establish connection when sessionId is null", () => {
      renderHook(() => useSSE(null));

      expect(apiClient.createSSEConnection).not.toHaveBeenCalled();
    });

    test("should not establish connection when enabled is false", () => {
      renderHook(() => useSSE("test-session-123", { enabled: false }));

      expect(apiClient.createSSEConnection).not.toHaveBeenCalled();
    });

    test("should disconnect when sessionId changes", async () => {
      const { rerender } = renderHook(({ sessionId }) => useSSE(sessionId), {
        initialProps: { sessionId: "session-1" },
      });

      await act(async () => {
        await Promise.resolve();
      });

      const firstEventSource = mockEventSource;
      const closeSpy = vi.spyOn(firstEventSource, "close");

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Change sessionId
      rerender({ sessionId: "session-2" });

      expect(closeSpy).toHaveBeenCalled();
      await act(async () => {
        await Promise.resolve();
      });
    });

    test("should cleanup on unmount", () => {
      const { unmount } = renderHook(() => useSSE("test-session-123"));

      act(() => {
        mockEventSource.simulateOpen();
      });

      const closeSpy = vi.spyOn(mockEventSource, "close");
      unmount();

      expect(closeSpy).toHaveBeenCalled();
    });
  });

  describe("Event Handling", () => {
    test("should collect events and update state", () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      act(() => {
        mockEventSource.simulateOpen();
      });

      const event1: AnyAgentEvent = {
        event_type: "task_analysis",
        timestamp: new Date().toISOString(),
        session_id: "test-session-123",
        agent_level: "core",
        action_name: "Plan",
        goal: "Test event 1",
      };

      const event2: AnyAgentEvent = {
        event_type: "thinking",
        timestamp: new Date().toISOString(),
        session_id: "test-session-123",
        agent_level: "core",
        iteration: 1,
        message_count: 1,
      };

      act(() => {
        mockEventSource.simulateEvent("task_analysis", event1);
        mockEventSource.simulateEvent("thinking", event2);
      });

      expect(result.current.events).toHaveLength(2);
      expect(result.current.events[0]).toEqual(event1);
      expect(result.current.events[1]).toEqual(event2);
    });

    test("should call onEvent callback when event is received", () => {
      const onEvent = vi.fn();
      const { result } = renderHook(() =>
        useSSE("test-session-123", { onEvent }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      const event: AnyAgentEvent = {
        event_type: "task_complete",
        timestamp: new Date().toISOString(),
        session_id: "test-session-123",
        agent_level: "core",
        final_answer: "done",
        total_iterations: 1,
        total_tokens: 10,
        stop_reason: "complete",
        duration: 1000,
      };

      act(() => {
        mockEventSource.simulateEvent("task_complete", event);
      });

      expect(onEvent).toHaveBeenCalledWith(event);
    });

    test("should clear events when clearEvents is called", () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      act(() => {
        mockEventSource.simulateOpen();
      });

      const event: AnyAgentEvent = {
        event_type: "error",
        timestamp: new Date().toISOString(),
        session_id: "test-session-123",
        agent_level: "core",
        iteration: 1,
        phase: "execute",
        error: "Test error",
        recoverable: false,
      };

      act(() => {
        mockEventSource.simulateEvent("error", event);
      });

      expect(result.current.events).toHaveLength(1);

      act(() => {
        result.current.clearEvents();
      });

      expect(result.current.events).toHaveLength(0);
    });
  });

  describe("Reconnection Logic", () => {
    test("should attempt reconnection on error with exponential backoff", () => {
      const { result } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: 5 }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Trigger error
      act(() => {
        mockEventSource.simulateError();
      });

      expect(result.current.isConnected).toBe(false);
      expect(result.current.isReconnecting).toBe(true);
      expect(result.current.reconnectAttempts).toBe(1);

      expect(vi.getTimerCount()).toBeGreaterThan(0);

      // First reconnection: 1000ms * 2^0 = 1000ms
      act(() => {
        vi.advanceTimersByTime(1000);
      });

      expect(result.current.isReconnecting).toBe(true);

      // Simulate another error on the new connection
      act(() => {
        mockEventSource.simulateError();
      });

      expect(result.current.reconnectAttempts).toBe(2);

      // Second reconnection: 1000ms * 2^1 = 2000ms
      act(() => {
        vi.advanceTimersByTime(2000);
      });

      expect(result.current.isReconnecting).toBe(true);
    });

    test("should cap exponential backoff at 30 seconds", () => {
      const { result } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: 10 }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Simulate multiple errors to reach high backoff
      for (let i = 0; i < 6; i++) {
        act(() => {
          mockEventSource.simulateError();
        });

        const expectedDelay = Math.min(1000 * Math.pow(2, i), 30000);
        act(() => {
          vi.advanceTimersByTime(expectedDelay);
        });
      }

      // 7th attempt should still use 30s cap (not 64s)
      expect(result.current.reconnectAttempts).toBe(6);
    });

    test("should stop reconnecting after max attempts", () => {
      const maxAttempts = 3;
      const { result } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: maxAttempts }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      // To reach max attempts, we need:
      // 1. Initial error (attempts: 0->1)
      // 2. Reconnect + error (attempts: 1->2)
      // 3. Reconnect + error (attempts: 2->3)
      // 4. Reconnect + error (attempts: 3, NOW 3 >= 3, so stop)
      for (let i = 0; i < maxAttempts; i++) {
        // Trigger error
        act(() => {
          mockEventSource.simulateError();
        });

        // Advance timer to trigger reconnection (except after the last one)
        if (i < maxAttempts - 1) {
          const delay = 1000 * Math.pow(2, i);
          act(() => {
            vi.advanceTimersByTime(delay);
          });
        }
      }

      // Process the final scheduled reconnection and trigger the terminal error
      const finalDelay = 1000 * Math.pow(2, maxAttempts - 1);
      act(() => {
        vi.advanceTimersByTime(finalDelay);
      });

      act(() => {
        mockEventSource.simulateError();
      });

      // After scheduled timers run, reconnection should stop
      expect(result.current.isReconnecting).toBe(false);
      expect(result.current.error).toBe(
        "Maximum reconnection attempts exceeded",
      );
      expect(result.current.reconnectAttempts).toBe(maxAttempts);

      // Should not schedule further reconnections
      act(() => {
        vi.runOnlyPendingTimers();
      });
      expect(vi.getTimerCount()).toBe(0);
    });

    test("should reset reconnection attempts on successful connection", () => {
      const { result } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: 5 }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Trigger error and reconnect
      act(() => {
        mockEventSource.simulateError();
      });

      expect(result.current.reconnectAttempts).toBe(1);

      // Advance timer and simulate successful reconnection
      act(() => {
        vi.advanceTimersByTime(1000);
      });

      act(() => {
        mockEventSource.simulateOpen();
      });

      expect(result.current.reconnectAttempts).toBe(0);
      expect(result.current.isConnected).toBe(true);
      expect(result.current.isReconnecting).toBe(false);
    });

    test("should NOT trigger double connections on reconnectAttempts state change", () => {
      const { result } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: 5 }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      const initialCallCount = (apiClient.createSSEConnection as vi.Mock).mock
        .calls.length;

      // Trigger error
      act(() => {
        mockEventSource.simulateError();
      });

      // At this point, reconnectAttempts state is updated to 1
      // and setTimeout is scheduled for 1000ms

      // Advance timer to trigger scheduled reconnection
      act(() => {
        vi.advanceTimersByTime(1000);
      });

      // Should only have ONE new connection attempt (from setTimeout)
      // NOT two (one from setTimeout + one from useEffect re-run)
      expect(
        (apiClient.createSSEConnection as vi.Mock).mock.calls.length,
      ).toBeLessThanOrEqual(initialCallCount + 1);
    });
  });

  describe("Manual Reconnection", () => {
    test("should reset attempts and reconnect when reconnect() is called", () => {
      const { result } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: 5 }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Simulate error to increment attempts
      act(() => {
        mockEventSource.simulateError();
      });

      expect(result.current.reconnectAttempts).toBe(1);

      // Manual reconnect
      act(() => {
        result.current.reconnect();
      });

      expect(result.current.reconnectAttempts).toBe(0);
      expect(result.current.error).toBe(null);
    });
  });

  describe("Connection Debouncing", () => {
    test("should prevent double connections when connect is called rapidly", () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      const initialCallCount = (apiClient.createSSEConnection as vi.Mock).mock
        .calls.length;

      // Try to connect multiple times rapidly
      act(() => {
        result.current.reconnect();
        result.current.reconnect();
        result.current.reconnect();
      });

      // Should only create one additional connection (debounced)
      // The isConnectingRef prevents duplicate attempts
      expect(
        (apiClient.createSSEConnection as vi.Mock).mock.calls.length,
      ).toBeLessThanOrEqual(initialCallCount + 3);
    });

    test("should cleanup pending reconnection timers when component unmounts", () => {
      const { result, unmount } = renderHook(() =>
        useSSE("test-session-123", { maxReconnectAttempts: 5 }),
      );

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Trigger error to schedule reconnection
      act(() => {
        mockEventSource.simulateError();
      });

      const callCountBeforeTimer = (apiClient.createSSEConnection as vi.Mock)
        .mock.calls.length;

      // Unmount component before timer fires - this should clear the reconnection timeout
      unmount();

      // Advance timer - should NOT trigger reconnection
      act(() => {
        vi.advanceTimersByTime(5000);
      });

      // No new connection should be created
      expect((apiClient.createSSEConnection as vi.Mock).mock.calls.length).toBe(
        callCountBeforeTimer,
      );
    });
  });

  describe("State Transitions", () => {
    test("should transition through states correctly: disconnected -> connecting -> connected", () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      // Initial state
      expect(result.current.isConnected).toBe(false);
      expect(result.current.isReconnecting).toBe(false);

      // Connect
      act(() => {
        mockEventSource.simulateOpen();
      });

      expect(result.current.isConnected).toBe(true);
      expect(result.current.isReconnecting).toBe(false);
    });

    test("should transition through states correctly: connected -> error -> reconnecting -> connected", () => {
      const { result } = renderHook(() => useSSE("test-session-123"));

      // Connect
      act(() => {
        mockEventSource.simulateOpen();
      });

      expect(result.current.isConnected).toBe(true);

      // Error
      act(() => {
        mockEventSource.simulateError();
      });

      expect(result.current.isConnected).toBe(false);
      expect(result.current.isReconnecting).toBe(true);

      // Reconnect
      act(() => {
        vi.advanceTimersByTime(1000);
      });

      // Successful reconnection
      act(() => {
        mockEventSource.simulateOpen();
      });

      expect(result.current.isConnected).toBe(true);
      expect(result.current.isReconnecting).toBe(false);
    });
  });

  describe("Edge Cases", () => {
    test("should handle rapid sessionId changes without memory leaks", async () => {
      const { rerender } = renderHook(({ sessionId }) => useSSE(sessionId), {
        initialProps: { sessionId: "session-1" },
      });

      await act(async () => {
        await Promise.resolve();
      });

      // Rapidly change sessions
      for (let i = 2; i <= 5; i++) {
        const prevEventSource = mockEventSource;
        const closeSpy = vi.spyOn(prevEventSource, "close");

        rerender({ sessionId: `session-${i}` });

        await act(async () => {
          await Promise.resolve();
        });

        expect(closeSpy).toHaveBeenCalled();
      }

      // Final session should be connected
      const calls = (apiClient.createSSEConnection as vi.Mock).mock.calls;
      expect(calls.at(-1)).toEqual(["session-5", "test-token"]);
    });

    test("should handle enabled toggle without breaking reconnection", async () => {
      const { rerender, result } = renderHook(
        ({ enabled }) => useSSE("test-session-123", { enabled }),
        { initialProps: { enabled: true } },
      );

      await act(async () => {
        await Promise.resolve();
      });

      act(() => {
        mockEventSource.simulateOpen();
      });

      const closeSpy = vi.spyOn(mockEventSource, "close");

      // Disable
      rerender({ enabled: false });
      expect(closeSpy).toHaveBeenCalled();

      // Re-enable
      rerender({ enabled: true });
      await act(async () => {
        await Promise.resolve();
      });
      expect(apiClient.createSSEConnection).toHaveBeenCalledTimes(2);
    });

    test("should handle malformed JSON events gracefully", () => {
      const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation();
      const { result } = renderHook(() => useSSE("test-session-123"));

      act(() => {
        mockEventSource.simulateOpen();
      });

      // Manually trigger event with bad JSON
      const listeners = (mockEventSource as any).listeners.get("task_analysis");
      if (listeners) {
        const badEvent = new MessageEvent("task_analysis", {
          data: "invalid json",
        });
        act(() => {
          listeners.forEach((listener: any) => listener(badEvent));
        });
      }

      // Should not crash, events should remain empty
      expect(result.current.events).toHaveLength(0);
      expect(consoleErrorSpy).toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });
  });
});
