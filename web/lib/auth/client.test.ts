// @vitest-environment happy-dom
import { describe, expect, beforeEach, vi, it, afterEach } from "vitest";

import { authClient } from "./client";

const STORAGE_KEY = "alex.console.auth";

function futureDate(minutes: number): string {
  return new Date(Date.now() + minutes * 60 * 1000).toISOString();
}

describe("authClient.handleStorageEvent", () => {
  beforeEach(() => {
    window.localStorage.clear();
    authClient.clearSession();
  });

  afterEach(() => {
    window.localStorage.clear();
    authClient.clearSession();
  });

  it("ignores storage events for other keys", () => {
    const listener = vi.fn();
    const unsubscribe = authClient.subscribe(listener);

    const event = new StorageEvent("storage", {
      key: "other-key",
      newValue: "{}",
    });

    authClient.handleStorageEvent(event);

    expect(listener).not.toHaveBeenCalled();

    unsubscribe();
  });

  it("updates the session when a new value is stored", () => {
    const listener = vi.fn();
    const unsubscribe = authClient.subscribe(listener);

    const storedSession = {
      accessToken: "access-token",
      accessExpiry: futureDate(5),
      refreshExpiry: futureDate(60),
      user: {
        id: "user-1",
        email: "user@example.com",
        displayName: "User",
        pointsBalance: 42,
        subscription: {
          tier: "supporter",
          monthlyPriceCents: 1200,
          expiresAt: futureDate(60 * 24),
          isPaid: true,
        },
      },
    };

    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(storedSession));

    const event = new StorageEvent("storage", {
      key: STORAGE_KEY,
      newValue: JSON.stringify(storedSession),
    });

    authClient.handleStorageEvent(event);

    expect(listener).toHaveBeenCalledTimes(1);
    const nextSession = authClient.getSession();
    expect(nextSession).toEqual(storedSession);

    unsubscribe();
  });

  it("clears the session when storage is emptied", () => {
    const listener = vi.fn();
    const unsubscribe = authClient.subscribe(listener);

    const storedSession = {
      accessToken: "token",
      accessExpiry: futureDate(5),
      refreshExpiry: futureDate(60),
      user: {
        id: "user-1",
        email: "user@example.com",
        displayName: "User",
        pointsBalance: 1,
        subscription: {
          tier: "free",
          monthlyPriceCents: 0,
          expiresAt: null,
          isPaid: false,
        },
      },
    };

    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(storedSession));
    authClient.handleStorageEvent(
      new StorageEvent("storage", {
        key: STORAGE_KEY,
        newValue: JSON.stringify(storedSession),
      }),
    );
    listener.mockClear();

    window.localStorage.removeItem(STORAGE_KEY);
    authClient.handleStorageEvent(
      new StorageEvent("storage", { key: STORAGE_KEY, newValue: null }),
    );

    expect(listener).toHaveBeenCalledTimes(1);
    expect(authClient.getSession()).toBeNull();

    unsubscribe();
  });
});

describe("authClient.waitForOAuthSession", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.localStorage.clear();
    authClient.clearSession();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    window.localStorage.clear();
    authClient.clearSession();
  });

  it("resolves when the session becomes available", async () => {
    const session = {
      accessToken: "oauth-access",
      accessExpiry: futureDate(5),
      refreshExpiry: futureDate(60),
      user: {
        id: "user-2",
        email: "oauth@example.com",
        displayName: "OAuth User",
        pointsBalance: 0,
        subscription: {
          tier: "free",
          monthlyPriceCents: 0,
          expiresAt: null,
          isPaid: false,
        },
      },
    };

    const popup = {
      closed: false,
      close: vi.fn(() => {
        popup.closed = true;
      }),
    } as unknown as Window;

    const resumeSpy = vi
      .spyOn(authClient, "resumeFromRefreshCookie")
      .mockResolvedValueOnce(null)
      .mockResolvedValue(session);

    const promise = authClient.waitForOAuthSession("google", {
      popup,
      timeoutMs: 5000,
      pollIntervalMs: 250,
    });

    await vi.advanceTimersByTimeAsync(250);
    const result = await promise;

    expect(result).toEqual(session);
    expect(resumeSpy).toHaveBeenCalledTimes(2);
    expect(popup.close).toHaveBeenCalledTimes(1);
  });

  it("rejects if the popup window closes", async () => {
    const popup = {
      closed: false,
      close: vi.fn(() => {
        popup.closed = true;
      }),
    } as unknown as Window;

    vi.spyOn(authClient, "resumeFromRefreshCookie").mockResolvedValue(null);

    const promise = authClient.waitForOAuthSession("google", {
      popup,
      timeoutMs: 5000,
      pollIntervalMs: 250,
    });

    popup.closed = true;

    const rejection = expect(promise).rejects.toThrow("OAuth window closed");
    await vi.advanceTimersByTimeAsync(250);
    await rejection;
  });

  it("rejects when the flow is cancelled", async () => {
    vi.spyOn(authClient, "resumeFromRefreshCookie").mockResolvedValue(null);

    const controller = new AbortController();

    const promise = authClient.waitForOAuthSession("wechat", {
      signal: controller.signal,
      timeoutMs: 5000,
      pollIntervalMs: 250,
    });

    const rejection = expect(promise).rejects.toThrow("OAuth login cancelled");
    controller.abort();
    await rejection;
  });

  it("resolves immediately when it receives an OAuth success message", async () => {
    const session = {
      accessToken: "oauth-access",
      accessExpiry: futureDate(5),
      refreshExpiry: futureDate(60),
      user: {
        id: "user-42",
        email: "oauth@example.com",
        displayName: "OAuth User",
        pointsBalance: 0,
        subscription: {
          tier: "free",
          monthlyPriceCents: 0,
          expiresAt: null,
          isPaid: false,
        },
      },
    };

    const popup = {
      closed: false,
      close: vi.fn(() => {
        popup.closed = true;
      }),
    } as unknown as Window;

    const resumeSpy = vi
      .spyOn(authClient, "resumeFromRefreshCookie")
      .mockResolvedValueOnce(null)
      .mockResolvedValue(session);

    const promise = authClient.waitForOAuthSession("google", {
      popup,
      timeoutMs: 5000,
      pollIntervalMs: 250,
    });

    await Promise.resolve();

    window.dispatchEvent(
      new MessageEvent("message", {
        data: { source: "alex-auth", status: "success" },
      }),
    );

    const result = await promise;

    expect(result).toEqual(session);
    expect(resumeSpy).toHaveBeenCalledTimes(2);
    expect(popup.close).toHaveBeenCalledTimes(1);
  });
});
